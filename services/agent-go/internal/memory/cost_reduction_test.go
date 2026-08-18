package memory

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// promptCapturingProvider ghi lại prompt ĐÃ GỬI để test đo được đúng thứ tốn tiền.
type promptCapturingProvider struct {
	mu       sync.Mutex
	lastUser string
	lastSys  string
	calls    int
}

func (p *promptCapturingProvider) Name() string { return "capture" }

func (p *promptCapturingProvider) Generate(_ context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	p.mu.Lock()
	p.calls++
	p.lastSys = req.System
	if len(req.Messages) > 0 {
		p.lastUser = req.Messages[len(req.Messages)-1].Content
	}
	p.mu.Unlock()

	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: `{"user_facts":[],"knowledge_items":[]}`}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (p *promptCapturingProvider) userPrompt() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastUser
}

// Learner chạy sau MỖI câu trả lời, nên gửi lại toàn bộ hội thoại mỗi lượt là
// trả tiền nhiều lần cho cùng một đoạn text. Reflection chỉ được nhận các lượt
// CUỐI (lượt cũ đã reflect ở lần trước).
func TestReflectAndExtract_ChiGuiCacLuotCuoi(t *testing.T) {
	prov := &promptCapturingProvider{}

	// 10 message: 5 cặp trao đổi. Mỗi câu có mốc riêng để biết câu nào bị gửi.
	var msgs []provider.Message
	for i := 1; i <= 5; i++ {
		msgs = append(msgs,
			provider.Message{Role: provider.RoleUser, Content: "CAU-HOI-" + string(rune('0'+i))},
			provider.Message{Role: provider.RoleAssistant, Content: "TRA-LOI-" + string(rune('0'+i))},
		)
	}

	if _, err := ReflectAndExtract(context.Background(), prov, "deepseek-v4-flash", msgs); err != nil {
		t.Fatalf("ReflectAndExtract lỗi: %v", err)
	}

	sent := prov.userPrompt()

	// Phải có lượt mới nhất.
	for _, want := range []string{"CAU-HOI-5", "TRA-LOI-5"} {
		if !strings.Contains(sent, want) {
			t.Errorf("prompt thiếu %q — lượt hiện tại phải được reflect", want)
		}
	}

	// KHÔNG được có các lượt cũ (đã reflect ở những lần gọi trước).
	for _, notWant := range []string{"CAU-HOI-1", "TRA-LOI-1", "CAU-HOI-2", "CAU-HOI-3"} {
		if strings.Contains(sent, notWant) {
			t.Errorf("prompt vẫn chứa %q — đang trả tiền lặp cho lượt cũ", notWant)
		}
	}
}

// Tool message không được chiếm slot của maxReflectionMessages: nếu cắt TRƯỚC khi
// lọc role thì một lượt có nhiều tool call sẽ đẩy hết user/assistant ra ngoài và
// reflection nhận prompt rỗng.
func TestReflectAndExtract_ToolMessageKhongChiemSlot(t *testing.T) {
	prov := &promptCapturingProvider{}

	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "CAU-HOI-THAT"},
		{Role: provider.RoleAssistant, Content: "TRA-LOI-THAT"},
		{Role: provider.RoleTool, Content: "tool output 1"},
		{Role: provider.RoleTool, Content: "tool output 2"},
		{Role: provider.RoleTool, Content: "tool output 3"},
		{Role: provider.RoleTool, Content: "tool output 4"},
	}

	if _, err := ReflectAndExtract(context.Background(), prov, "m", msgs); err != nil {
		t.Fatalf("ReflectAndExtract lỗi: %v", err)
	}

	sent := prov.userPrompt()
	if !strings.Contains(sent, "CAU-HOI-THAT") || !strings.Contains(sent, "TRA-LOI-THAT") {
		t.Errorf("prompt mất đoạn hội thoại thật vì tool message chiếm slot: %q", sent)
	}
	if strings.Contains(sent, "tool output") {
		t.Error("tool output bị đưa vào prompt reflection — chỉ cần user/assistant")
	}
}

// countingEmbedder đếm số lần gọi API embedding (Voyage) — mỗi lần là tiền + latency.
type countingEmbedder struct {
	mu    sync.Mutex
	calls int
}

func (c *countingEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	out := make([][]float64, len(texts))
	for i := range out {
		out[i] = []float64{0.1, 0.2, 0.3}
	}
	return out, nil
}

func (c *countingEmbedder) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestRecallNode_KhongGoiEmbeddingKhiKeywordDaTimDuoc(t *testing.T) {
	emb := &countingEmbedder{}
	store := NewStore()
	store.SetEmbedder(emb)

	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-a")

	// Set() cũng embed để phục vụ semantic search → reset bộ đếm sau khi seed
	// để chỉ đếm phần recall.
	store.Set("tenant-a", "user_name", "An")
	before := emb.count()

	node := RecallNode(store)
	s := &agent.State{Messages: []provider.Message{
		{Role: provider.RoleUser, Content: "tên tôi là gì?"},
	}}

	if _, err := node(ctx, s, func(agent.Event) {}); err != nil {
		t.Fatalf("RecallNode lỗi: %v", err)
	}

	if len(s.RecalledMemories) == 0 {
		t.Fatal("không recall được gì — keyword lookup phải khớp 'tên' → user_name")
	}
	if got := emb.count() - before; got != 0 {
		t.Errorf("gọi embedding %d lần dù keyword đã tìm được — đang tốn tiền vô ích", got)
	}
}

func TestRecallNode_VanGoiEmbeddingKhiKeywordKhongRaGi(t *testing.T) {
	emb := &countingEmbedder{}
	store := NewStore()
	store.SetEmbedder(emb)

	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-a")
	store.Set("tenant-a", "framework", "Fastify")
	before := emb.count()

	node := RecallNode(store)
	// Câu hỏi diễn đạt hoàn toàn khác cách fact được lưu → keyword không khớp,
	// đây đúng là lúc semantic search có giá trị và PHẢI chạy.
	s := &agent.State{Messages: []provider.Message{
		{Role: provider.RoleUser, Content: "hệ thống backend đang chạy trên nền gì nhỉ"},
	}}

	if _, err := node(ctx, s, func(agent.Event) {}); err != nil {
		t.Fatalf("RecallNode lỗi: %v", err)
	}

	if got := emb.count() - before; got == 0 {
		t.Error("không gọi embedding khi keyword không ra gì — mất luôn semantic search")
	}
}
