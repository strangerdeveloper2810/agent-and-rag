package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// fakeSuggestionsRunner trả về đúng 1 response text cố định, ghi lại prompt đã
// nhận (UserMessage) để test kiểm tra ngữ cảnh cá nhân hoá có được đưa vào
// prompt hay không.
type fakeSuggestionsRunner struct {
	responseText string
	err          error
	lastPrompt   string
}

func (f *fakeSuggestionsRunner) Run(_ context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	f.lastPrompt = in.UserMessage
	if f.err != nil {
		return provider.Usage{}, f.err
	}
	emit(agent.Event{Type: "text", Text: f.responseText})
	return provider.Usage{}, nil
}

type fakeMessagesFetcher struct {
	messages []string
	err      error
}

func (f *fakeMessagesFetcher) RecentUserMessages(_ context.Context, _ string, _ int) ([]string, error) {
	return f.messages, f.err
}

type fakeFactsProvider struct {
	facts map[string]string
}

func (f *fakeFactsProvider) All(_ string) map[string]string {
	return f.facts
}

func doSuggestionsRequest(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/suggestions", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}

// TestSuggestionsHandler_TaggedSuggestions khoá format response MỚI: mỗi gợi
// ý kèm category (creative/rag/dev/search/productivity) để FE lọc theo tab
// đang chọn mà KHÔNG cần gọi lại LLM mỗi lần đổi tab.
func TestSuggestionsHandler_TaggedSuggestions(t *testing.T) {
	runner := &fakeSuggestionsRunner{
		responseText: `[{"text":"Tóm tắt code Go này","category":"dev"},{"text":"Viết ý tưởng mới","category":"creative"}]`,
	}
	h := NewSuggestionsHandler(runner, &fakeMessagesFetcher{}, &fakeFactsProvider{})

	body := doSuggestionsRequest(t, h)
	raw, err := json.Marshal(body["suggestions"])
	if err != nil {
		t.Fatalf("marshal suggestions: %v", err)
	}

	var got []suggestionItem
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal suggestions: %v", err)
	}
	want := []suggestionItem{
		{Text: "Tóm tắt code Go này", Category: "dev"},
		{Text: "Viết ý tưởng mới", Category: "creative"},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("suggestions = %+v, want %+v", got, want)
	}
}

// TestSuggestionsHandler_LegacyFlatStringArray: nếu LLM (do biến thiên) vẫn
// trả mảng string thuần (format cũ, không category) thì vẫn phải parse được,
// chỉ là category rỗng — không được vỡ/rơi về fallback oan.
func TestSuggestionsHandler_LegacyFlatStringArray(t *testing.T) {
	runner := &fakeSuggestionsRunner{responseText: `["Câu hỏi cũ 1","Câu hỏi cũ 2"]`}
	h := NewSuggestionsHandler(runner, &fakeMessagesFetcher{}, &fakeFactsProvider{})

	body := doSuggestionsRequest(t, h)
	raw, _ := json.Marshal(body["suggestions"])
	var got []suggestionItem
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal suggestions: %v", err)
	}
	if len(got) != 2 || got[0].Text != "Câu hỏi cũ 1" || got[0].Category != "" {
		t.Errorf("suggestions = %+v, want 2 items, category rỗng", got)
	}
}

// TestSuggestionsHandler_FallbackOnRunnerError: runner lỗi → fallback tĩnh,
// không panic, không trả lỗi HTTP.
func TestSuggestionsHandler_FallbackOnRunnerError(t *testing.T) {
	runner := &fakeSuggestionsRunner{err: errors.New("llm down")}
	h := NewSuggestionsHandler(runner, &fakeMessagesFetcher{}, &fakeFactsProvider{})

	body := doSuggestionsRequest(t, h)
	raw, _ := json.Marshal(body["suggestions"])
	var got []suggestionItem
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal suggestions: %v", err)
	}
	if len(got) == 0 {
		t.Error("fallback suggestions rỗng — phải có ít nhất vài câu tĩnh")
	}
}

// TestSuggestionsHandler_PromptIncludesPersonalContext: prompt gửi cho LLM
// phải kèm lịch sử hội thoại gần đây + facts đã học + mốc thời gian thật —
// đây chính là phần "dynamic" mà trước đây bị thiếu (prompt cũ hoàn toàn
// tĩnh, không đổi giữa các user/thời điểm).
func TestSuggestionsHandler_PromptIncludesPersonalContext(t *testing.T) {
	runner := &fakeSuggestionsRunner{responseText: `[]`}
	h := NewSuggestionsHandler(
		runner,
		&fakeMessagesFetcher{messages: []string{"làm sao debug goroutine leak"}},
		&fakeFactsProvider{facts: map[string]string{"web_framework": "Fastify"}},
	)

	doSuggestionsRequest(t, h)

	if !strings.Contains(runner.lastPrompt, "debug goroutine leak") {
		t.Errorf("prompt thiếu lịch sử hội thoại gần đây: %q", runner.lastPrompt)
	}
	if !strings.Contains(runner.lastPrompt, "Fastify") {
		t.Errorf("prompt thiếu fact đã học: %q", runner.lastPrompt)
	}
	if !strings.Contains(runner.lastPrompt, "Hôm nay là") {
		t.Errorf("prompt thiếu mốc thời gian thật: %q", runner.lastPrompt)
	}
}

// TestSuggestionsHandler_GracefulWithoutHistoryOrFacts: user mới, chưa có
// lịch sử/facts nào (hoặc mongoClient/store nil khi wiring) — không được
// panic hay lỗi, chỉ đơn giản là prompt không có 2 mục đó.
func TestSuggestionsHandler_GracefulWithoutHistoryOrFacts(t *testing.T) {
	runner := &fakeSuggestionsRunner{responseText: `[]`}
	h := NewSuggestionsHandler(runner, nil, nil)

	body := doSuggestionsRequest(t, h)
	if body["suggestions"] == nil {
		t.Error("response thiếu field suggestions")
	}
}

// TestParseSuggestions_ExtractsArrayFromProse: model đôi khi kèm giải thích
// quanh mảng JSON dù đã được yêu cầu không làm vậy — vẫn phải trích được.
func TestParseSuggestions_ExtractsArrayFromProse(t *testing.T) {
	got := parseSuggestions("Đây là gợi ý:\n[{\"text\":\"a\",\"category\":\"dev\"}]\nHy vọng hữu ích.")
	if len(got) != 1 || got[0].Text != "a" || got[0].Category != "dev" {
		t.Errorf("parseSuggestions = %+v, want 1 item {a, dev}", got)
	}
}

// TestParseSuggestions_GarbageReturnsNil: không tìm được mảng JSON hợp lệ nào
// → nil, để ServeHTTP rơi về fallback tĩnh (không panic, không mảng rỗng giả).
func TestParseSuggestions_GarbageReturnsNil(t *testing.T) {
	if got := parseSuggestions("tôi không biết"); got != nil {
		t.Errorf("parseSuggestions = %v, want nil", got)
	}
	if got := parseSuggestions("[không phải json]"); got != nil {
		t.Errorf("parseSuggestions = %v, want nil", got)
	}
}

// TestFallbackSuggestions_NeverEmpty: fallback tĩnh phải luôn có nội dung hợp
// lệ — đây là lớp chốt cuối khi cả LLM lẫn parse đều thất bại.
func TestFallbackSuggestions_NeverEmpty(t *testing.T) {
	got := fallbackSuggestions()
	if len(got) == 0 {
		t.Fatal("fallbackSuggestions() rỗng")
	}
	for i, s := range got {
		if s.Text == "" {
			t.Errorf("gợi ý %d rỗng", i)
		}
	}
}
