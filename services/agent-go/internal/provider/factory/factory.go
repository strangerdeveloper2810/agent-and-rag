// Package factory chọn Provider theo config.
// Hỗ trợ chế độ "auto": Gemini → DeepSeek → Claude fallback chain.
// DeepSeek đóng vai trò immediate fallback rẻ tiền khi Gemini bị rate limit.
package factory

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/provider/anthropic"
	"github.com/ai-agent-tut/agent-go/internal/provider/deepseek"
	"github.com/ai-agent-tut/agent-go/internal/provider/fallback"
	"github.com/ai-agent-tut/agent-go/internal/provider/gemini"
)

// New tạo Provider theo cfg.Provider:
//
//	"gemini"    → Gemini (single)
//	"anthropic" → Claude (single)
//	"deepseek"  → DeepSeek (single)
//	"auto"      → Fallback chain: Gemini (toàn bộ free-tier pool) → DeepSeek (nếu có key) → Claude (nếu có key)
func New(cfg config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "gemini":
		return newGemini(cfg)
	case "anthropic":
		return newAnthropic(cfg)
	case "deepseek":
		return newDeepSeek(cfg)
	case "auto":
		return newAuto(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %q (use gemini, anthropic, deepseek, or auto)", cfg.Provider)
	}
}

func newGemini(cfg config.Config) (provider.Provider, error) {
	if cfg.GeminiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required for provider=gemini")
	}
	return gemini.New(cfg.GeminiKey, cfg.GeminiModel, provider.ThinkingLevel(cfg.ThinkingLevel))
}

func newAnthropic(cfg config.Config) (provider.Provider, error) {
	if cfg.AnthropicKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for provider=anthropic")
	}
	return anthropic.New(cfg.AnthropicKey, cfg.AnthropicModel)
}

func newDeepSeek(cfg config.Config) (provider.Provider, error) {
	if cfg.DeepSeekKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY is required for provider=deepseek")
	}
	return deepseek.New(cfg.DeepSeekKey, cfg.DeepSeekFlashModel, cfg.DeepSeekProModel)
}

// newAuto tạo provider chain thông minh:
//
//	Order: Toàn bộ Gemini free-tier pool (3.1-flash-lite → 3.5-flash-lite → 3.7-flash → ...) → DeepSeek (siêu rẻ) → Claude (chốt chặn)
//	Chỉ cần ít nhất 1 provider có key.
func newAuto(cfg config.Config) (provider.Provider, error) {
	hasGemini := cfg.GeminiKey != ""
	hasDeepSeek := cfg.DeepSeekKey != ""
	hasClaude := cfg.AnthropicKey != ""

	providers := make([]provider.Provider, 0, 10)

	if hasGemini {
		// Thu thập danh sách model không trùng lặp theo đúng thứ tự ưu tiên
		models := make([]string, 0, 2+len(cfg.GeminiFallbackModels))
		seen := make(map[string]bool)

		addModel := func(m string) {
			m = strings.TrimSpace(m)
			if m != "" && !seen[m] {
				seen[m] = true
				models = append(models, m)
			}
		}

		// 1. Primary Gemini model (vd: gemini-3.1-flash-lite: 500 RPD)
		addModel(cfg.GeminiModel)

		// 2. Secondary Gemini model (nếu có config riêng)
		if cfg.GeminiSecondaryModel != "" {
			addModel(cfg.GeminiSecondaryModel)
		}

		// 3. Toàn bộ Fallback Gemini models pool (3.5-flash-lite, 3.7-flash, 3.6-flash, 3.5-flash, 3-flash, 2.5-flash-lite, 2.5-flash)
		for _, m := range cfg.GeminiFallbackModels {
			addModel(m)
		}

		for _, m := range models {
			mCfg := cfg
			mCfg.GeminiModel = m
			g, err := newGemini(mCfg)
			if err == nil {
				providers = append(providers, g)
			} else if m == cfg.GeminiModel {
				// Lỗi constructor của primary model → báo lỗi ngay
				return nil, err
			}
		}
	}
	if hasDeepSeek {
		// DeepSeek (primary cost-effective pay-as-you-go)
		d, err := newDeepSeek(cfg)
		if err != nil {
			return nil, err
		}
		providers = append(providers, d)
	}
	if hasClaude {
		// Claude (last resort fallback)
		c, err := newAnthropic(cfg)
		if err != nil {
			return nil, err
		}
		providers = append(providers, c)
	}

	switch len(providers) {
	case 0:
		return nil, fmt.Errorf("auto provider: need at least one of GEMINI_API_KEY, DEEPSEEK_API_KEY, or ANTHROPIC_API_KEY")
	case 1:
		return providers[0], nil
	default:
		// 15s base cooldown kết hợp circuit breaker (exponential backoff cho RPM, day-lock 2h cho RPD)
		return fallback.New(15*time.Second, providers...)
	}
}

// NewReflectionProvider tạo provider RIÊNG cho tác vụ reflection nền (trích
// user facts / knowledge items sau mỗi lượt chat — xem internal/memory.Learner).
//
// Vì sao KHÔNG dùng chung provider chính (factory.New): reflection là tác vụ
// phụ trợ, không nên cạnh tranh quota Gemini free-tier với luồng chat chính —
// log production cho thấy reflection cascade qua 6+ biến thể Gemini (429 liên
// tiếp) trước khi rơi xuống DeepSeek, làm chậm và tốn request quota vô ích.
// DeepSeek đơn (rẻ, không rate-limit chặt như Gemini free-tier) là lựa chọn
// hợp lý cho 1 tác vụ trích xuất JSON máy móc, không cần model tốt nhất.
//
// Nếu KHÔNG có DEEPSEEK_API_KEY, fallback về provider chính (factory.New) để
// không phá vỡ khi thiếu key — hành vi giống trước khi có hàm này.
func NewReflectionProvider(cfg config.Config) (provider.Provider, error) {
	if cfg.DeepSeekKey != "" {
		return newDeepSeek(cfg)
	}
	slog.Warn("factory: thiếu DEEPSEEK_API_KEY — reflection dùng chung provider chính, có thể cạnh tranh quota Gemini với chat chính")
	return New(cfg)
}
