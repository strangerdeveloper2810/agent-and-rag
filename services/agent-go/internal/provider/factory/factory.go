// Package factory chọn Provider theo config.
// Hỗ trợ chế độ "auto": nếu cả Gemini và Claude key đều có → tạo fallback chain.
package factory

import (
	"fmt"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/provider/anthropic"
	"github.com/ai-agent-tut/agent-go/internal/provider/fallback"
	"github.com/ai-agent-tut/agent-go/internal/provider/gemini"
)

// New tạo Provider theo cfg.Provider:
//
//	"gemini"    → Gemini (single)
//	"anthropic" → Claude (single)
//	"auto"      → Fallback chain: Gemini → Claude (nếu có cả 2 key)
//	               Nếu chỉ có 1 key → dùng single provider đó
func New(cfg config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "gemini":
		return newGemini(cfg)
	case "anthropic":
		return newAnthropic(cfg)
	case "auto":
		return newAuto(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %q (use gemini, anthropic, or auto)", cfg.Provider)
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

// newAuto tạo provider chain thông minh:
// - Có cả 2 key → Gemini (primary) → Claude (fallback)
// - Chỉ có Gemini → Gemini single
// - Chỉ có Claude → Claude single
// - Không có key nào → lỗi
func newAuto(cfg config.Config) (provider.Provider, error) {
	hasGemini := cfg.GeminiKey != ""
	hasClaude := cfg.AnthropicKey != ""

	switch {
	case hasGemini && hasClaude:
		gem, err := newGemini(cfg)
		if err != nil {
			return nil, err
		}
		cla, err := newAnthropic(cfg)
		if err != nil {
			return nil, err
		}
		return fallback.New(30*time.Second, gem, cla)

	case hasGemini:
		return newGemini(cfg)

	case hasClaude:
		return newAnthropic(cfg)

	default:
		return nil, fmt.Errorf("auto provider: need at least GEMINI_API_KEY or ANTHROPIC_API_KEY")
	}
}
