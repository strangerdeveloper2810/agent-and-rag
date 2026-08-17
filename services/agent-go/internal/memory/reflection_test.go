package memory

import (
	"context"
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
