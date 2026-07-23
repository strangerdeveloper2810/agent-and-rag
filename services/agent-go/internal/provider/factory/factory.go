// Package factory chọn Provider theo config (tách package để tránh import cycle:
// gemini/anthropic import provider; factory import cả ba).
package factory

import (
	"errors"

	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/provider/anthropic"
	"github.com/ai-agent-tut/agent-go/internal/provider/gemini"
)

// New tạo Provider theo cfg.Provider ("gemini" | "anthropic").
func New(cfg config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "gemini":
		if cfg.GeminiKey == "" {
			return nil, errors.New("GEMINI_API_KEY required for provider=gemini")
		}
		c, err := gemini.New(cfg.GeminiKey, cfg.GeminiModel, provider.ThinkingLevel(cfg.ThinkingLevel))
		if err != nil {
			return nil, err
		}
		return c, nil
	case "anthropic":
		if cfg.AnthropicKey == "" {
			return nil, errors.New("ANTHROPIC_API_KEY required for provider=anthropic")
		}
		c, err := anthropic.New(cfg.AnthropicKey, cfg.AnthropicModel)
		if err != nil {
			return nil, err
		}
		return c, nil
	default:
		return nil, errors.New("unknown LLM_PROVIDER: " + cfg.Provider)
	}
}
