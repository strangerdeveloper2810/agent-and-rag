package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

type mockReflectionProvider struct {
	response string
}

func (m *mockReflectionProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: m.response}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (m *mockReflectionProvider) Name() string { return "mock" }

// chunkErrorThenTextProvider trả ChunkError ở(các) lần gọi ĐẦU, rồi trả text
// hợp lệ từ 1 mốc nào đó — mô phỏng đúng case thấy trong log dev thật: lỗi
// provider thoáng qua GIỮA stream (không phải lỗi ngay khi gọi Generate()),
// khiến channel không có ChunkText nào, fullResp rỗng.
type chunkErrorThenTextProvider struct {
	calls         int
	errorForCalls int // trả ChunkError cho `errorForCalls` lần gọi đầu tiên
	err           error
	successResp   string
}

func (m *chunkErrorThenTextProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	m.calls++
	ch := make(chan provider.StreamChunk, 2)
	if m.calls <= m.errorForCalls {
		ch <- provider.StreamChunk{Kind: provider.ChunkError, Err: m.err}
	} else {
		ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: m.successResp}
	}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (m *chunkErrorThenTextProvider) Name() string { return "mock-chunkerror" }

// sequenceReflectionProvider trả về response KHÁC NHAU cho mỗi lần Generate()
// được gọi liên tiếp — dùng để test hành vi retry của ReflectAndExtract khi
// lần gọi đầu ra JSON sai cú pháp nhưng lần sau ra JSON hợp lệ.
type sequenceReflectionProvider struct {
	responses []string
	calls     int
}

func (m *sequenceReflectionProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	idx := m.calls
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	resp := m.responses[idx]
	m.calls++

	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: resp}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (m *sequenceReflectionProvider) Name() string { return "mock-sequence" }

// truncatingProvider mô phỏng model chạm trần token: trả JSON dở dang kèm
// FinishReason=FinishLength (tín hiệu CHẮC CHẮN bị cắt), và ghi lại MaxTokens
// của từng lần gọi để kiểm tra retry có nâng ngân sách hay không.
type truncatingProvider struct {
	calls            int
	truncateForCalls int // trả bản bị cắt cho `truncateForCalls` lần đầu
	truncatedResp    string
	successResp      string
	seenMaxTokens    []int
	seenThinking     []provider.ThinkingLevel
}

func (m *truncatingProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	m.calls++
	m.seenMaxTokens = append(m.seenMaxTokens, req.Options.MaxTokens)
	m.seenThinking = append(m.seenThinking, req.Options.ThinkingLevel)

	ch := make(chan provider.StreamChunk, 3)
	if m.calls <= m.truncateForCalls {
		ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: m.truncatedResp}
		ch <- provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishLength}
	} else {
		ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: m.successResp}
		ch <- provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishStop}
	}
	close(ch)
	return ch, nil
}

func (m *truncatingProvider) Name() string { return "mock-truncating" }

// TestReflectAndExtract_TruncatedByTokenLimit_RetriesWithDoubleBudget: khi
// FinishReason=FinishLength, biết CHẮC là chạm trần token nên retry cùng ngân
// sách là vô nghĩa (lần sau cắt y vậy). Trước fix, ChunkDone bị bỏ hoàn toàn
// nên mọi nguyên nhân đều hiện ra dưới cùng triệu chứng "JSON hỏng" và retry
// mù với đúng MaxTokens cũ.
func TestReflectAndExtract_TruncatedByTokenLimit_RetriesWithDoubleBudget(t *testing.T) {
	mockP := &truncatingProvider{
		truncateForCalls: 1,
		truncatedResp:    `{"user_facts": [], "knowledge_items": [{"title": "A", "summary": "B", "tags": ["go", "conc`,
		successResp:      `{"user_facts": [], "knowledge_items": [{"title": "A", "summary": "B", "tags": ["go"], "content": "xong"}]}`,
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", []provider.Message{
		{Role: provider.RoleUser, Content: "hỏi"},
		{Role: provider.RoleAssistant, Content: "đáp"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockP.calls != 2 {
		t.Fatalf("số lần gọi LLM = %d, want 2 (bị cắt lần 1, thành công lần 2)", mockP.calls)
	}
	if len(mockP.seenMaxTokens) != 2 {
		t.Fatalf("không ghi được MaxTokens: %v", mockP.seenMaxTokens)
	}
	if mockP.seenMaxTokens[1] != mockP.seenMaxTokens[0]*2 {
		t.Errorf("MaxTokens lần 2 = %d, want gấp đôi lần 1 (%d)", mockP.seenMaxTokens[1], mockP.seenMaxTokens[0])
	}
	if len(res.KnowledgeItems) != 1 || res.KnowledgeItems[0].Content != "xong" {
		t.Errorf("kết quả = %+v, want lấy được item từ lần gọi thứ 2", res.KnowledgeItems)
	}
}

// Reflection phải yêu cầu ThinkingOff: token suy luận tính vào max_tokens (đã
// verify bằng API thật với deepseek-v4-flash), nên bật thinking cho task trích
// xuất theo schema chỉ làm output bị cắt.
func TestReflectAndExtract_RequestsThinkingOff(t *testing.T) {
	mockP := &truncatingProvider{
		successResp: `{"user_facts": [], "knowledge_items": []}`,
	}

	if _, err := ReflectAndExtract(context.Background(), mockP, "mock-model", []provider.Message{
		{Role: provider.RoleUser, Content: "hỏi"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockP.seenThinking) == 0 || mockP.seenThinking[0] != provider.ThinkingOff {
		t.Errorf("ThinkingLevel gửi đi = %v, want OFF", mockP.seenThinking)
	}
}

// timeoutProvider mô phỏng đường "cắt cụt im lặng" nguy hiểm nhất: ctx hết hạn
// giữa lúc đọc stream → provider thoát vòng emit nên channel đóng mà KHÔNG có
// ChunkError lẫn ChunkDone. Trước fix, reflection nhầm thành "JSON hỏng" rồi
// retry, trong khi ngân sách thời gian đã cạn nên retry chắc chắn cũng chết.
type timeoutProvider struct {
	calls int
}

func (m *timeoutProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	m.calls++
	ch := make(chan provider.StreamChunk, 1)
	// Trả 1 phần text rồi đóng channel không có Done/Error, đồng thời chờ ctx
	// hết hạn để ctx.Err() != nil ở phía caller.
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: `{"user_facts": [{"cat`}
	close(ch)
	<-ctx.Done()
	return ch, nil
}

func (m *timeoutProvider) Name() string { return "mock-timeout" }

func TestReflectAndExtract_Timeout_DoesNotRetry(t *testing.T) {
	mockP := &timeoutProvider{}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res, err := ReflectAndExtract(ctx, mockP, "mock-model", []provider.Message{
		{Role: provider.RoleUser, Content: "hỏi"},
	})
	if err != nil {
		t.Fatalf("hết thời gian phải bỏ qua êm, không trả lỗi: %v", err)
	}
	if res == nil || len(res.UserFacts) != 0 {
		t.Errorf("kết quả = %+v, want rỗng", res)
	}
	if mockP.calls != 1 {
		t.Errorf("số lần gọi = %d, want 1 (KHÔNG retry khi đã hết thời gian)", mockP.calls)
	}
}

func TestReflectAndExtract_Success(t *testing.T) {
	jsonResp := `{
		"user_facts": [
			{"category": "tech_stack", "key": "backend_framework", "value": "Go + Fastify", "confidence": 0.95},
			{"category": "coding_preference", "key": "css_style", "value": "Vanilla CSS", "confidence": 0.9}
		],
		"knowledge_items": [
			{
				"title": "Fix Google Search Scraping with Tavily",
				"summary": "Use Tavily AI search as primary provider and Bing as fallback.",
				"tags": ["web-search", "tavily", "scraping"],
				"content": "Google search blocks direct HTTP requests. Integration with Tavily API resolves bot challenges."
			}
		]
	}`

	mockP := &mockReflectionProvider{response: jsonResp}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Từ nay frontend nhớ dùng Vanilla CSS nhé, backend thì chạy Go + Fastify."},
		{Role: provider.RoleAssistant, Content: "Rõ thưa sir. Tôi đã ghi nhận quy chuẩn frontend và backend."},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.UserFacts) != 2 {
		t.Fatalf("expected 2 user facts, got %d", len(res.UserFacts))
	}
	if res.UserFacts[0].Key != "backend_framework" || res.UserFacts[0].Value != "Go + Fastify" {
		t.Errorf("fact 0 mismatch: %+v", res.UserFacts[0])
	}
	if res.UserFacts[1].Key != "css_style" || res.UserFacts[1].Value != "Vanilla CSS" {
		t.Errorf("fact 1 mismatch: %+v", res.UserFacts[1])
	}

	if len(res.KnowledgeItems) != 1 {
		t.Fatalf("expected 1 knowledge item, got %d", len(res.KnowledgeItems))
	}
	if res.KnowledgeItems[0].Title != "Fix Google Search Scraping with Tavily" {
		t.Errorf("knowledge item title mismatch: %+v", res.KnowledgeItems[0])
	}
}

// TestReflectAndExtract_TruncatedJSON_Recovers tái hiện đúng case thực tế đã
// gặp trong log dev: response bị cắt cụt giữa chừng khi viết field "content"
// dài của knowledge_items (chạm MaxTokens). Trước fix, json.Unmarshal fail
// toàn bộ khiến user_facts đã hoàn chỉnh TRƯỚC chỗ cắt cũng bị mất theo —
// repairTruncatedJSON phải cứu được cả user_facts lẫn knowledge_items (dù
// content của item cuối kết thúc giữa câu).
func TestReflectAndExtract_TruncatedJSON_Recovers(t *testing.T) {
	truncated := `{
  "user_facts": [
    {"category": "user_profile", "key": "role", "value": "backend developer", "confidence": 0.9},
    {"category": "user_profile", "key": "learning_scope", "value": "PostgreSQL for backend only", "confidence": 0.85}
  ],
  "knowledge_items": [
    {
      "title": "Backend-Focused PostgreSQL Learning Priorities",
      "summary": "Focus on SQL fluency and data modeling.",
      "tags": ["postgresql", "backend"],
      "content": "1. SQL thuần thục — CRUD, JOIN.\n2. Data modeling —`

	mockP := &mockReflectionProvider{response: truncated}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Nếu chỉ là backend thôi thì sao"},
		{Role: provider.RoleAssistant, Content: "..."},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.UserFacts) != 2 {
		t.Fatalf("expected 2 user facts recovered từ JSON bị cắt, got %d: %+v", len(res.UserFacts), res.UserFacts)
	}
	if res.UserFacts[0].Key != "role" || res.UserFacts[1].Key != "learning_scope" {
		t.Errorf("user facts mismatch: %+v", res.UserFacts)
	}

	if len(res.KnowledgeItems) != 1 {
		t.Fatalf("expected 1 knowledge item recovered (content dở dang), got %d", len(res.KnowledgeItems))
	}
	if res.KnowledgeItems[0].Title != "Backend-Focused PostgreSQL Learning Priorities" {
		t.Errorf("knowledge item title mismatch: %+v", res.KnowledgeItems[0])
	}
}

// TestReflectAndExtract_TruncatedMidArray_Recovers tái hiện đúng case log dev
// thật: model bị cắt NGAY SAU dấu phẩy giữa lúc liệt kê phần tử mảng "tags"
// (chưa kịp sinh phần tử tiếp theo). repairTruncatedJSON (bản cũ) đóng ngoặc
// ngay sau dấu phẩy đó, để lại trailing comma (`"goroutine",]`) — vẫn không
// phải JSON hợp lệ, khiến cả lượt học mất trắng dù user_facts/phần lớn
// knowledge_items đã hoàn chỉnh.
func TestReflectAndExtract_TruncatedMidArray_Recovers(t *testing.T) {
	truncated := `{
  "user_facts": [],
  "knowledge_items": [
    {
      "title": "Go Concurrency: Goroutines và Channels",
      "summary": "Go xử lý đồng thời bằng goroutine và channel.",
      "tags": ["golang", "concurrency", "goroutine",`

	mockP := &mockReflectionProvider{response: truncated}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "giải thích golang concurrency"},
		{Role: provider.RoleAssistant, Content: "..."},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.KnowledgeItems) != 1 {
		t.Fatalf("expected 1 knowledge item recovered, got %d: %+v", len(res.KnowledgeItems), res.KnowledgeItems)
	}
	if res.KnowledgeItems[0].Title != "Go Concurrency: Goroutines và Channels" {
		t.Errorf("knowledge item title mismatch: %+v", res.KnowledgeItems[0])
	}
}

// TestReflectAndExtract_GarbageJSON_ReturnsEmpty đảm bảo repairTruncatedJSON
// không "cứu ép" input hoàn toàn rác (không phải JSON bị cắt cụt) thành kết
// quả giả — vẫn phải fallback về rỗng như hành vi cũ.
func TestReflectAndExtract_GarbageJSON_ReturnsEmpty(t *testing.T) {
	mockP := &mockReflectionProvider{response: "hoàn toàn không phải JSON, model bị lỗi"}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "test"},
		{Role: provider.RoleAssistant, Content: "test"},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.UserFacts) != 0 || len(res.KnowledgeItems) != 0 {
		t.Fatalf("expected empty result cho garbage input, got %+v", res)
	}
}

// TestReflectAndExtract_UnescapedQuote_RetriesAndRecovers tái hiện đúng bug
// thực tế từ log dev: LLM sinh content chứa dấu ngoặc kép CHƯA escape (vd
// `similarity: "cosine"`) khiến json.Unmarshal fail với lỗi "invalid
// character 'c' after object key:value pair" — KHÁC bug bị cắt cụt
// (repairTruncatedJSON không cứu được case này vì JSON có đủ dấu đóng, chỉ
// là escaping sai giữa chừng). Lần gọi đầu lỗi, lần 2 (retry) phải parse
// được JSON hợp lệ.
func TestReflectAndExtract_UnescapedQuote_RetriesAndRecovers(t *testing.T) {
	// Lỗi thật: "cosine" không được escape bên trong content.
	malformed := `{"user_facts": [], "knowledge_items": [{"title": "Atlas Vector Search", "summary": "Tối ưu index.", "tags": ["mongodb"], "content": "Dùng similarity: "cosine" cho index."}]}`
	valid := `{"user_facts": [], "knowledge_items": [{"title": "Atlas Vector Search", "summary": "Tối ưu index.", "tags": ["mongodb"], "content": "Dùng similarity: cosine cho index."}]}`

	mockP := &sequenceReflectionProvider{responses: []string{malformed, valid}}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "test"},
		{Role: provider.RoleAssistant, Content: "test"},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockP.calls != 2 {
		t.Fatalf("expected 2 lần gọi Generate (1 lỗi + 1 retry thành công), got %d", mockP.calls)
	}
	if len(res.KnowledgeItems) != 1 || res.KnowledgeItems[0].Title != "Atlas Vector Search" {
		t.Fatalf("expected knowledge item khôi phục từ lần retry, got %+v", res)
	}
}

// TestReflectAndExtract_AlwaysMalformed_GivesUpGracefully xác nhận khi TẤT
// CẢ các lần thử (kể cả retry) đều ra JSON hỏng, hàm trả về rỗng êm thay vì
// panic/lỗi — giữ đúng hợp đồng cũ (Learner chỉ log WARN rồi bỏ qua lượt học).
func TestReflectAndExtract_AlwaysMalformed_GivesUpGracefully(t *testing.T) {
	malformed := `{"user_facts": [], "knowledge_items": [{"content": "broken "quote" here"}]}`
	mockP := &sequenceReflectionProvider{responses: []string{malformed, malformed, malformed}}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "test"},
		{Role: provider.RoleAssistant, Content: "test"},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockP.calls != maxReflectionAttempts {
		t.Fatalf("expected đúng %d lần gọi (hết retry budget), got %d", maxReflectionAttempts, mockP.calls)
	}
	if len(res.UserFacts) != 0 || len(res.KnowledgeItems) != 0 {
		t.Fatalf("expected empty result sau khi hết retry, got %+v", res)
	}
}

// TestReflectAndExtract_ChunkError_RetriesAndRecovers tái hiện đúng case từ
// log dev thật: lần gọi đầu provider trả ChunkError giữa stream (không có
// ChunkText nào) khiến fullResp rỗng — trước fix, lỗi hiển thị mơ hồ là
// "unexpected end of JSON input (raw=\"\")"; sau fix, ChunkError được nhận
// diện rõ ràng VÀ vẫn retry được (như log dev cho thấy retry thực tế đã cứu
// được lượt học này).
func TestReflectAndExtract_ChunkError_RetriesAndRecovers(t *testing.T) {
	validResp := `{"user_facts": [], "knowledge_items": [{"title": "So sánh flagship LLM", "summary": "tóm tắt", "tags": ["llm"], "content": "nội dung"}]}`
	mockP := &chunkErrorThenTextProvider{
		errorForCalls: 1,
		err:           errors.New("rate limit exceeded"),
		successResp:   validResp,
	}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "test"},
		{Role: provider.RoleAssistant, Content: "test"},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockP.calls != 2 {
		t.Fatalf("expected 2 lần gọi (1 ChunkError + 1 retry thành công), got %d", mockP.calls)
	}
	if len(res.KnowledgeItems) != 1 || res.KnowledgeItems[0].Title != "So sánh flagship LLM" {
		t.Fatalf("expected khôi phục từ lần retry, got %+v", res)
	}
}

// TestReflectAndExtract_ChunkError_AlwaysFails_GivesUpGracefully xác nhận khi
// MỌI lần thử (kể cả retry) đều gặp ChunkError, hàm trả rỗng êm thay vì
// panic/leak lỗi provider ra ngoài — giữ đúng hợp đồng cũ với Learner.
func TestReflectAndExtract_ChunkError_AlwaysFails_GivesUpGracefully(t *testing.T) {
	mockP := &chunkErrorThenTextProvider{
		errorForCalls: maxReflectionAttempts,
		err:           errors.New("provider tạm thời không khả dụng"),
	}
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "test"},
		{Role: provider.RoleAssistant, Content: "test"},
	}

	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockP.calls != maxReflectionAttempts {
		t.Fatalf("expected đúng %d lần gọi (hết retry budget), got %d", maxReflectionAttempts, mockP.calls)
	}
	if len(res.UserFacts) != 0 || len(res.KnowledgeItems) != 0 {
		t.Fatalf("expected empty result sau khi hết retry, got %+v", res)
	}
}

func TestRepairTruncatedJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "valid JSON không bị đụng",
			in:   `{"a": 1, "b": [1, 2, 3]}`,
			want: `{"a": 1, "b": [1, 2, 3]}`,
		},
		{
			name: "cắt giữa string value",
			in:   `{"a": "hello wor`,
			want: `{"a": "hello wor"}`,
		},
		{
			name: "cắt ngay sau khi đóng string, còn dở object+array",
			in:   `{"items": [{"title": "x"`,
			want: `{"items": [{"title": "x"}]}`,
		},
		{
			name: "chuỗi rỗng không đổi",
			in:   "",
			want: "",
		},
		{
			name: "cắt ngay sau dấu phẩy giữa mảng string — không để lại trailing comma",
			in:   `{"tags": ["golang", "concurrency",`,
			want: `{"tags": ["golang", "concurrency"]}`,
		},
		{
			name: "cắt ngay sau dấu phẩy giữa 2 phần tử object trong mảng",
			in:   `{"items": [{"title": "a"}, `,
			want: `{"items": [{"title": "a"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repairTruncatedJSON(tt.in)
			if got != tt.want {
				t.Errorf("repairTruncatedJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestReflectAndExtract_EmptyMessages(t *testing.T) {
	mockP := &mockReflectionProvider{response: "{}"}
	res, err := ReflectAndExtract(context.Background(), mockP, "mock-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.UserFacts) != 0 || len(res.KnowledgeItems) != 0 {
		t.Fatalf("expected empty result, got %+v", res)
	}
}

func TestLearner_LearnFromConversation(t *testing.T) {
	store := NewStore()
	jsonResp := `{
		"user_facts": [
			{"category": "tech_stack", "key": "ui_library", "value": "Tailwind v4", "confidence": 0.9}
		],
		"knowledge_items": []
	}`

	mockP := &mockReflectionProvider{response: jsonResp}
	learner := NewLearner(store, nil, mockP, "mock-model", nil)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Tôi dùng Tailwind v4 cho dự án."},
		{Role: provider.RoleAssistant, Content: "Đã rõ."},
	}

	// Trigger async learning
	learner.LearnFromConversation(context.Background(), messages, "conv-123")

	// Poll for goroutine completion
	var foundVal string
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		if val, ok := store.Get("default", "ui_library"); ok {
			foundVal = val
			break
		}
	}

	if foundVal != "Tailwind v4" {
		t.Errorf("expected ui_library in store, got %q", foundVal)
	}
}

// TestLearner_LearnFromConversation_TenantIsolation xác nhận fix P0 xuyên
// suốt cả luồng: request context mang tenantID (qua middleware.TenantIDKey,
// giống ChatHandler dùng r.Context() sau khi TenantMiddleware chạy) phải
// được Learner truyền tới goroutine nền và cuối cùng ghi đúng namespace của
// Store — tenant A "học" một fact thì tenant B tuyệt đối không được thấy nó,
// dù dùng chung 1 Store singleton (đúng như wiring thật trong main.go).
func TestLearner_LearnFromConversation_TenantIsolation(t *testing.T) {
	store := NewStore()
	jsonResp := `{
		"user_facts": [
			{"category": "user_profile", "key": "user_name", "value": "Linh", "confidence": 0.9}
		],
		"knowledge_items": []
	}`

	mockP := &mockReflectionProvider{response: jsonResp}
	learner := NewLearner(store, nil, mockP, "mock-model", nil)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Tôi tên là Linh."},
		{Role: provider.RoleAssistant, Content: "Đã rõ."},
	}

	// Giả lập request context của tenant "tenant-a" — đúng như TenantMiddleware
	// sẽ set trước khi ChatHandler gọi LearnFromConversation(r.Context(), ...).
	tenantACtx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-a")
	learner.LearnFromConversation(tenantACtx, messages, "conv-tenant-a")

	// Poll for goroutine completion.
	var found bool
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		if _, ok := store.Get("tenant-a", "user_name"); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected user_name học được nằm trong namespace tenant-a")
	}

	if v, _ := store.Get("tenant-a", "user_name"); v != "Linh" {
		t.Errorf("Get(tenant-a, user_name) = %q, want Linh", v)
	}
	// Tenant khác (kể cả "default" — tenant fallback khi thiếu header
	// X-Tenant-ID) tuyệt đối không được thấy fact vừa học của tenant-a.
	if _, ok := store.Get("tenant-b", "user_name"); ok {
		t.Fatal("tenant-b không được thấy user_name học được của tenant-a — rò rỉ chéo tenant (P0)")
	}
	if _, ok := store.Get("default", "user_name"); ok {
		t.Fatal("tenant \"default\" không được thấy user_name học được của tenant-a — rò rỉ chéo tenant (P0)")
	}
}
