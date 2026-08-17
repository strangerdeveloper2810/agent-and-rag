package config

import (
	"os"
	"strings"
	"testing"
)

// clearEnv xoá mọi biến môi trường Load() đọc, để mỗi test bắt đầu từ trạng thái
// sạch bất kể môi trường máy chạy test. t.Setenv tự khôi phục sau test.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"PORT", "LLM_PROVIDER", "GEMINI_API_KEY", "GOOGLE_API_KEY", "GEMINI_MODEL",
		"GOOGLE_THINKING_LEVEL", "ANTHROPIC_API_KEY", "CLAUDE_MODEL",
		"OLLAMA_URL", "OLLAMA_MODEL", "DEEPSEEK_API_KEY", "DEEPSEEK_FLASH_MODEL",
		"DEEPSEEK_PRO_MODEL", "JARVIS_DB_PATH", "JARVIS_SKILLS_DIR",
		"MONGODB_URI", "MONGODB_DB", "VOYAGE_API_KEY", "EMBED_MODEL",
		"ENABLE_HYBRID_SEARCH", "ENABLE_RERANK", "ENABLE_DYNAMIC_THINKING",
		"ENABLE_PLANNING", "HOME",
	} {
		t.Setenv(k, "")
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("CFG_TEST_KEY", "value")
	if got := envOr("CFG_TEST_KEY", "fallback"); got != "value" {
		t.Errorf("envOr with set var = %q, want %q", got, "value")
	}

	t.Setenv("CFG_TEST_KEY", "")
	if got := envOr("CFG_TEST_KEY", "fallback"); got != "fallback" {
		t.Errorf("envOr with empty var = %q, want %q", got, "fallback")
	}

	if got := envOr("CFG_TEST_MISSING_KEY", "fallback"); got != "fallback" {
		t.Errorf("envOr with unset var = %q, want %q", got, "fallback")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("LLM_PROVIDER", "ollama") // local, không cần key

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	checks := map[string]struct{ got, want string }{
		"Port":               {c.Port, "3002"},
		"GeminiModel":        {c.GeminiModel, "gemini-2.5-flash"},
		"ThinkingLevel":      {c.ThinkingLevel, "OFF"},
		"AnthropicModel":     {c.AnthropicModel, "claude-haiku-4-5-20251001"},
		"OllamaURL":          {c.OllamaURL, "http://localhost:11434"},
		"OllamaModel":        {c.OllamaModel, "llama3.1:8b"},
		"DeepSeekFlashModel": {c.DeepSeekFlashModel, "deepseek-v4-flash"},
		"DeepSeekProModel":   {c.DeepSeekProModel, "deepseek-v4-pro"},
		"DBPath":             {c.DBPath, "jarvis.db"},
		"SkillsDir":          {c.SkillsDir, "./skills"},
		"MongoDB":            {c.MongoDB, "ai_agent_tut"},
		"EmbedModel":         {c.EmbedModel, "nomic-embed-text"},
	}
	for name, ck := range checks {
		if ck.got != ck.want {
			t.Errorf("%s = %q, want %q", name, ck.got, ck.want)
		}
	}

	if c.MaxSteps != 12 || c.MaxTokens != 0 || c.MaxContextTokens != 100000 ||
		c.MaxToolOutput != 24000 || c.ShellTimeout != 30 {
		t.Errorf("limits = %+v, want defaults (12/0/100000/24000/30)", c)
	}

	// Bool defaults: hybrid search + rerank BẬT, dynamic thinking + planning TẮT.
	if !c.EnableHybridSearch || !c.EnableRerank {
		t.Errorf("EnableHybridSearch=%v EnableRerank=%v, want both true", c.EnableHybridSearch, c.EnableRerank)
	}
	if c.EnableDynamicThinking || c.EnablePlanning {
		t.Errorf("EnableDynamicThinking=%v EnablePlanning=%v, want both false",
			c.EnableDynamicThinking, c.EnablePlanning)
	}

	if len(c.AllowedPaths) != 2 || c.AllowedPaths[0] != "." {
		t.Errorf("AllowedPaths = %v, want [. $HOME]", c.AllowedPaths)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("LLM_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "gk")
	t.Setenv("PORT", "9999")
	t.Setenv("GEMINI_MODEL", "gemini-3-pro")
	t.Setenv("GOOGLE_THINKING_LEVEL", "HIGH")
	t.Setenv("JARVIS_DB_PATH", "/tmp/x.db")
	t.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("MONGODB_DB", "other_db")
	t.Setenv("VOYAGE_API_KEY", "vk")
	t.Setenv("ENABLE_HYBRID_SEARCH", "false")
	t.Setenv("ENABLE_RERANK", "false")
	t.Setenv("ENABLE_DYNAMIC_THINKING", "true")
	t.Setenv("ENABLE_PLANNING", "true")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Port != "9999" || c.GeminiModel != "gemini-3-pro" || c.ThinkingLevel != "HIGH" {
		t.Errorf("overrides not applied: %+v", c)
	}
	if c.DBPath != "/tmp/x.db" || c.MongoURI != "mongodb://localhost:27017" || c.MongoDB != "other_db" {
		t.Errorf("storage overrides not applied: %+v", c)
	}
	if c.VoyageKey != "vk" {
		t.Errorf("VoyageKey = %q, want vk", c.VoyageKey)
	}
	if c.EnableHybridSearch || c.EnableRerank {
		t.Error("ENABLE_HYBRID_SEARCH/ENABLE_RERANK=false phải tắt cờ")
	}
	if !c.EnableDynamicThinking || !c.EnablePlanning {
		t.Error("ENABLE_DYNAMIC_THINKING/ENABLE_PLANNING=true phải bật cờ")
	}
}

// GEMINI_API_KEY trống thì rơi về GOOGLE_API_KEY.
func TestLoad_GeminiKeyFallsBackToGoogleKey(t *testing.T) {
	clearEnv(t)
	t.Setenv("LLM_PROVIDER", "gemini")
	t.Setenv("GOOGLE_API_KEY", "google-key")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.GeminiKey != "google-key" {
		t.Errorf("GeminiKey = %q, want google-key", c.GeminiKey)
	}
}

func TestLoad_ProviderValidation(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "gemini thiếu key",
			env:     map[string]string{"LLM_PROVIDER": "gemini"},
			wantErr: "requires API key",
		},
		{
			name:    "anthropic thiếu key",
			env:     map[string]string{"LLM_PROVIDER": "anthropic"},
			wantErr: "requires API key",
		},
		{
			name:    "deepseek thiếu key",
			env:     map[string]string{"LLM_PROVIDER": "deepseek"},
			wantErr: "requires API key",
		},
		{
			name:    "auto không có key nào",
			env:     map[string]string{"LLM_PROVIDER": "auto"},
			wantErr: "requires API key",
		},
		{
			name:    "provider lạ",
			env:     map[string]string{"LLM_PROVIDER": "openai"},
			wantErr: "unknown LLM_PROVIDER",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("Load() = nil error, want error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %q, want chứa %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoad_ProviderAccepted(t *testing.T) {
	cases := []struct {
		provider string
		key      string
	}{
		{"gemini", "GEMINI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"deepseek", "DEEPSEEK_API_KEY"},
		{"auto", "DEEPSEEK_API_KEY"},
		{"auto", "GEMINI_API_KEY"},
		{"auto", "ANTHROPIC_API_KEY"},
	}

	for _, tc := range cases {
		t.Run(tc.provider+"/"+tc.key, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("LLM_PROVIDER", tc.provider)
			t.Setenv(tc.key, "secret")

			c, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if c.Provider != tc.provider {
				t.Errorf("Provider = %q, want %q", c.Provider, tc.provider)
			}
		})
	}
}

// ollama chạy local nên không cần key nào.
func TestLoad_OllamaNeedsNoKey(t *testing.T) {
	clearEnv(t)
	t.Setenv("LLM_PROVIDER", "ollama")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load with ollama: %v", err)
	}
	if c.Provider != "ollama" {
		t.Errorf("Provider = %q, want ollama", c.Provider)
	}
}

func TestLoad_AllowedPathsIncludesHome(t *testing.T) {
	clearEnv(t)
	t.Setenv("LLM_PROVIDER", "ollama")
	t.Setenv("HOME", "/home/tester")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.AllowedPaths[1] != "/home/tester" {
		t.Errorf("AllowedPaths[1] = %q, want /home/tester", c.AllowedPaths[1])
	}
	if os.Getenv("HOME") != "/home/tester" {
		t.Fatal("t.Setenv(HOME) không có hiệu lực")
	}
}
