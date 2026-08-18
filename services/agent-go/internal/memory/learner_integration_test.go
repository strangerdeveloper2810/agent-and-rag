package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// scriptedLearnerProvider trả một JSON reflection cố định (hoặc lỗi) — đủ để
// chạy trọn đường học mà không cần LLM thật, không cần Mongo.
type scriptedLearnerProvider struct {
	json string
	err  error
}

func (s *scriptedLearnerProvider) Name() string { return "scripted-learner" }

func (s *scriptedLearnerProvider) Generate(_ context.Context, _ provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: s.json}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func tenantCtx(id string) context.Context {
	return context.WithValue(context.Background(), middleware.TenantIDKey, id)
}

// waitFor chờ điều kiện thành true trong tối đa d — learner chạy nền nên không
// thể assert ngay sau khi gọi.
func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// Integration test: đi trọn đường LearnFromConversation → ReflectAndExtract →
// Store, với Mongo = nil (môi trường CI không có Atlas).
func TestLearnFromConversation_LuuFactVaoStoreTheoTenant(t *testing.T) {
	prov := &scriptedLearnerProvider{json: `{
		"user_facts": [
			{"category":"tech_stack","key":"backend_framework","value":"Go + Fastify","confidence":0.9},
			{"category":"user_profile","key":"user_name","value":"An","confidence":0.95}
		],
		"knowledge_items": []
	}`}

	store := NewStore()
	l := NewLearner(store, nil, prov, "deepseek-v4-flash", nil)

	msgs := exchange(
		"Dự án của tôi dùng Go với Fastify, tôi tên An",
		"Ghi nhận rồi nhé.",
	)
	l.LearnFromConversation(tenantCtx("tenant-a"), msgs, "conv-1")

	ok := waitFor(2*time.Second, func() bool {
		_, found := store.Get("tenant-a", "backend_framework")
		return found
	})
	if !ok {
		t.Fatal("fact không được lưu vào store sau khi reflection xong")
	}

	if v, _ := store.Get("tenant-a", "backend_framework"); v != "Go + Fastify" {
		t.Errorf("backend_framework = %q, want %q", v, "Go + Fastify")
	}
	if v, _ := store.Get("tenant-a", "user_name"); v != "An" {
		t.Errorf("user_name = %q, want %q", v, "An")
	}

	// Fact PHẢI scoped theo tenant — tenant khác không được thấy.
	if _, found := store.Get("tenant-b", "user_name"); found {
		t.Error("tenant-b thấy được fact của tenant-a — rò rỉ giữa tenant")
	}
}

func TestLearnFromConversation_BoQuaFactRong(t *testing.T) {
	prov := &scriptedLearnerProvider{json: `{
		"user_facts": [
			{"category":"rule","key":"   ","value":"khong co key","confidence":0.9},
			{"category":"rule","key":"co_key","value":"   ","confidence":0.9},
			{"category":"rule","key":"hop_le","value":"giu lai","confidence":0.9}
		],
		"knowledge_items": [
			{"title":"","summary":"thieu title","tags":[],"content":"abc"},
			{"title":"co title","summary":"thieu content","tags":[],"content":""}
		]
	}`}

	store := NewStore()
	l := NewLearner(store, nil, prov, "deepseek-v4-flash", nil)
	l.LearnFromConversation(tenantCtx("tenant-a"),
		exchange("Quy ước của tôi là dùng snake_case cho tên bảng", "Rõ."), "conv-1")

	if !waitFor(2*time.Second, func() bool {
		_, found := store.Get("tenant-a", "hop_le")
		return found
	}) {
		t.Fatal("fact hợp lệ không được lưu")
	}

	// Key rỗng và value rỗng đều phải bị bỏ, không tạo rác trong store.
	if _, found := store.Get("tenant-a", "   "); found {
		t.Error("fact với key rỗng vẫn được lưu")
	}
	if _, found := store.Get("tenant-a", "co_key"); found {
		t.Error("fact với value rỗng vẫn được lưu")
	}
}

func TestLearnFromConversation_ReflectionLoi_KhongPanic(t *testing.T) {
	prov := &scriptedLearnerProvider{err: errors.New("provider sập")}

	store := NewStore()
	l := NewLearner(store, nil, prov, "deepseek-v4-flash", nil)
	l.LearnFromConversation(tenantCtx("tenant-a"),
		exchange("Dự án của tôi dùng Postgres và Redis nhé", "Rõ."), "conv-1")

	// Chờ đủ lâu để goroutine nền chạy xong; điều cần khẳng định là không panic
	// và store không có gì.
	time.Sleep(200 * time.Millisecond)

	if n := len(store.All("tenant-a")); n != 0 {
		t.Errorf("store có %d item sau khi reflection lỗi, want 0", n)
	}
}

func TestLearnFromConversation_LearnerNilVaProviderNil(t *testing.T) {
	// Không được panic khi learner chưa được cấu hình (provider nil / receiver nil).
	var nilLearner *Learner
	nilLearner.LearnFromConversation(tenantCtx("t"), exchange("Dự án dùng Go nhé", "ok"), "c")

	l := NewLearner(NewStore(), nil, nil, "m", nil)
	l.LearnFromConversation(tenantCtx("t"), exchange("Dự án dùng Go nhé", "ok"), "c")

	// Ít hơn 2 message cũng phải bỏ qua.
	l2 := NewLearner(NewStore(), nil, &scriptedLearnerProvider{json: `{}`}, "m", nil)
	l2.LearnFromConversation(tenantCtx("t"),
		[]provider.Message{{Role: provider.RoleUser, Content: "chỉ có một tin nhắn dài đủ để qua gate"}}, "c")
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Sửa lỗi Google Search", "s-a-l-i-google-search"},
		{"Hello World", "hello-world"},
		{"  spaces  ", "spaces"},
		{"Multiple---Dashes", "multiple-dashes"},
		{"UPPER_case_123", "upper-case-123"},
		{"!!!", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := slugify(tc.in); got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
