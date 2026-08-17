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
