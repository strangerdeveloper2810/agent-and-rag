// Package config nạp cấu hình từ biến môi trường (fail-fast nếu thiếu bắt buộc).
package config

import (
	"fmt"
	"os"
)

// Config chứa toàn bộ cấu hình JARVIS — từ LLM provider đến storage paths.
type Config struct {
	// Server
	Port string // default: 3002

	// Provider (LLM)
	Provider      string // "gemini" | "anthropic" | "ollama"
	GeminiKey     string
	GeminiModel   string
	ThinkingLevel string // OFF|LOW|MEDIUM|HIGH (Gemini 3.x)
	AnthropicKey  string
	AnthropicModel string

	// Ollama (local LLM)
	OllamaURL   string // default: http://localhost:11434
	OllamaModel string // default: llama3.1:8b

	// Storage
	DBPath       string // SQLite database path. default: ~/.jarvis/jarvis.db
	SkillsDir    string // Skills directory. default: ./skills
	AllowedPaths []string // File tool allowed paths. default: [".", "$HOME"]

	// Embedding
	VoyageKey string
	EmbedModel string // embedding model. default: nomic-embed-text

	// Limits
	MaxSteps       int // default: 12
	MaxTokens      int // default: 0 (unlimited)
	MaxToolOutput  int // max chars from tool output. default: 24000
	ShellTimeout   int // seconds. default: 30
}

// Load đọc env → Config với defaults hợp lý.
func Load() (Config, error) {
	c := Config{
		Port:           envOr("PORT", "3002"),
		Provider:       envOr("LLM_PROVIDER", "gemini"),
		GeminiKey:      os.Getenv("GEMINI_API_KEY"),
		GeminiModel:    envOr("GEMINI_MODEL", "gemini-2.5-flash"),
		ThinkingLevel:  envOr("GOOGLE_THINKING_LEVEL", "OFF"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel: envOr("CLAUDE_MODEL", "claude-haiku-4-5-20251001"),
		OllamaURL:      envOr("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:    envOr("OLLAMA_MODEL", "llama3.1:8b"),
		DBPath:         envOr("JARVIS_DB_PATH", "jarvis.db"),
		SkillsDir:      envOr("JARVIS_SKILLS_DIR", "./skills"),
		AllowedPaths:   []string{".", os.Getenv("HOME")},
		VoyageKey:      os.Getenv("VOYAGE_API_KEY"),
		EmbedModel:     envOr("EMBED_MODEL", "nomic-embed-text"),
		MaxSteps:       12,
		MaxTokens:      0,
		MaxToolOutput:  24000,
		ShellTimeout:   30,
	}

	// Validate: ít nhất 1 LLM provider phải được cấu hình
	hasProvider := false
	switch c.Provider {
	case "gemini":
		hasProvider = c.GeminiKey != ""
	case "anthropic":
		hasProvider = c.AnthropicKey != ""
	case "ollama":
		hasProvider = true // local, no key needed
	default:
		return Config{}, fmt.Errorf("unknown LLM_PROVIDER: %q (use gemini, anthropic, or ollama)", c.Provider)
	}
	if !hasProvider {
		return Config{}, fmt.Errorf("%s requires API key (set %s_API_KEY)", c.Provider, c.Provider)
	}

	return c, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
