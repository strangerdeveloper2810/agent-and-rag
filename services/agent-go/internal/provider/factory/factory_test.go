// Package factory tests — các constructor provider không gọi mạng lúc khởi tạo,
// nên test chỉ cần key giả để kiểm tra logic chọn provider.
package factory

import (
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/config"
)

func baseCfg() config.Config {
	return config.Config{
		GeminiModel:        "gemini-3.1-flash-lite",
		AnthropicModel:     "claude-haiku-4-5-20251001",
		DeepSeekFlashModel: "deepseek-v4-flash",
		DeepSeekProModel:   "deepseek-v4-pro",
	}
}

func TestNew_SingleProviders(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*config.Config)
		wantName string
	}{
		{
			name:     "gemini",
			mutate:   func(c *config.Config) { c.Provider = "gemini"; c.GeminiKey = "gk" },
			wantName: "gemini",
		},
		{
			name:     "anthropic",
			mutate:   func(c *config.Config) { c.Provider = "anthropic"; c.AnthropicKey = "ak" },
			wantName: "anthropic",
		},
		{
			name:     "deepseek",
			mutate:   func(c *config.Config) { c.Provider = "deepseek"; c.DeepSeekKey = "dk" },
			wantName: "deepseek",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseCfg()
			tc.mutate(&cfg)

			p, err := New(cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if p.Name() != tc.wantName {
				t.Errorf("Name() = %q, want %q", p.Name(), tc.wantName)
			}
		})
	}
}

func TestNew_MissingKeys(t *testing.T) {
	cases := []struct {
		provider string
		wantErr  string
	}{
		{"gemini", "GEMINI_API_KEY is required"},
		{"anthropic", "ANTHROPIC_API_KEY is required"},
		{"deepseek", "DEEPSEEK_API_KEY is required"},
		{"auto", "need at least one of"},
	}

	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			cfg := baseCfg()
			cfg.Provider = tc.provider

			_, err := New(cfg)
			if err == nil {
				t.Fatal("New() = nil error, want error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %q, want chứa %q", err, tc.wantErr)
			}
		})
	}
}

func TestNew_UnknownProvider(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "openai"

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() = nil error, want error")
	}
	if !strings.Contains(err.Error(), "unknown LLM_PROVIDER") {
		t.Errorf("err = %q, want chứa unknown LLM_PROVIDER", err)
	}
}

// auto + đúng 1 key (không cấu hình secondary model) → trả provider đơn, KHÔNG bọc fallback.
func TestNew_AutoSingleKeyReturnsPlainProvider(t *testing.T) {
	cases := map[string]func(*config.Config){
		"deepseek":  func(c *config.Config) { c.DeepSeekKey = "dk" },
		"gemini":    func(c *config.Config) { c.GeminiKey = "gk" },
		"anthropic": func(c *config.Config) { c.AnthropicKey = "ak" },
	}

	for wantName, mutate := range cases {
		t.Run(wantName, func(t *testing.T) {
			cfg := baseCfg()
			cfg.Provider = "auto"
			mutate(&cfg)

			p, err := New(cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if p.Name() != wantName {
				t.Errorf("Name() = %q, want %q (không bọc fallback khi chỉ 1 provider)", p.Name(), wantName)
			}
		})
	}
}

// auto + nhiều key → fallback chain theo thứ tự Gemini → DeepSeek → Claude.
func TestNew_AutoChainOrder(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.DeepSeekKey = "dk"
	cfg.AnthropicKey = "ak"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	want := "fallback[gemini→deepseek→anthropic]"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q", p.Name(), want)
	}
}

// auto + dual Gemini models → fallback chain Gemini Primary → Gemini Secondary.
func TestNew_AutoDualGeminiModels(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.GeminiSecondaryModel = "gemini-3.5-flash-lite"
	cfg.DeepSeekKey = "dk"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	want := "fallback[gemini→gemini→deepseek]"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q", p.Name(), want)
	}
}

func TestNew_AutoTwoKeys(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.AnthropicKey = "ak"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "fallback[gemini→anthropic]" {
		t.Errorf("Name() = %q, want fallback[gemini→anthropic]", p.Name())
	}
}

// Model rỗng: gemini bắt buộc có model, deepseek tự điền default.
func TestNew_EmptyModels(t *testing.T) {
	cfg := config.Config{Provider: "gemini", GeminiKey: "gk"}
	if _, err := New(cfg); err == nil {
		t.Error("gemini với model rỗng phải lỗi")
	}

	cfg = config.Config{Provider: "deepseek", DeepSeekKey: "dk"}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("deepseek với model rỗng phải tự điền default: %v", err)
	}
	if p.Name() != "deepseek" {
		t.Errorf("Name() = %q, want deepseek", p.Name())
	}
}

// auto phải trả lỗi của provider con thay vì nuốt im lặng.
func TestNew_AutoPropagatesConstructorError(t *testing.T) {
	cfg := config.Config{
		Provider:     "auto",
		GeminiKey:    "gk", // model rỗng → gemini.New lỗi
		AnthropicKey: "ak",
	}

	if _, err := New(cfg); err == nil {
		t.Fatal("New() = nil error, want lỗi từ gemini.New")
	}
}

func TestNew_AutoPropagatesAnthropicError(t *testing.T) {
	cfg := config.Config{
		Provider:     "auto",
		DeepSeekKey:  "dk",
		AnthropicKey: "ak", // AnthropicModel rỗng → anthropic.New lỗi
	}

	if _, err := New(cfg); err == nil {
		t.Fatal("New() = nil error, want lỗi từ anthropic.New")
	}
}
