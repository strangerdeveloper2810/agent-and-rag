// Package config nạp cấu hình từ biến môi trường (fail-fast nếu thiếu bắt buộc).
package config

import (
	"errors"
	"os"
)

type Config struct {
	Port string

	MongoURI string
	MongoDB  string

	Provider string // "gemini" | "anthropic"

	GeminiKey     string
	GeminiModel   string
	ThinkingLevel string // OFF|LOW|MEDIUM|HIGH (Gemini 3.x)

	AnthropicKey   string
	AnthropicModel string

	VoyageKey string
}

// Load đọc env → Config. Thiếu MONGODB_URI là lỗi.
func Load() (Config, error) {
	c := Config{
		Port:           envOr("PORT", "3002"),
		MongoURI:       os.Getenv("MONGODB_URI"),
		MongoDB:        envOr("MONGODB_DB", "ai_agent_tut"),
		Provider:       envOr("LLM_PROVIDER", "gemini"),
		GeminiKey:      os.Getenv("GEMINI_API_KEY"),
		GeminiModel:    envOr("GEMINI_MODEL", "gemini-3.1-flash-lite"),
		ThinkingLevel:  envOr("GOOGLE_THINKING_LEVEL", "LOW"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel: envOr("CLAUDE_MODEL", "claude-haiku-4-5-20251001"),
		VoyageKey:      os.Getenv("VOYAGE_API_KEY"),
	}
	if c.MongoURI == "" {
		return Config{}, errors.New("MONGODB_URI is required")
	}
	return c, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
