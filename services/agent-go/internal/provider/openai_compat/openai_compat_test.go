package openai_compat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// newTestClient trỏ Client vào httptest server thay vì server thật.
func newTestClient(t *testing.T, apiKey string, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, apiKey, "test-model")
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

// --- Constructor ---

func TestNew_RequiresBaseURL(t *testing.T) {
	if _, err := New("", "key", "model"); err == nil {
		t.Error("New với baseURL rỗng phải lỗi")
	}
}

func TestNew_RequiresModel(t *testing.T) {
	if _, err := New("http://localhost:8000/v1", "key", ""); err == nil {
		t.Error("New với model rỗng phải lỗi")
	}
}

func TestNew_APIKeyOptional(t *testing.T) {
	c, err := New("http://localhost:8000/v1", "", "llama-3")
	if err != nil {
		t.Fatalf("New với apiKey rỗng không được lỗi (nhiều server local không cần auth): %v", err)
	}
	if c.Name() != "openai_compat" {
		t.Errorf("Name() = %q, want openai_compat", c.Name())
	}
	if c.Model() != "llama-3" {
		t.Errorf("Model() = %q, want llama-3", c.Model())
	}
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	c, err := New("http://localhost:8000/v1/", "", "m")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.baseURL != "http://localhost:8000/v1" {
		t.Errorf("baseURL = %q, want không có dấu / cuối", c.baseURL)
	}
}

// --- Dịch message/tool ---

func TestToMessages_Basic(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: "hỏi"},
		{Role: provider.RoleAssistant, Content: "đáp"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
			{ID: "c1", Name: "web.search", Args: json.RawMessage(`{"q":"go"}`)},
		}},
		{Role: provider.RoleTool, ToolCallID: "c1", Content: "kết quả"},
	}

	got := toMessages(msgs)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}

	if got[0].Role != "system" || got[1].Role != "user" || got[2].Content != "đáp" {
		t.Errorf("map role sai: %+v", got[:3])
	}

	// Assistant có tool_calls thì KHÔNG được kèm content.
	if got[3].Content != "" {
		t.Errorf("assistant kèm tool_calls có content = %q, want rỗng", got[3].Content)
	}
	// Tên tool KHÔNG bị sanitize (khác DeepSeek) — server OpenAI-compatible
	// thông thường chấp nhận dấu chấm trong tên function.
	if len(got[3].ToolCalls) != 1 || got[3].ToolCalls[0].Function.Name != "web.search" {
		t.Errorf("tool call = %+v", got[3].ToolCalls)
	}
	if got[3].ToolCalls[0].Type != "function" {
		t.Errorf("type = %q, want function", got[3].ToolCalls[0].Type)
	}

	if got[4].Role != "tool" || got[4].ToolCallID != "c1" || got[4].Content != "kết quả" {
		t.Errorf("tool result = %+v", got[4])
	}
}

func TestToMessages_SkipsUnknownRole(t *testing.T) {
	got := toMessages([]provider.Message{{Role: provider.Role("weird"), Content: "x"}})
	if len(got) != 0 {
		t.Errorf("role lạ phải bị bỏ, got %+v", got)
	}
}

func TestToTools_Basic(t *testing.T) {
	tools := []provider.ToolDef{
		{Name: "web.search", Description: "tìm", Schema: json.RawMessage(`{"type":"object"}`)},
		{Name: "echo", Description: "vọng"}, // schema rỗng
		{Name: "bad", Schema: json.RawMessage(`{invalid`)},
	}

	got := toTools(tools)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Tên tool giữ nguyên dấu chấm (không sanitize).
	if got[0].Function.Name != "web.search" || got[0].Function.Parameters["type"] != "object" {
		t.Errorf("tool 0 = %+v", got[0].Function)
	}
	if got[1].Function.Parameters == nil {
		t.Error("schema rỗng phải thành object rỗng, không nil")
	}
	if len(got[2].Function.Parameters) != 0 {
		t.Errorf("schema hỏng = %+v, want rỗng", got[2].Function.Parameters)
	}
}

func TestToTools_Empty(t *testing.T) {
	if got := toTools(nil); len(got) != 0 {
		t.Errorf("toTools(nil) = %+v, want rỗng", got)
	}
}

// --- mapFinishReason ---

func TestMapFinishReason(t *testing.T) {
	cases := map[string]provider.FinishReason{
		"length":              provider.FinishLength,
		"tool_calls":          provider.FinishToolCalls,
		"stop":                provider.FinishStop,
		"":                    "",
		"content_filter":      "",
		"unknown_from_future": "",
	}
	for raw, want := range cases {
		if got := mapFinishReason(raw); got != want {
			t.Errorf("mapFinishReason(%q) = %q, want %q", raw, got, want)
		}
	}
}

// --- Generate (qua httptest) ---

const sseOK = `data: {"choices":[{"delta":{"content":"OK"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`

func TestGenerate_StreamsTextAndUsage(t *testing.T) {
	var gotBody ocChatRequest
	c := newTestClient(t, "test-key", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"xin "}}]}
data: {"choices":[{"delta":{"content":"chào"}}]}
data: {"usage":{"prompt_tokens":7,"completion_tokens":2}}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		System:   "bạn là JARVIS",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	chunks := drain(t, stream)

	var text strings.Builder
	var usage *provider.Usage
	var done bool
	for _, ch := range chunks {
		switch ch.Kind {
		case provider.ChunkText:
			text.WriteString(ch.Text)
		case provider.ChunkUsage:
			usage = ch.Usage
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
	if usage == nil || usage.InputTokens != 7 || usage.OutputTokens != 2 {
		t.Errorf("usage = %+v, want {7 2}", usage)
	}
	if !done {
		t.Error("thiếu ChunkDone")
	}

	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" {
		t.Errorf("messages gửi lên = %+v", gotBody.Messages)
	}
	if !gotBody.Stream {
		t.Error("stream = false, want true")
	}
	if gotBody.Model != "test-model" {
		t.Errorf("model = %q, want test-model", gotBody.Model)
	}
}

// apiKey rỗng → KHÔNG gửi header Authorization gì cả (khác việc gửi "Bearer "
// rỗng) — nhiều server local (vLLM, llama.cpp server, LM Studio) không cần auth.
func TestGenerate_NoAuthHeaderWhenAPIKeyEmpty(t *testing.T) {
	var gotAuthHeader string
	var sawAuthHeader bool
	c := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader, sawAuthHeader = r.Header.Get("Authorization"), r.Header.Get("Authorization") != ""
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sseOK))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	drain(t, stream)

	if sawAuthHeader {
		t.Errorf("Authorization header = %q, want không được gửi khi apiKey rỗng", gotAuthHeader)
	}
}

func TestGenerate_IncrementalToolCalls(t *testing.T) {
	c := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"web.search"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"go\"}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "tìm go"}},
		Tools:    []provider.ToolDef{{Name: "web.search"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var calls []provider.ToolCall
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkToolCall && ch.ToolCall != nil {
			calls = append(calls, *ch.ToolCall)
		}
	}

	if len(calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(calls))
	}
	if calls[0].ID != "c1" || calls[0].Name != "web.search" {
		t.Errorf("tool call = %+v", calls[0])
	}
	if string(calls[0].Args) != `{"q":"go"}` {
		t.Errorf("args = %s, want {\"q\":\"go\"}", calls[0].Args)
	}
}

// Stream bị cắt giữa đường (không [DONE], không finish_reason) do connection
// reset — phải emit ChunkError, KHÔNG được emit ChunkDone như thành công.
func TestGenerate_StreamCutMidway_EmitsChunkError(t *testing.T) {
	c := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"xin "}}]}` + "\n"))
		w.(http.Flusher).Flush()

		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("ResponseWriter không hỗ trợ Hijack")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("Hijack: %v", err)
		}
		conn.Close()
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var gotChunkError, gotDone bool
	for _, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkError:
			gotChunkError = true
			if ch.Err == nil {
				t.Error("ChunkError.Err = nil, muốn có lỗi thật kèm theo")
			}
		case provider.ChunkDone:
			gotDone = true
		}
	}
	if !gotChunkError {
		t.Fatal("expected ChunkError khi connection bị đóng đột ngột giữa stream")
	}
	if gotDone {
		t.Error("không được emit ChunkDone cho stream bị cắt")
	}
}

// Thiếu marker [DONE] nhưng ĐÃ có finish_reason ở chunk cuối → vẫn coi là kết
// thúc hợp lệ (một số server không gửi [DONE]).
func TestGenerate_EndsCleanlyWithoutDoneMarker(t *testing.T) {
	c := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"ok"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var gotDone, gotErr bool
	for _, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkDone:
			gotDone = true
		case provider.ChunkError:
			gotErr = true
		}
	}
	if !gotDone {
		t.Error("thiếu ChunkDone dù đã có finish_reason ở chunk cuối")
	}
	if gotErr {
		t.Error("không được emit ChunkError khi stream có finish_reason hợp lệ")
	}
}

func TestGenerate_HTTPError(t *testing.T) {
	c := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"model not loaded"}`))
	})

	_, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err == nil {
		t.Fatal("Generate = nil error, want lỗi HTTP 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %q, want chứa 503", err)
	}
}

func TestGenerate_UnreachableHost(t *testing.T) {
	c, err := New("http://127.0.0.1:1", "", "m")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := c.Generate(context.Background(), provider.GenerateRequest{}); err == nil {
		t.Fatal("Generate = nil error, want lỗi kết nối")
	}
}

func TestGenerate_ContextCancelled(t *testing.T) {
	c := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Generate(ctx, provider.GenerateRequest{}); err == nil {
		t.Fatal("Generate với ctx đã huỷ = nil error, want lỗi")
	}
}

func TestGenerate_IgnoresMalformedLines(t *testing.T) {
	c := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`
: comment
data: không-phải-json
rác không có prefix
data: {"choices":[{"delta":{"content":"ok"}}]}
data: [DONE]
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var text string
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkText {
			text += ch.Text
		}
	}
	if text != "ok" {
		t.Errorf("text = %q, want ok", text)
	}
}

// --- flushToolCalls ---

func TestFlushToolCalls_EmptyArgsBecomesEmptyObject(t *testing.T) {
	var emitted []provider.StreamChunk
	emit := func(c provider.StreamChunk) bool {
		emitted = append(emitted, c)
		return true
	}

	calls := []pendingTool{{id: "1", name: "datetime"}}
	flushToolCalls(&calls, emit)

	if len(emitted) != 1 {
		t.Fatalf("emit %d chunk, want 1", len(emitted))
	}
	if got := string(emitted[0].ToolCall.Args); got != "{}" {
		t.Errorf("args = %q, want %q", got, "{}")
	}
}

func TestFlushToolCalls_SkipsIncomplete(t *testing.T) {
	var emitted []provider.StreamChunk
	emit := func(c provider.StreamChunk) bool {
		emitted = append(emitted, c)
		return true
	}

	calls := []pendingTool{{id: "1"}, {id: "2", name: "echo"}}
	calls[1].args.WriteString(`{"a":1}`)

	flushToolCalls(&calls, emit)

	if len(emitted) != 1 {
		t.Fatalf("emit %d chunk, want 1 (bỏ tool thiếu name)", len(emitted))
	}
	if emitted[0].ToolCall.Name != "echo" {
		t.Errorf("tool = %q, want echo", emitted[0].ToolCall.Name)
	}
	if calls != nil {
		t.Error("flushToolCalls phải reset slice về nil")
	}
}

// --- streamSSE trực tiếp (không qua HTTP) ---

func collectStream(t *testing.T, sse string) []provider.StreamChunk {
	t.Helper()
	c := &Client{}
	out := make(chan provider.StreamChunk)
	go c.streamSSE(context.Background(), io.NopCloser(strings.NewReader(sse)), out)

	var chunks []provider.StreamChunk
	for chunk := range out {
		chunks = append(chunks, chunk)
	}
	return chunks
}

func TestStreamSSE_TruncatedByLength(t *testing.T) {
	sse := `data: {"choices":[{"index":0,"delta":{"content":"một câu dài"}}]}
data: {"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}
data: [DONE]
`
	chunks := collectStream(t, sse)
	var done provider.StreamChunk
	for _, ch := range chunks {
		if ch.Kind == provider.ChunkDone {
			done = ch
		}
	}
	if done.FinishReason != provider.FinishLength {
		t.Errorf("done.FinishReason = %q, want %q", done.FinishReason, provider.FinishLength)
	}
}
