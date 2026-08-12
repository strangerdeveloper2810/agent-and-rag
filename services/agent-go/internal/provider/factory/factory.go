// Package factory chọn Provider theo config.
// Hỗ trợ chế độ "auto": Gemini → DeepSeek → Claude fallback chain.
// DeepSeek đóng vai trò immediate fallback rẻ tiền khi Gemini bị rate limit.
package factory

import (
	"fmt"

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
//	"auto"      → Fallback chain: Gemini → DeepSeek (nếu có key) → Claude (nếu có key)
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
//	Order: DeepSeek (primary cost-effective) → Gemini (fast secondary) → Claude (last resort)
//	Chỉ cần ít nhất 1 provider có key.
func newAuto(cfg config.Config) (provider.Provider, error) {
	hasDeepSeek := cfg.DeepSeekKey != ""
	hasGemini := cfg.GeminiKey != ""
	hasClaude := cfg.AnthropicKey != ""

	providers := make([]provider.Provider, 0, 3)

	if hasDeepSeek {
		d, err := newDeepSeek(cfg)
		if err != nil {
			return nil, err
		}
		providers = append(providers, d)
	}
	if hasGemini {
		g, err := newGemini(cfg)
		if err != nil {
			return nil, err
		}
		providers = append(providers, g)
	}
	if hasClaude {
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
		// sử dụng zero cooldown cho immediate failover khi rate limit
		return fallback.New(0, providers...)
	}
}
