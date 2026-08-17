package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// newTestClient trỏ Client vào httptest server thay vì API thật.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := New("test-key", "flash", "pro")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.baseURL = srv.URL
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

func TestNew(t *testing.T) {
	if _, err := New("", "f", "p"); err == nil {
		t.Error("New với apiKey rỗng phải lỗi")
	}

	c, err := New("k", "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.flashModel != "deepseek-v4-flash" || c.proModel != "deepseek-v4-pro" {
		t.Errorf("model mặc định = %q/%q", c.flashModel, c.proModel)
	}
	if c.Name() != "deepseek" {
		t.Errorf("Name() = %q, want deepseek", c.Name())
	}
}

// --- pickModel ---

func TestPickModel(t *testing.T) {
	c := &Client{flashModel: "flash", proModel: "pro"}

	longMsg := provider.Message{Role: provider.RoleUser, Content: strings.Repeat("x", 1001)}
	manyMsgs := make([]provider.Message, 11)
	for i := range manyMsgs {
		manyMsgs[i] = provider.Message{Role: provider.RoleUser, Content: "hi"}
	}

	cases := []struct {
		name string
		req  provider.GenerateRequest
		want string
	}{
		{
			name: "mặc định → flash",
			req:  provider.GenerateRequest{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}},
			want: "flash",
		},
		{
			name: "Options.Model thắng tất cả",
			req:  provider.GenerateRequest{Options: provider.ProviderOptions{Model: "custom"}},
			want: "custom",
		},
		{
			name: "thinking bật → pro",
			req:  provider.GenerateRequest{Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingHigh}},
			want: "pro",
		},
		{
			name: "thinking OFF → flash",
			req:  provider.GenerateRequest{Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingOff}},
			want: "flash",
		},
		{
			name: "hội thoại dài (>10 message) → pro",
			req:  provider.GenerateRequest{Messages: manyMsgs},
			want: "pro",
		},
		{
			name: "user message dài (>1000 ký tự) → pro",
			req:  provider.GenerateRequest{Messages: []provider.Message{longMsg}},
			want: "pro",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.pickModel(tc.req); got != tc.want {
				t.Errorf("pickModel = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- Dịch message/tool ---

func TestSanitizeName(t *testing.T) {
	if got := sanitizeName("web.search"); got != "web_search" {
		t.Errorf("sanitizeName = %q, want web_search", got)
	}
	if got := unsanitizeName("web_search"); got != "web.search" {
		t.Errorf("unsanitizeName = %q, want web.search", got)
	}
}

func TestToDSMessages(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: "hỏi"},
		{Role: provider.RoleAssistant, Content: "đáp"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
			{ID: "c1", Name: "web.search", Args: json.RawMessage(`{"q":"go"}`)},
		}},
		{Role: provider.RoleTool, ToolCallID: "c1", Content: "kết quả"},
	}

	got := toDSMessages(msgs)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}

	if got[0].Role != "system" || got[1].Role != "user" || got[2].Content != "đáp" {
		t.Errorf("map role sai: %+v", got[:3])
	}

	// Assistant có tool_calls thì KHÔNG được kèm content (yêu cầu của DeepSeek),
	// và tên tool phải sanitize dấu chấm.
	if got[3].Content != "" {
		t.Errorf("assistant kèm tool_calls có content = %q, want rỗng", got[3].Content)
	}
	if len(got[3].ToolCalls) != 1 || got[3].ToolCalls[0].Function.Name != "web_search" {
		t.Errorf("tool call = %+v", got[3].ToolCalls)
	}
	if got[3].ToolCalls[0].Type != "function" {
		t.Errorf("type = %q, want function", got[3].ToolCalls[0].Type)
	}

	if got[4].Role != "tool" || got[4].ToolCallID != "c1" || got[4].Content != "kết quả" {
		t.Errorf("tool result = %+v", got[4])
	}
}

func TestToDSMessages_SkipsUnknownRole(t *testing.T) {
	got := toDSMessages([]provider.Message{{Role: provider.Role("weird"), Content: "x"}})
	if len(got) != 0 {
		t.Errorf("role lạ phải bị bỏ, got %+v", got)
	}
}

func TestToDSTools(t *testing.T) {
	tools := []provider.ToolDef{
		{Name: "web.search", Description: "tìm", Schema: json.RawMessage(`{"type":"object"}`)},
		{Name: "echo", Description: "vọng"}, // schema rỗng
		{Name: "bad", Schema: json.RawMessage(`{invalid`)},
	}

	got := toDSTools(tools)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Function.Name != "web_search" || got[0].Function.Parameters["type"] != "object" {
		t.Errorf("tool 0 = %+v", got[0].Function)
	}
	if got[1].Function.Parameters == nil {
		t.Error("schema rỗng phải thành object rỗng, không nil")
	}
	// Schema hỏng: không panic, parameters rỗng.
	if len(got[2].Function.Parameters) != 0 {
		t.Errorf("schema hỏng = %+v, want rỗng", got[2].Function.Parameters)
	}
}

func TestToDSTools_Empty(t *testing.T) {
	if got := toDSTools(nil); len(got) != 0 {
		t.Errorf("toDSTools(nil) = %+v, want rỗng", got)
	}
}

// --- Generate (qua httptest) ---

// sseOK là 1 stream SSE hợp lệ tối thiểu, dùng cho các test chỉ quan tâm
// REQUEST BODY gửi lên chứ không quan tâm response.
const sseOK = `data: {"choices":[{"delta":{"content":"OK"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`

// TestGenerate_ThinkingOffDisablesReasoning khoá phát hiện đã VERIFY BẰNG API
// THẬT: với deepseek-v4-flash, thinking BẬT MẶC ĐỊNH và token suy luận TÍNH
// VÀO max_tokens. Cùng prompt max_tokens=16: không gửi field thinking → toàn
// bộ 16 token vào reasoning_content, content RỖNG, finish_reason="length";
// gửi {"type":"disabled"} → content="OK" tốn 1 token.
//
// Trước fix, ThinkingLevel chỉ được dùng để LOG chứ không gửi lên API, nên
// provider.ThinkingOff là no-op — đó là lý do HyDE và LLM rerank (MaxTokens=200)
// âm thầm trả rỗng dù code đã chủ động set ThinkingOff.
func TestGenerate_ThinkingOffDisablesReasoning(t *testing.T) {
	var gotBody dsChatRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sseOK))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		Options:  provider.ProviderOptions{ThinkingLevel: provider.ThinkingOff, MaxTokens: 200},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	drain(t, stream)

	if gotBody.Thinking == nil {
		t.Fatal("request thiếu field thinking — ThinkingOff phải gửi thinking:{\"type\":\"disabled\"} lên API")
	}
	if gotBody.Thinking.Type != "disabled" {
		t.Errorf("thinking.type = %q, want %q", gotBody.Thinking.Type, "disabled")
	}
	if gotBody.ReasoningEffort != "" {
		t.Errorf("reasoning_effort = %q, want rỗng khi đã disable thinking", gotBody.ReasoningEffort)
	}
}

// Với ThinkingLevel khác OFF thì KHÔNG disable, mà gửi reasoning_effort.
// Lưu ý đã verify bằng API thật: reasoning_effort="low" KHÔNG tắt suy luận.
func TestGenerate_ThinkingLevelMapsToReasoningEffort(t *testing.T) {
	tests := []struct {
		level      provider.ThinkingLevel
		wantEffort string
	}{
		{provider.ThinkingLow, "low"},
		{provider.ThinkingMedium, "high"},
		{provider.ThinkingHigh, "high"},
		{"", ""}, // không set → để model dùng mặc định
	}

	for _, tc := range tests {
		t.Run(string(tc.level), func(t *testing.T) {
			var gotBody dsChatRequest
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte(sseOK))
			})

			stream, err := c.Generate(context.Background(), provider.GenerateRequest{
				Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
				Options:  provider.ProviderOptions{ThinkingLevel: tc.level},
			})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			drain(t, stream)

			if gotBody.ReasoningEffort != tc.wantEffort {
				t.Errorf("reasoning_effort = %q, want %q", gotBody.ReasoningEffort, tc.wantEffort)
			}
			if gotBody.Thinking != nil {
				t.Errorf("thinking = %+v, want nil khi ThinkingLevel != OFF", gotBody.Thinking)
			}
		})
	}
}

// API tương thích OpenAI không gửi usage khi stream nếu thiếu
// stream_options.include_usage → ChunkUsage sẽ luôn thiếu số thật.
func TestGenerate_AlwaysRequestsStreamUsage(t *testing.T) {
	var gotBody dsChatRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
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

	if gotBody.StreamOptions == nil || !gotBody.StreamOptions.IncludeUsage {
		t.Errorf("stream_options = %+v, want include_usage=true", gotBody.StreamOptions)
	}
}

// reasoning_content là chuỗi suy luận, KHÔNG phải câu trả lời — không được
// lẫn vào ChunkText (nếu lẫn, user sẽ thấy phần model "tự nhủ" trong câu trả lời).
func TestGenerate_ReasoningContentNotEmittedAsText(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"reasoning_content":"Ta cần trả lời chính xác OK. Người dùng nói..."}}]}
data: {"choices":[{"delta":{"content":"OK"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var text strings.Builder
	for _, ch := range drain(t, stream) {
		if ch.Kind == provider.ChunkText {
			text.WriteString(ch.Text)
		}
	}
	if text.String() != "OK" {
		t.Errorf("text = %q, want %q — reasoning_content không được emit ra ChunkText", text.String(), "OK")
	}
}

// TestGenerate_StreamCutMidway_EmitsChunkError bịt lỗ mà fix scanner.Err()
// trước đó KHÔNG bắt được: bufio.Scanner.Err() chỉ trả lỗi NON-EOF, nên khi
// server/proxy đóng stream "sạch" giữa chừng thì Err() là nil và code cũ emit
// ChunkDone như thể thành công. Phân biệt bằng finish_reason: stream hoàn tất
// tử tế luôn có finish_reason ở chunk cuối.
func TestGenerate_StreamCutMidway_EmitsChunkError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Có text nhưng KHÔNG có finish_reason và KHÔNG có [DONE] — handler
		// return làm body đóng sạch (EOF), scanner.Err() sẽ là nil.
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"{\"user_facts\": [], \"knowledge_items\": [{\"tags\": [\"go\","}}]}
`))
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var gotErr, gotDone bool
	for _, ch := range drain(t, stream) {
		switch ch.Kind {
		case provider.ChunkError:
			gotErr = true
		case provider.ChunkDone:
			gotDone = true
		}
	}
	if !gotErr {
		t.Error("want ChunkError khi stream bị cắt giữa đường (không [DONE], không finish_reason)")
	}
	if gotDone {
		t.Error("không được emit ChunkDone cho stream bị cắt — caller sẽ tưởng là thành công")
	}
}

func TestGenerate_StreamsTextAndUsage(t *testing.T) {
	var gotBody dsChatRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
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

	// System prompt phải được chèn thành message đầu tiên.
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" {
		t.Errorf("messages gửi lên = %+v", gotBody.Messages)
	}
	if !gotBody.Stream {
		t.Error("stream = false, want true")
	}
	if gotBody.Model != "flash" {
		t.Errorf("model = %q, want flash", gotBody.Model)
	}
}

// TestGenerate_StreamReadError_EmitsChunkError tái hiện đúng bug thực tế từ
// log dev: connection bị đóng đột ngột GIỮA stream (reset, timeout mạng...)
// khiến scanner.Scan() trả false vì LỖI ĐỌC THẬT, không phải hết stream bình
// thường. Trước fix, code không check scanner.Err() nên vẫn emit ChunkDone
// như thành công, khiến caller nhận response "rỗng nhưng thành công" — hiện
// ra ở tầng gọi (rerankLLM, HyDE, memory.ReflectAndExtract) dưới dạng lỗi mơ
// hồ "unexpected end of JSON input" thay vì lỗi provider/mạng thật.
func TestGenerate_StreamReadError_EmitsChunkError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"xin "}}]}` + "\n"))
		w.(http.Flusher).Flush()

		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("ResponseWriter không hỗ trợ Hijack — không mô phỏng được connection reset")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("Hijack: %v", err)
		}
		conn.Close() // đóng đột ngột GIỮA stream, mô phỏng connection reset thật
	})

	stream, err := c.Generate(context.Background(), provider.GenerateRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	chunks := drain(t, stream)

	var gotChunkError bool
	for _, ch := range chunks {
		if ch.Kind == provider.ChunkError {
			gotChunkError = true
			if ch.Err == nil {
				t.Error("ChunkError.Err = nil, muốn có lỗi thật kèm theo")
			}
		}
	}
	if !gotChunkError {
		t.Fatalf("expected 1 ChunkError khi connection bị đóng đột ngột giữa stream, got %+v", chunks)
	}
}

func TestGenerate_IncrementalToolCalls(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"web_search"}}]}}]}
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
	// Args ghép từ nhiều delta; tên tool phải được un-sanitize về dấu chấm.
	if calls[0].ID != "c1" || calls[0].Name != "web.search" {
		t.Errorf("tool call = %+v", calls[0])
	}
	if string(calls[0].Args) != `{"q":"go"}` {
		t.Errorf("args = %s, want {\"q\":\"go\"}", calls[0].Args)
	}
}

func TestGenerate_HTTPError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	})

	_, err := c.Generate(context.Background(), provider.GenerateRequest{})
	if err == nil {
		t.Fatal("Generate = nil error, want lỗi HTTP 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("err = %q, want chứa 429", err)
	}
}

func TestGenerate_UnreachableHost(t *testing.T) {
	c, err := New("k", "flash", "pro")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.baseURL = "http://127.0.0.1:1" // cổng không ai nghe

	if _, err := c.Generate(context.Background(), provider.GenerateRequest{}); err == nil {
		t.Fatal("Generate = nil error, want lỗi kết nối")
	}
}

func TestGenerate_ContextCancelled(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Generate(ctx, provider.GenerateRequest{}); err == nil {
		t.Fatal("Generate với ctx đã huỷ = nil error, want lỗi")
	}
}

// SSE có dòng rác/không phải JSON → bỏ qua, không làm hỏng stream.
func TestGenerate_IgnoresMalformedLines(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
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

// Tool call thiếu tên hoặc thiếu args thì không được phát ra.
// TestFlushToolCalls_EmptyArgsBecomesEmptyObject: tool call CÓ tên nhưng
// arguments rỗng phải được emit với "{}" chứ không bị bỏ. API tương thích
// OpenAI gửi arguments: "" cho tool mà mọi tham số đều optional — code cũ đòi
// args.Len() > 0 nên bỏ ÂM THẦM: tool không chạy, và nếu lượt đó cũng không có
// text thì caller báo lỗi mơ hồ "empty response". Adapter Anthropic đã xử lý
// đúng case này từ trước.
func TestFlushToolCalls_EmptyArgsBecomesEmptyObject(t *testing.T) {
	var emitted []provider.StreamChunk
	emit := func(c provider.StreamChunk) bool {
		emitted = append(emitted, c)
		return true
	}

	calls := []pendingTool{{id: "1", name: "datetime"}} // name có, args rỗng
	flushToolCalls(&calls, emit)

	if len(emitted) != 1 {
		t.Fatalf("emit %d chunk, want 1 — tool call không có args vẫn phải chạy", len(emitted))
	}
	if got := string(emitted[0].ToolCall.Args); got != "{}" {
		t.Errorf("args = %q, want %q", got, "{}")
	}
	if emitted[0].ToolCall.Name != "datetime" {
		t.Errorf("tool = %q, want datetime", emitted[0].ToolCall.Name)
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
		t.Fatalf("emit %d chunk, want 1 (bỏ tool thiếu name/args)", len(emitted))
	}
	if emitted[0].ToolCall.Name != "echo" {
		t.Errorf("tool = %q, want echo", emitted[0].ToolCall.Name)
	}
	if calls != nil {
		t.Error("flushToolCalls phải reset slice về nil")
	}
}

func TestFlushToolCalls_StopsWhenEmitFails(t *testing.T) {
	n := 0
	emit := func(provider.StreamChunk) bool {
		n++
		return false // ctx huỷ giữa chừng
	}

	calls := []pendingTool{{id: "1", name: "a"}, {id: "2", name: "b"}}
	calls[0].args.WriteString("{}")
	calls[1].args.WriteString("{}")

	flushToolCalls(&calls, emit)

	if n != 1 {
		t.Errorf("emit gọi %d lần, want 1 (dừng ngay khi emit trả false)", n)
	}
}
