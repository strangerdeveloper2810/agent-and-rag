package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// newTestClient dựng Client trỏ genai SDK vào httptest server (New() dùng
// endpoint thật nên test tự lắp genai.Client).
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	gc, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:      "test-key",
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{BaseURL: srv.URL},
	})
	if err != nil {
		t.Fatalf("genai.NewClient: %v", err)
	}
	return &Client{client: gc, model: "gemini-2.5-flash"}
}

func drain(t *testing.T, stream <-chan provider.StreamChunk) []provider.StreamChunk {
	t.Helper()
	var out []provider.StreamChunk
	for c := range stream {
		out = append(out, c)
	}
	return out
}

// --- Constructor ---

func TestNew_Validation(t *testing.T) {
	if _, err := New("", "m", provider.ThinkingOff); err == nil {
		t.Error("New với apiKey rỗng phải lỗi")
	}
	if _, err := New("k", "", provider.ThinkingOff); err == nil {
		t.Error("New với model rỗng phải lỗi")
	}

	c, err := New("k", "gemini-2.5-flash", provider.ThinkingHigh)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Name() != "gemini" {
		t.Errorf("Name() = %q, want gemini", c.Name())
	}
	if c.thinking != provider.ThinkingHigh {
		t.Errorf("thinking = %q, want HIGH", c.thinking)
	}
}

// --- Generate ---

func TestGenerate_TextAndUsage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "streamGenerateContent") {
			t.Errorf("path = %q, want streamGenerateContent", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"xin "}]}}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"chào"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}

`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		System:   "sys",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var text strings.Builder
	var usage *provider.Usage
	var done *provider.StreamChunk
	for _, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkText:
			text.WriteString(ch.Text)
		case provider.ChunkUsage:
			usage = ch.Usage
		case provider.ChunkDone:
			cp := ch
			done = &cp
		case provider.ChunkError:
			t.Fatalf("chunk lỗi: %v", ch.Err)
		}
	}

	if text.String() != "xin chào" {
		t.Errorf("text = %q, want %q", text.String(), "xin chào")
	}
	if usage == nil || usage.InputTokens != 5 || usage.OutputTokens != 2 {
		t.Errorf("usage = %+v, want {5 2}", usage)
	}
	if done == nil || done.FinishReason != provider.FinishStop {
		t.Errorf("done = %+v, want FinishReason stop", done)
	}
}

// finishReason MAX_TOKENS → FinishLength (câu trả lời bị cắt).
func TestGenerate_TruncatedByMaxTokens(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"cụt"}]},"finishReason":"MAX_TOKENS"}]}

`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Options: provider.ProviderOptions{MaxTokens: 4},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var done *provider.StreamChunk
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkDone {
			cp := ch
			done = &cp
		}
	}

	if done == nil || done.FinishReason != provider.FinishLength {
		t.Errorf("done = %+v, want FinishReason length", done)
	}
}

func TestGenerate_FunctionCall(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"id":"fc1","name":"web.search","args":{"q":"go"}}}]},"finishReason":"STOP"}]}

`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "tìm go"}},
		Tools:    []provider.ToolDef{{Name: "web.search", Description: "tìm"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var call *provider.ToolCall
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkToolCall {
			call = ch.ToolCall
		}
	}

	if call == nil {
		t.Fatal("không nhận được tool call")
	}
	if call.Name != "web.search" || call.ID != "fc1" {
		t.Errorf("tool call = %+v", call)
	}
	if !strings.Contains(string(call.Args), `"go"`) {
		t.Errorf("args = %s", call.Args)
	}
}

// Part có Thought=true là suy nghĩ nội bộ — không được stream ra ngoài.
func TestGenerate_SkipsThoughtParts(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"nghĩ thầm","thought":true},{"text":"nói ra"}]},"finishReason":"STOP"}]}

`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var text strings.Builder
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkText {
			text.WriteString(ch.Text)
		}
	}
	if text.String() != "nói ra" {
		t.Errorf("text = %q, want %q (bỏ phần thought)", text.String(), "nói ra")
	}
}

func TestGenerate_HTTPErrorEmitsErrorChunk(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"quota"}}`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var sawError bool
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkError {
			sawError = true
		}
	}
	if !sawError {
		t.Error("lỗi HTTP phải phát ChunkError")
	}
}

func TestGenerate_NilClient(t *testing.T) {
	c := &Client{model: "m"}
	if _, err := c.Generate(context.Background(), provider.GenerateRequest{}); err == nil {
		t.Fatal("Generate với client nil = nil error, want lỗi")
	}
}

// Có tool function → KHÔNG được bật Google Search built-in (Gemini cấm trộn).
func TestGenerate_ThinkingAndCacheOptions(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}

`))
	})
	c.SetCacheName("cachedContents/abc")

	if c.CacheName() != "cachedContents/abc" {
		t.Errorf("CacheName() = %q", c.CacheName())
	}

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingMedium},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	drain(t, stream)

	c.SetCacheName("")
	if c.CacheName() != "" {
		t.Errorf("CacheName() sau khi xoá = %q, want rỗng", c.CacheName())
	}
}

// --- CreateCachedContent ---

func TestCreateCachedContent_NilClient(t *testing.T) {
	c := &Client{model: "m"}
	if _, err := c.CreateCachedContent(context.Background(), "sys", nil); err == nil {
		t.Fatal("CreateCachedContent với client nil = nil error, want lỗi")
	}
}

func TestCreateCachedContent_EmptyPromptIsNoop(t *testing.T) {
	c := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("không được gọi API khi systemPrompt rỗng")
	})

	name, err := c.CreateCachedContent(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("CreateCachedContent: %v", err)
	}
	if name != "" {
		t.Errorf("name = %q, want rỗng", name)
	}
}

func TestCreateCachedContent_Success(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"cachedContents/xyz","expireTime":"2030-01-01T00:00:00Z"}`))
	})

	name, err := c.CreateCachedContent(context.Background(), "sys prompt",
		[]provider.ToolDef{{Name: "echo", Description: "vọng"}})
	if err != nil {
		t.Fatalf("CreateCachedContent: %v", err)
	}
	if name != "cachedContents/xyz" {
		t.Errorf("name = %q, want cachedContents/xyz", name)
	}
}

// Lỗi tạo cache phải degrade êm: trả "" và KHÔNG lỗi (caller chạy tiếp không cache).
func TestCreateCachedContent_FailsGracefully(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"boom"}}`))
	})

	name, err := c.CreateCachedContent(context.Background(), "sys", nil)
	if err != nil {
		t.Errorf("err = %v, want nil (graceful degradation)", err)
	}
	if name != "" {
		t.Errorf("name = %q, want rỗng", name)
	}
}

// --- findToolName / findThoughtSignature ---

func TestFindToolName(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "hi"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "echo"}}},
	}

	if got := findToolName(msgs, "c1"); got != "echo" {
		t.Errorf("findToolName = %q, want echo", got)
	}
	if got := findToolName(msgs, "unknown"); got != "" {
		t.Errorf("findToolName với id lạ = %q, want rỗng", got)
	}
	if got := findToolName(nil, "c1"); got != "" {
		t.Errorf("findToolName(nil) = %q, want rỗng", got)
	}
}

func TestFindThoughtSignature(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
			{ID: "c1", Name: "echo", ThoughtSignature: []byte("sig")},
			{ID: "c2", Name: "echo2"},
		}},
	}

	if got := findThoughtSignature(msgs, "c1"); string(got) != "sig" {
		t.Errorf("findThoughtSignature = %q, want sig", got)
	}
	// Tool call không có signature → nil.
	if got := findThoughtSignature(msgs, "c2"); got != nil {
		t.Errorf("findThoughtSignature(c2) = %q, want nil", got)
	}
	if got := findThoughtSignature(msgs, "unknown"); got != nil {
		t.Errorf("findThoughtSignature id lạ = %q, want nil", got)
	}
}
