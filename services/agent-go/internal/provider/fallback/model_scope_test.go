package fallback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// recordingProvider ghi lại model mà nó ĐƯỢC YÊU CẦU dùng, để test kiểm tra
// chuỗi fallback có truyền sai tên model sang provider khác họ hay không.
type recordingProvider struct {
	name      string
	seenModel string
	called    bool
	fail      error
}

func (r *recordingProvider) Name() string { return r.name }

func (r *recordingProvider) Generate(_ context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	r.called = true
	r.seenModel = req.Options.Model
	if r.fail != nil {
		return nil, r.fail
	}
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: r.name}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func drain(t *testing.T, stream <-chan provider.StreamChunk) {
	t.Helper()
	for range stream {
	}
}

// Đây là hồi quy cho một bug tốn tiền thật: fastModel() trả "deepseek-v4-flash",
// chuỗi fallback truyền nguyên tên đó cho MỌI provider, và gemini.Client tôn
// trọng Options.Model → gọi Gemini API với model không tồn tại, chắc chắn lỗi,
// rồi mới rơi xuống DeepSeek. Mỗi lượt learner/summarize đốt 2 request rác.
func TestScopeModel_KhongRoRiTenModelSangProviderKhacHo(t *testing.T) {
	gem := &recordingProvider{name: "gemini"}
	ds := &recordingProvider{name: "deepseek"}

	fb, err := New(time.Second, gem, ds)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{
		Options: provider.ProviderOptions{Model: "deepseek-v4-flash"},
	})
	if err != nil {
		t.Fatal(err)
	}
	drain(t, stream)

	// Gemini vẫn được gọi trước (nó là primary) nhưng PHẢI dùng model mặc định
	// của chính nó, không phải tên model của DeepSeek.
	if !gem.called {
		t.Fatal("gemini phải được gọi (nó là primary)")
	}
	if gem.seenModel != "" {
		t.Errorf("gemini nhận model %q — override của provider khác họ phải bị bỏ", gem.seenModel)
	}
}

func TestScopeModel_GiuOverrideChoDungProvider(t *testing.T) {
	// Primary lỗi để chuỗi rơi xuống deepseek, nơi override PHẢI được giữ.
	gem := &recordingProvider{name: "gemini", fail: errors.New("429 Too Many Requests")}
	ds := &recordingProvider{name: "deepseek"}

	fb, err := New(time.Second, gem, ds)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{
		Options: provider.ProviderOptions{Model: "deepseek-v4-flash"},
	})
	if err != nil {
		t.Fatal(err)
	}
	drain(t, stream)

	if ds.seenModel != "deepseek-v4-flash" {
		t.Errorf("deepseek nhận model %q, muốn %q — model đúng họ phải được giữ",
			ds.seenModel, "deepseek-v4-flash")
	}
}

func TestModelFamily(t *testing.T) {
	cases := []struct {
		model string
		want  string
	}{
		{"", ""},
		{"gemini-3.1-flash-lite", "gemini"},
		{"models/gemini-3.5-flash-lite", "gemini"},
		{"gemma-3-27b", "gemini"},
		{"deepseek-v4-flash", "deepseek"},
		{"deepseek-v4-pro", "deepseek"},
		{"claude-haiku-4-5-20251001", "anthropic"},
		// Model tự host / tên lạ: không nhận ra họ thì KHÔNG được đoán, cứ để
		// nguyên override cho provider tự xử lý (vd ollama chạy model bất kỳ).
		{"llama3.2:3b", ""},
		{"my-finetune-v2", ""},
	}
	for _, tc := range cases {
		if got := modelFamily(tc.model); got != tc.want {
			t.Errorf("modelFamily(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestScopeModel_ModelLaThiGiuNguyen(t *testing.T) {
	req := provider.GenerateRequest{
		Options: provider.ProviderOptions{Model: "llama3.2:3b"},
	}
	if got := scopeModel(req, "ollama").Options.Model; got != "llama3.2:3b" {
		t.Errorf("model không nhận ra họ phải giữ nguyên, got %q", got)
	}
}
