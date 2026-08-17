package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestToOllamaMessages_Basic(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "Hello"},
		{Role: provider.RoleAssistant, Content: "Hi there!"},
	}
	result := toOllamaMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].Role != "user" || result[0].Content != "Hello" {
		t.Errorf("msg[0] = %+v", result[0])
	}
	if result[1].Role != "assistant" || result[1].Content != "Hi there!" {
		t.Errorf("msg[1] = %+v", result[1])
	}
}

func TestToOllamaMessages_WithToolCalls(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, Content: "Let me check", ToolCalls: []provider.ToolCall{
			{ID: "c1", Name: "echo", Args: json.RawMessage(`{"x":1}`)},
		}},
	}
	result := toOllamaMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if len(result[0].ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(result[0].ToolCalls))
	}
	if result[0].ToolCalls[0].Function.Name != "echo" {
		t.Errorf("tool name = %q, want echo", result[0].ToolCalls[0].Function.Name)
	}
}

func TestToOllamaTools(t *testing.T) {
	tools := []provider.ToolDef{
		{Name: "echo", Description: "Echo tool", Schema: json.RawMessage(`{"type":"object"}`)},
	}
	result := toOllamaTools(tools)
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0].Function.Name != "echo" {
		t.Errorf("name = %q, want echo", result[0].Function.Name)
	}
}

func TestToOllamaTools_Empty(t *testing.T) {
	if result := toOllamaTools(nil); result != nil {
		t.Errorf("expected nil for empty tools, got %v", result)
	}
}

func TestFromOllamaChunk_Text(t *testing.T) {
	chunk, err := fromOllamaChunk([]byte(`{"message":{"role":"assistant","content":"Hello"},"done":false}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Kind != provider.ChunkText || chunk.Text != "Hello" {
		t.Errorf("chunk = %+v, want text 'Hello'", chunk)
	}
}

func TestFromOllamaChunk_Done(t *testing.T) {
	chunk, err := fromOllamaChunk([]byte(`{"message":{"role":"assistant","content":""},"done":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Kind != provider.ChunkDone {
		t.Errorf("chunk.Kind = %v, want ChunkDone", chunk.Kind)
	}
}

func TestFromOllamaChunk_ToolCall(t *testing.T) {
	chunk, err := fromOllamaChunk([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"echo","arguments":{"x":1}}}]},"done":false}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Kind != provider.ChunkToolCall {
		t.Fatalf("chunk.Kind = %v, want ChunkToolCall", chunk.Kind)
	}
	if chunk.ToolCall.Name != "echo" {
		t.Errorf("tool name = %q, want echo", chunk.ToolCall.Name)
	}
}

func TestGenerate_Streaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		chunks := []string{
			`{"message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"message":{"role":"assistant","content":" World"},"done":false}`,
			`{"message":{"role":"assistant","content":""},"done":true}`,
		}
		for _, c := range chunks {
			w.Write([]byte(c + "\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "test-model")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Generate(ctx, provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var texts []string
	for chunk := range stream {
		if chunk.Kind == provider.ChunkText {
			texts = append(texts, chunk.Text)
		}
		if chunk.Kind == provider.ChunkError {
			t.Fatalf("unexpected error: %v", chunk.Err)
		}
	}

	combined := strings.Join(texts, "")
	if combined != "Hello World" {
		t.Errorf("combined = %q, want 'Hello World'", combined)
	}
}

func TestGenerate_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never finish — test timeout
		select {}
	}))
	defer server.Close()

	client, _ := New(server.URL, "test-model")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before Generate

	_, err := client.Generate(ctx, provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestFromOllamaChunk_DoneReason(t *testing.T) {
	cases := map[string]provider.FinishReason{
		`{"done":true,"done_reason":"length"}`: provider.FinishLength,
		`{"done":true,"done_reason":"stop"}`:   provider.FinishStop,
		`{"done":true}`:                        "",
		`{"done":true,"done_reason":"load"}`:   "",
	}
	for line, want := range cases {
		chunk, err := fromOllamaChunk([]byte(line))
		if err != nil {
			t.Fatalf("fromOllamaChunk(%s): %v", line, err)
		}
		if chunk.Kind != provider.ChunkDone {
			t.Errorf("Kind = %v, want ChunkDone (line %s)", chunk.Kind, line)
		}
		if chunk.FinishReason != want {
			t.Errorf("FinishReason = %q, want %q (line %s)", chunk.FinishReason, want, line)
		}
	}
}
