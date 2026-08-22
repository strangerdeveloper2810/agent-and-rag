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

// auto + full Gemini fallback pool → chuỗi gồm nhiều model Gemini + DeepSeek + Claude
func TestNew_AutoFallbackGeminiPool(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.GeminiFallbackModels = []string{"gemini-3.5-flash-lite", "gemini-3.7-flash", "gemini-2.5-flash"}
	cfg.DeepSeekKey = "dk"
	cfg.AnthropicKey = "ak"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 4 gemini (1 primary + 3 fallback) + 1 deepseek + 1 anthropic = 6 providers
	want := "fallback[gemini→gemini→gemini→gemini→deepseek→anthropic]"
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

// NewReflectionProvider phải trả DeepSeek đơn (không bọc chain Gemini) khi có
// DEEPSEEK_API_KEY — reflection là tác vụ nền, không nên cạnh tranh quota
// Gemini với luồng chat chính (bug đã thấy trong log prod: reflection cascade
// qua 6+ biến thể Gemini trước khi rơi xuống DeepSeek).
func TestNewReflectionProvider_PrefersDeepSeek(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.DeepSeekKey = "dk"
	cfg.AnthropicKey = "ak"

	p, err := NewReflectionProvider(cfg)
	if err != nil {
		t.Fatalf("NewReflectionProvider: %v", err)
	}
	if p.Name() != "deepseek" {
		t.Errorf("Name() = %q, want %q (phải dùng DeepSeek đơn, không chain Gemini)", p.Name(), "deepseek")
	}
}

// Không có DEEPSEEK_API_KEY → fallback về provider chính (factory.New), giữ
// hành vi cũ, không được lỗi hay trả nil.
func TestNewReflectionProvider_FallsBackWithoutDeepSeek(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.AnthropicKey = "ak"

	p, err := NewReflectionProvider(cfg)
	if err != nil {
		t.Fatalf("NewReflectionProvider: %v", err)
	}
	if p.Name() != "fallback[gemini→anthropic]" {
		t.Errorf("Name() = %q, want fallback[gemini→anthropic] (fallback về provider chính khi thiếu DeepSeek key)", p.Name())
	}
}

// auto + Claude key thứ 2 → chain thêm 1 tầng Claude, tên phân biệt để dễ
// debug log (anthropic-1/anthropic-2) thay vì trùng tên "anthropic".
func TestNew_AutoWithSecondAnthropicKey(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.AnthropicKey = "ak1"
	cfg.AnthropicKey2 = "ak2"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want := "fallback[gemini→anthropic-1→anthropic-2]"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q", p.Name(), want)
	}
}

// Không set AnthropicKey2 → hành vi CŨ y nguyên (backward-compat), tên vẫn
// là "anthropic" (không bị đổi thành "anthropic-1" khi chỉ có 1 key).
func TestNew_AutoWithoutSecondAnthropicKey_Unchanged(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.AnthropicKey = "ak1"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want := "fallback[gemini→anthropic]"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q (không set key 2 phải giữ nguyên tên cũ)", p.Name(), want)
	}
}

// provider="router" với RouterLocalBackend mặc định (rỗng → "ollama") phải
// wire đúng: local=ollama, cloud=chain "auto" hiện có, không lỗi "unknown
// LLM_PROVIDER".
func TestNew_RouterDefaultsToOllamaLocal(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "router"
	cfg.GeminiKey = "gk"
	cfg.DeepSeekKey = "dk"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want := "router(local=ollama,cloud=fallback[gemini→deepseek])"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q", p.Name(), want)
	}
}

// provider="router" + RouterLocalBackend="openai_compat" phải dùng
// openai_compat làm local backend thay vì ollama.
func TestNew_RouterWithOpenAICompatLocal(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "router"
	cfg.RouterLocalBackend = "openai_compat"
	cfg.OpenAICompatBaseURL = "http://localhost:8000/v1"
	cfg.OpenAICompatModel = "local-model"
	cfg.DeepSeekKey = "dk"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want := "router(local=openai_compat,cloud=deepseek)"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q", p.Name(), want)
	}
}

// provider="router" nhưng cloud chain không có key nào → lỗi propagate từ
// newAuto, không bị nuốt im lặng.
func TestNew_RouterPropagatesCloudChainError(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "router"

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() = nil error, want lỗi từ newAuto (cloud chain thiếu key)")
	}
	if !strings.Contains(err.Error(), "need at least one of") {
		t.Errorf("err = %q, want chứa lỗi cloud chain thiếu key", err)
	}
}

// RouterLocalBackend không hợp lệ → lỗi rõ ràng, không mặc định âm thầm về ollama.
func TestNew_RouterUnknownLocalBackend(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "router"
	cfg.RouterLocalBackend = "vllm-direct"
	cfg.DeepSeekKey = "dk"

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() = nil error, want lỗi ROUTER_LOCAL_BACKEND không hợp lệ")
	}
	if !strings.Contains(err.Error(), "ROUTER_LOCAL_BACKEND") {
		t.Errorf("err = %q, want chứa ROUTER_LOCAL_BACKEND", err)
	}
}
