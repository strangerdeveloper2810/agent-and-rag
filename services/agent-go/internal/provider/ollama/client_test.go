package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, "llama3.1:8b")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func drain(t *testing.T, stream <-chan provider.StreamChunk) []provider.StreamChunk {
	t.Helper()
	var out []provider.StreamChunk
	for c := range stream {
		out = append(out, c)
	}
	return out
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	c, err := New("http://localhost:11434/", "m")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %q, want không có dấu / cuối", c.baseURL)
	}
	if c.Name() != "ollama" {
		t.Errorf("Name() = %q, want ollama", c.Name())
	}
}

func TestGenerate_StreamsTextAndDone(t *testing.T) {
	var got ollamaChatRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %q, want /api/chat", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)

		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"xin "},"done":false}
{"message":{"role":"assistant","content":"chào"},"done":false}
{"done":true,"done_reason":"stop"}
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		System:   "sys",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		Options:  provider.ProviderOptions{MaxTokens: 128},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var text strings.Builder
	var done bool
	for _, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkText:
			text.WriteString(ch.Text)
		case provider.ChunkDone:
			done = true
			if ch.FinishReason != provider.FinishStop {
				t.Errorf("FinishReason = %q, want stop", ch.FinishReason)
			}
		}
	}

	if text.String() != "xin chào" {
		t.Errorf("text = %q, want %q", text.String(), "xin chào")
	}
	if !done {
		t.Error("thiếu ChunkDone")
	}

	// System phải nằm ở message đầu; MaxTokens map sang options.num_predict.
	if len(got.Messages) != 2 || got.Messages[0].Role != "system" {
		t.Errorf("messages = %+v", got.Messages)
	}
	if got.Options == nil || got.Options.NumPredict != 128 {
		t.Errorf("options = %+v, want num_predict 128", got.Options)
	}
	if !got.Stream {
		t.Error("stream = false, want true")
	}
}

func TestGenerate_ToolCall(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"message":{"tool_calls":[{"id":"c1","function":{"name":"echo","arguments":{"text":"hi"}}}]},"done":false}
{"done":true}
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "echo hi"}},
		Tools:    []provider.ToolDef{{Name: "echo", Description: "vọng", Schema: json.RawMessage(`{"type":"object"}`)}},
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
	if call.ID != "c1" || call.Name != "echo" || string(call.Args) != `{"text":"hi"}` {
		t.Errorf("tool call = %+v", call)
	}
}

func TestGenerate_MalformedLineEmitsError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("không-phải-json\n{\"done\":true}\n"))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var sawError, sawDone bool
	for _, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkError:
			sawError = true
		case provider.ChunkDone:
			sawDone = true
		}
	}

	if !sawError {
		t.Error("dòng hỏng phải phát ChunkError")
	}
	if !sawDone {
		t.Error("stream phải tiếp tục sau dòng hỏng")
	}
}

func TestGenerate_UnreachableHost(t *testing.T) {
	c, err := New("http://127.0.0.1:1", "m")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.Generate(context.Background(), provider.GenerateRequest{}); err == nil {
		t.Fatal("Generate = nil error, want lỗi kết nối")
	}
}

func TestGenerate_ContextCancelled(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{\"done\":true}\n"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Generate(ctx, provider.GenerateRequest{}); err == nil {
		t.Fatal("Generate với ctx đã huỷ = nil error, want lỗi")
	}
}

// --- Embed ---

func TestEmbed(t *testing.T) {
	var got embedRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %q, want /api/embed", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2],[0.3,0.4]]}`))
	})

	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 2 || vecs[1][1] != 0.4 {
		t.Errorf("embeddings = %v", vecs)
	}
	if got.Model != "nomic-embed-text" || len(got.Input) != 2 {
		t.Errorf("request = %+v", got)
	}
}

func TestEmbed_DecodeError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("không-phải-json"))
	})

	if _, err := c.Embed(context.Background(), []string{"a"}); err == nil {
		t.Fatal("Embed = nil error, want lỗi decode")
	}
}

func TestEmbed_UnreachableHost(t *testing.T) {
	c, err := New("http://127.0.0.1:1", "m")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.Embed(context.Background(), []string{"a"}); err == nil {
		t.Fatal("Embed = nil error, want lỗi kết nối")
	}
}

// --- toOllamaMessages ---

func TestToOllamaMessages_AssistantWithToolCalls(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "hi"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
			{ID: "c1", Name: "echo", Args: json.RawMessage(`{"text":"x"}`)},
		}},
		{Role: provider.RoleTool, ToolCallID: "c1", Content: "x"},
	}

	got := toOllamaMessages(msgs)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if len(got[1].ToolCalls) != 1 || got[1].ToolCalls[0].Function.Name != "echo" {
		t.Errorf("assistant tool calls = %+v", got[1])
	}
	// Assistant chỉ có tool call thì content phải rỗng.
	if got[1].Content != "" {
		t.Errorf("content = %q, want rỗng", got[1].Content)
	}
	if got[2].Content != "x" {
		t.Errorf("tool result content = %q, want x", got[2].Content)
	}
}

func TestToOllamaTools_NilInput(t *testing.T) {
	if got := toOllamaTools(nil); got != nil {
		t.Errorf("toOllamaTools(nil) = %+v, want nil", got)
	}
}
