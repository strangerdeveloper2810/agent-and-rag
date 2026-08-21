package memory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// safeCapture bọc 1 chuỗi bằng mutex — cần vì mockReflectionProvider.onGenerate
// chạy trong goroutine NỀN của Learner.LearnFromConversation, còn test đọc lại
// (waitFor/assert) từ goroutine chính; một string trần không có lock ở đây bị
// go test -race bắt được là data race thật.
type safeCapture struct {
	mu  sync.Mutex
	val string
}

func (c *safeCapture) set(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.val = v
}

func (c *safeCapture) get() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val
}

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

// TestLearner_BatchTurns_SkipsUntilNthTurn khoá đúng hành vi batch: khi
// SetBatchTurns(3), 2 lượt đầu KHÔNG gọi provider (dù có fact đáng học), chỉ
// lượt thứ 3 mới thực sự chạy reflection.
func TestLearner_BatchTurns_SkipsUntilNthTurn(t *testing.T) {
	spy := &gateSpyProvider{}
	ctx := tenantCtx("tenant-batch")

	l := NewLearner(NewStore(), nil, spy, "deepseek-v4-flash", nil)
	l.SetBatchTurns(3)

	l.LearnFromConversation(ctx, exchange("tôi tên An", "Chào An!"), "conv-batch")
	l.LearnFromConversation(ctx, exchange("tôi thích Go", "Ghi nhận!"), "conv-batch")
	time.Sleep(150 * time.Millisecond)
	if n := spy.calls.Load(); n != 0 {
		t.Fatalf("provider bị gọi %d lần trước lượt thứ N — batch không hoạt động", n)
	}

	// "tôi cần Postgres" (từ khoá "cần") — PHẢI qua được gate worthLearning
	// (Task 3 đã siết gate: câu ngắn không có từ khoá bị bỏ qua), nếu không
	// batch counter không tăng và test này fail vì lý do khác, không liên
	// quan đến batching.
	l.LearnFromConversation(ctx, exchange("tôi cần Postgres", "Ghi nhận!"), "conv-batch")
	if !waitFor(2*time.Second, func() bool { return spy.calls.Load() > 0 }) {
		t.Error("lượt thứ N (đủ batch) nhưng provider không được gọi")
	}
}

// Mặc định (không gọi SetBatchTurns) PHẢI giữ hành vi cũ: gọi ngay lượt đầu.
// Đây là test hồi quy quan trọng nhất — bảo vệ 2 test cũ trong
// learner_gate_test.go khỏi bị batch mặc định phá vỡ.
func TestLearner_DefaultBatchTurns_IsOne(t *testing.T) {
	spy := &gateSpyProvider{}
	ctx := tenantCtx("tenant-default")

	l := NewLearner(NewStore(), nil, spy, "deepseek-v4-flash", nil)
	l.LearnFromConversation(ctx, exchange("tôi tên Bình", "Chào Bình!"), "conv-default")

	if !waitFor(2*time.Second, func() bool { return spy.calls.Load() > 0 }) {
		t.Error("batch mặc định phải = 1 (gọi ngay), không được hoãn")
	}
}

// TestLearner_DefaultBatchTurns_PreservesFullWindow là test hồi quy cho Bug 1
// (window-sizing): với batchTurns MẶC ĐỊNH (1, không gọi SetBatchTurns), cửa
// sổ tin nhắn gửi tới reflection PHẢI giữ nguyên hành vi cũ của
// ReflectAndExtract — tối thiểu maxReflectionMessages (4 = 2 lượt trao đổi:
// lượt hiện tại + 1 lượt trước làm ngữ cảnh). Công thức cũ 2*batchTurns cho
// N=1 ra window=2, co hẹp MỘT NỬA so với hành vi cũ, làm mất lượt trước —
// đúng như báo cáo review đã tái hiện bằng thực nghiệm.
func TestLearner_DefaultBatchTurns_PreservesFullWindow(t *testing.T) {
	captured := &safeCapture{}
	mockP := &mockReflectionProvider{
		response: `{"user_facts":[],"knowledge_items":[]}`,
		onGenerate: func(req provider.GenerateRequest) {
			captured.set(req.Messages[0].Content)
		},
	}

	l := NewLearner(NewStore(), nil, mockP, "deepseek-v4-flash", nil)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Lượt trước user"},
		{Role: provider.RoleAssistant, Content: "Lượt trước assistant"},
		{Role: provider.RoleUser, Content: "tôi tên An"},
		{Role: provider.RoleAssistant, Content: "Chào An!"},
	}

	l.LearnFromConversation(tenantCtx("tenant-window-default"), messages, "conv-window-default")

	if !waitFor(2*time.Second, func() bool { return captured.get() != "" }) {
		t.Fatal("provider không được gọi")
	}
	if prompt := captured.get(); !strings.Contains(prompt, "Lượt trước user") {
		t.Errorf("prompt thiếu 'Lượt trước user' — batchTurns mặc định=1 phải giữ cửa sổ cũ "+
			"(tối thiểu 4 tin nhắn = 2 lượt), không co hẹp về 2*batchTurns=2: %q", prompt)
	}
}

// TestLearner_BatchTurns_WindowCoversAllRawTurnsInBatch là test hồi quy cho
// Bug 2 (window-sizing): khi có lượt TÁN GẪU bị gate worthLearning chặn xen
// giữa 1 batch (không tăng batchTurns counter), cửa sổ tin nhắn khi batch
// thực sự fire vẫn phải đủ rộng để bao gồm lượt fact ĐẦU TIÊN của batch đó.
// Nếu chỉ dùng công thức cố định 2*batchTurns (chỉ đếm lượt "đáng học"), lượt
// tán gẫu xen giữa vẫn nằm trong lịch sử ĐẦY ĐỦ được gửi lại mỗi lần, đẩy lượt
// fact đầu tiên ra ngoài cửa sổ — đúng bug đã được review tái hiện.
func TestLearner_BatchTurns_WindowCoversAllRawTurnsInBatch(t *testing.T) {
	captured := &safeCapture{}
	mockP := &mockReflectionProvider{
		response: `{"user_facts":[],"knowledge_items":[]}`,
		onGenerate: func(req provider.GenerateRequest) {
			captured.set(req.Messages[0].Content)
		},
	}

	l := NewLearner(NewStore(), nil, mockP, "deepseek-v4-flash", nil)
	l.SetBatchTurns(3)
	ctx := tenantCtx("tenant-window-batch")

	var history []provider.Message
	appendTurn := func(user, assistant string) {
		history = append(history,
			provider.Message{Role: provider.RoleUser, Content: user},
			provider.Message{Role: provider.RoleAssistant, Content: assistant},
		)
	}

	// Lượt A: fact (từ khoá "tên") → batch counter → 1.
	appendTurn("tôi tên An-FACT-A", "Chào An!")
	l.LearnFromConversation(ctx, history, "conv-window-batch")

	// Lượt B: tán gẫu ("cảm ơn nhé" — xem learner_gate_test.go) → gate chặn,
	// KHÔNG tăng batch counter, nhưng VẪN nằm trong lịch sử đầy đủ.
	appendTurn("cảm ơn nhé", "Không có gì!")
	l.LearnFromConversation(ctx, history, "conv-window-batch")

	// Lượt C: fact (từ khoá "thích") → batch counter → 2.
	appendTurn("tôi thích Go-FACT-C", "Ghi nhận!")
	l.LearnFromConversation(ctx, history, "conv-window-batch")

	time.Sleep(150 * time.Millisecond)
	if prompt := captured.get(); prompt != "" {
		t.Fatalf("provider bị gọi trước khi đủ batch: %q", prompt)
	}

	// Lượt D: fact (từ khoá "cần") → batch counter → 3 → fires.
	appendTurn("tôi cần Postgres-FACT-D", "Ghi nhận!")
	l.LearnFromConversation(ctx, history, "conv-window-batch")

	if !waitFor(2*time.Second, func() bool { return captured.get() != "" }) {
		t.Fatal("lượt thứ N (đủ batch) nhưng provider không được gọi")
	}
	prompt := captured.get()
	if !strings.Contains(prompt, "FACT-A") {
		t.Errorf("prompt thiếu nội dung lượt A (lượt fact ĐẦU TIÊN của batch) — window bị co hẹp "+
			"bởi lượt tán gẫu (B) xen giữa: %q", prompt)
	}
	if !strings.Contains(prompt, "FACT-C") || !strings.Contains(prompt, "FACT-D") {
		t.Errorf("prompt thiếu lượt C hoặc D: %q", prompt)
	}
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
