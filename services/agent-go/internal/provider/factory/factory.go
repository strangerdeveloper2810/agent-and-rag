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
	"github.com/ai-agent-tut/agent-go/internal/provider/ollama"
	"github.com/ai-agent-tut/agent-go/internal/provider/openai_compat"
	"github.com/ai-agent-tut/agent-go/internal/provider/router"
)

// namedOverride bọc 1 provider để đổi Name() hiển thị — dùng khi có NHIỀU
// instance cùng loại provider trong 1 chain (vd 2 Claude key khác nhau) và
// muốn log phân biệt được đang chạy key nào, không đổi gì trong package con
// (anthropic.Client.Name() vẫn trả "anthropic" như cũ).
//
// ⚠️ Đổi tên qua namedOverride làm "anthropic-1"/"anthropic-2" KHÔNG còn khớp
// exact-match với modelFamily() trong fallback.go (luôn trả "anthropic" cho
// model claude-*) — nếu sau này có caller set Options.Model="claude-..." khi
// gọi qua chain có 2 Claude key, scopeModel() sẽ không nhận diện đúng family
// nữa. Hiện tại chưa có caller nào set Options.Model=claude-* (luôn DeepSeek/
// Gemini) nên chưa phải bug sống, nhưng cần biết nếu sau này thêm caller mới.
type namedOverride struct {
	provider.Provider
	name string
}

func (n namedOverride) Name() string { return n.name }

// Model forward tới provider gốc nếu nó implement interface này (vd
// *anthropic.Client.Model()) — thiếu forward này thì log lỗi fallback cho
// nhánh anthropic-1/anthropic-2 sẽ mất tên model thật, chỉ còn tên chain
// position (regression đúng loại vấn đề cả plan này đang sửa).
func (n namedOverride) Model() string {
	if mn, ok := n.Provider.(interface{ Model() string }); ok {
		return mn.Model()
	}
	return ""
}

// New tạo Provider theo cfg.Provider:
//
//	"gemini"        → Gemini (single)
//	"anthropic"     → Claude (single)
//	"deepseek"      → DeepSeek (single)
//	"ollama"        → Ollama local (single)
//	"openai_compat" → server local kiểu OpenAI-compatible (vLLM, llama.cpp, LM Studio...) (single)
//	"auto"          → Fallback chain: Gemini (toàn bộ free-tier pool) → DeepSeek (nếu có key) → Claude (nếu có key)
//	"router"        → RouterProvider: request "nhẹ" (ThinkingOff + không tool) đi local
//	                  (Ollama/OpenAI-compat theo cfg.RouterLocalBackend), còn lại đi chain "auto"
func New(cfg config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "gemini":
		return newGemini(cfg)
	case "anthropic":
		return newAnthropic(cfg)
	case "deepseek":
		return newDeepSeek(cfg)
	case "ollama":
		return newOllama(cfg)
	case "openai_compat":
		return newOpenAICompat(cfg)
	case "auto":
		return newAuto(cfg)
	case "router":
		return newRouter(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %q (use gemini, anthropic, deepseek, ollama, openai_compat, router, or auto)", cfg.Provider)
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

// newOllama tạo provider Ollama local — không cần API key, chỉ cần server
// Ollama đang chạy tại cfg.OllamaURL. cfg.OllamaURL/OllamaModel đã có default
// hợp lý ("http://localhost:11434"/"llama3.1:8b") ngay trong config.Load(),
// nên không cần default lại ở đây.
func newOllama(cfg config.Config) (provider.Provider, error) {
	return ollama.New(cfg.OllamaURL, cfg.OllamaModel)
}

// newOpenAICompat tạo provider trỏ tới server local kiểu OpenAI-compatible
// tuỳ ý (vLLM, llama.cpp server, LM Studio...). Khác Ollama: không có default
// baseURL/model cố định vì mỗi server local nghe ở cổng khác nhau và chạy
// model do người dùng tự nạp — cfg.OpenAICompatKey để trống nếu server không
// yêu cầu auth.
func newOpenAICompat(cfg config.Config) (provider.Provider, error) {
	return openai_compat.New(cfg.OpenAICompatBaseURL, cfg.OpenAICompatKey, cfg.OpenAICompatModel)
}

// newAuto tạo provider chain thông minh:
//
//	Order: Toàn bộ Gemini free-tier pool (3.1-flash-lite → 3.5-flash-lite → 3.7-flash → ...) → DeepSeek (siêu rẻ) → Claude (chốt chặn)
//	Chỉ cần ít nhất 1 provider có key.
func newAuto(cfg config.Config) (provider.Provider, error) {
	hasGemini := cfg.GeminiKey != ""
	hasDeepSeek := cfg.DeepSeekKey != ""
	hasClaude := cfg.AnthropicKey != ""

	if cfg.AnthropicKey2 != "" && !hasClaude {
		// Key 2 vô nghĩa nếu không có key 1 — im lặng bỏ qua sẽ khiến operator
		// tưởng key 2 đã được dùng (vd gõ nhầm ANTHROPIC_API_KEY_2 thay vì
		// ANTHROPIC_API_KEY trên VPS) mà không có tín hiệu gì báo lỗi.
		slog.Warn("factory: ANTHROPIC_API_KEY_2 được set nhưng thiếu ANTHROPIC_API_KEY — key thứ 2 bị bỏ qua, kiểm tra lại biến môi trường")
	}

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
		// Claude key 1 (last resort fallback trước đây).
		c, err := newAnthropic(cfg)
		if err != nil {
			return nil, err
		}
		if cfg.AnthropicKey2 != "" {
			// Có key 2 → đổi tên cả 2 thành anthropic-1/anthropic-2 để log
			// phân biệt được. Không đổi tên khi CHỈ có 1 key (giữ đúng tên
			// "anthropic" cũ, backward-compat với test/log hiện có).
			providers = append(providers, namedOverride{Provider: c, name: "anthropic-1"})

			c2, err := anthropic.New(cfg.AnthropicKey2, cfg.AnthropicModel)
			if err != nil {
				return nil, err
			}
			providers = append(providers, namedOverride{Provider: c2, name: "anthropic-2"})
		} else {
			providers = append(providers, c)
		}
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

// newRouter tạo RouterProvider (internal/provider/router): request "nhẹ"
// (ThinkingLevel=OFF và không kèm tool nào) được route sang model LOCAL rẻ/
// nhanh; các request còn lại (cần tool hoặc thinking) route sang chuỗi CLOUD
// hiện có (newAuto — tái dùng nguyên vẹn, không viết lại fallback chain).
//
// Local backend chọn theo cfg.RouterLocalBackend ("ollama" mặc định, hoặc
// "openai_compat") — tái dùng đúng 2 constructor private đã có sẵn ở trên,
// không viết lại logic gọi Ollama/OpenAI-compat.
func newRouter(cfg config.Config) (provider.Provider, error) {
	cloud, err := newAuto(cfg)
	if err != nil {
		return nil, fmt.Errorf("router: cloud chain: %w", err)
	}

	var local provider.Provider
	switch cfg.RouterLocalBackend {
	case "", "ollama":
		local, err = newOllama(cfg)
	case "openai_compat":
		local, err = newOpenAICompat(cfg)
	default:
		return nil, fmt.Errorf("router: unknown ROUTER_LOCAL_BACKEND: %q (use ollama or openai_compat)", cfg.RouterLocalBackend)
	}
	if err != nil {
		return nil, fmt.Errorf("router: local backend %q: %w", cfg.RouterLocalBackend, err)
	}

	return router.New(local, cloud), nil
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
