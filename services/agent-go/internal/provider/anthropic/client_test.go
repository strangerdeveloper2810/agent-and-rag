package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// newTestClient dựng Client trỏ SDK vào httptest server (New() hard-code base URL
// thật nên test tự lắp sdk.Client).
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &Client{
		sdk: sdk.NewClient(
			option.WithAPIKey("test-key"),
			option.WithBaseURL(srv.URL),
			option.WithMaxRetries(0),
		),
		model: "claude-haiku-4-5-20251001",
	}
}

// sseWriter ghi các event SSE theo định dạng Anthropic Messages streaming.
func writeSSE(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		_, _ = w.Write([]byte(e))
	}
}

const (
	evMessageStart = `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":11,"output_tokens":1}}}

`
	evTextBlockStart = `event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`
	evTextDelta = `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"xin chào"}}

`
	evBlockStop = `event: content_block_stop
data: {"type":"content_block_stop","index":0}

`
	evMessageStop = `event: message_stop
data: {"type":"message_stop"}

`
)

func messageDelta(stopReason string, outputTokens int) string {
	return `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"` + stopReason + `","stop_sequence":null},"usage":{"output_tokens":` +
		itoa(outputTokens) + `}}

`
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
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

func TestNew(t *testing.T) {
	if _, err := New("", "m"); err == nil {
		t.Error("New với apiKey rỗng phải lỗi")
	}
	if _, err := New("k", ""); err == nil {
		t.Error("New với model rỗng phải lỗi")
	}

	c, err := New("k", "m")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Name() != "anthropic" {
		t.Errorf("Name() = %q, want anthropic", c.Name())
	}
	if c.model != "m" {
		t.Errorf("model = %q, want m", c.model)
	}
}

func TestUnsanitizeToolName(t *testing.T) {
	if got := unsanitizeToolName("web_search"); got != "web.search" {
		t.Errorf("unsanitizeToolName = %q, want web.search", got)
	}
	if got := sanitizeToolName("web.search"); got != "web_search" {
		t.Errorf("sanitizeToolName = %q, want web_search", got)
	}
}

// --- buildParams ---

func TestBuildParams_Defaults(t *testing.T) {
	c := &Client{model: "default-model"}

	params := c.buildParams(provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})

	if string(params.Model) != "default-model" {
		t.Errorf("Model = %q, want default-model", params.Model)
	}
	if params.MaxTokens != defaultMaxTokens {
		t.Errorf("MaxTokens = %d, want %d", params.MaxTokens, defaultMaxTokens)
	}
	if len(params.Messages) != 1 {
		t.Errorf("Messages len = %d, want 1", len(params.Messages))
	}
	if len(params.System) != 0 {
		t.Errorf("System = %+v, want rỗng", params.System)
	}
	if len(params.Tools) != 0 {
		t.Errorf("Tools = %+v, want rỗng", params.Tools)
	}
}

func TestBuildParams_Overrides(t *testing.T) {
	c := &Client{model: "default-model"}

	params := c.buildParams(provider.GenerateRequest{
		System:   "bạn là JARVIS",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		Tools:    []provider.ToolDef{{Name: "web.search", Schema: json.RawMessage(`{"properties":{"q":{"type":"string"}},"required":["q"]}`)}},
		Options: provider.ProviderOptions{
			Model:     "claude-opus-5",
			MaxTokens: 100,
			Cache:     true,
		},
	})

	if string(params.Model) != "claude-opus-5" {
		t.Errorf("Model = %q, want claude-opus-5", params.Model)
	}
	if params.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", params.MaxTokens)
	}
	if len(params.System) != 1 || params.System[0].Text != "bạn là JARVIS" {
		t.Errorf("System = %+v", params.System)
	}
	// Cache=true phải đánh dấu cache_control trên khối system.
	if params.System[0].CacheControl.Type == "" {
		t.Error("Cache=true nhưng system không có cache_control")
	}
	if len(params.Tools) != 1 {
		t.Errorf("Tools len = %d, want 1", len(params.Tools))
	}
}

func TestBuildParams_NoCacheControl(t *testing.T) {
	c := &Client{model: "m"}
	params := c.buildParams(provider.GenerateRequest{
		System:  "sys",
		Options: provider.ProviderOptions{Cache: false},
	})

	if params.System[0].CacheControl.Type != "" {
		t.Error("Cache=false nhưng system vẫn có cache_control")
	}
}

// --- Generate ---

func TestGenerate_TextStream(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("X-Api-Key = %q", r.Header.Get("X-Api-Key"))
		}
		writeSSE(w,
			evMessageStart, evTextBlockStart, evTextDelta, evBlockStop,
			messageDelta("end_turn", 5), evMessageStop,
		)
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var text strings.Builder
	var usage *provider.Usage
	var done *provider.StreamChunk
	for i, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkText:
			text.WriteString(ch.Text)
		case provider.ChunkUsage:
			usage = ch.Usage
		case provider.ChunkDone:
			c := ch
			done = &c
		case provider.ChunkError:
			t.Fatalf("chunk %d lỗi: %v", i, ch.Err)
		}
	}

	if text.String() != "xin chào" {
		t.Errorf("text = %q, want %q", text.String(), "xin chào")
	}
	if usage == nil || usage.InputTokens != 11 {
		t.Errorf("usage = %+v, want InputTokens 11", usage)
	}
	if done == nil {
		t.Fatal("thiếu ChunkDone")
	}
	if done.FinishReason != provider.FinishStop {
		t.Errorf("FinishReason = %q, want stop", done.FinishReason)
	}
}

// stop_reason "max_tokens" phải map thành FinishLength (câu trả lời bị cắt).
func TestGenerate_TruncatedByMaxTokens(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeSSE(w,
			evMessageStart, evTextBlockStart, evTextDelta, evBlockStop,
			messageDelta("max_tokens", 4096), evMessageStop,
		)
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var done *provider.StreamChunk
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkDone {
			c := ch
			done = &c
		}
	}

	if done == nil {
		t.Fatal("thiếu ChunkDone")
	}
	if done.FinishReason != provider.FinishLength {
		t.Errorf("FinishReason = %q, want %q", done.FinishReason, provider.FinishLength)
	}
}

func TestGenerate_ToolUseBlock(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeSSE(w,
			evMessageStart,
			`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"web_search","input":{}}}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"q\":\"go\"}"}}

`,
			evBlockStop,
			messageDelta("tool_use", 12), evMessageStop,
		)
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "tìm go"}},
		Tools:    []provider.ToolDef{{Name: "web.search"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var call *provider.ToolCall
	var done *provider.StreamChunk
	for _, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkToolCall:
			call = ch.ToolCall
		case provider.ChunkDone:
			c := ch
			done = &c
		case provider.ChunkError:
			t.Fatalf("chunk lỗi: %v", ch.Err)
		}
	}

	if call == nil {
		t.Fatal("không nhận được tool call")
	}
	if call.ID != "toolu_1" {
		t.Errorf("ID = %q, want toolu_1", call.ID)
	}
	// Tên tool phải được un-sanitize về dấu chấm.
	if call.Name != "web.search" {
		t.Errorf("Name = %q, want web.search", call.Name)
	}
	if !strings.Contains(string(call.Args), `"go"`) {
		t.Errorf("Args = %s, want chứa \"go\"", call.Args)
	}
	if done == nil || done.FinishReason != provider.FinishToolCalls {
		t.Errorf("done = %+v, want FinishReason tool_calls", done)
	}
}

func TestGenerate_HTTPErrorEmitsErrorChunk(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var sawError bool
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkError {
			sawError = true
			if ch.Err == nil {
				t.Error("ChunkError có Err = nil")
			}
		}
	}
	if !sawError {
		t.Error("lỗi HTTP phải phát ChunkError")
	}
}

func TestGenerate_ContextCancelled(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeSSE(w, evMessageStart, evTextBlockStart, evTextDelta, evBlockStop,
			messageDelta("end_turn", 1), evMessageStop)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream, err := c.Generate(ctx, provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Channel phải đóng (không treo) khi ctx đã huỷ.
	drain(t, stream)
}

func TestSend_ReturnsFalseOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := make(chan provider.StreamChunk) // không buffer, không ai đọc
	if send(ctx, out, provider.StreamChunk{Kind: provider.ChunkDone}) {
		t.Error("send với ctx đã huỷ = true, want false")
	}
}

func TestSend_DeliversChunk(t *testing.T) {
	out := make(chan provider.StreamChunk, 1)
	if !send(context.Background(), out, provider.StreamChunk{Kind: provider.ChunkText, Text: "x"}) {
		t.Fatal("send = false, want true")
	}
	if got := <-out; got.Text != "x" {
		t.Errorf("nhận %+v, want text x", got)
	}
}

func TestToolInput(t *testing.T) {
	if got := toolInput(nil); string(got.(json.RawMessage)) != "{}" {
		t.Errorf("toolInput(nil) = %v, want {}", got)
	}
	args := json.RawMessage(`{"a":1}`)
	if got := toolInput(args); string(got.(json.RawMessage)) != `{"a":1}` {
		t.Errorf("toolInput = %v", got)
	}
}
