package memory

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func exchange(user, assistant string) []provider.Message {
	return []provider.Message{
		{Role: provider.RoleUser, Content: user},
		{Role: provider.RoleAssistant, Content: assistant},
	}
}

func TestWorthLearning(t *testing.T) {
	longAnswer := ""
	for len([]rune(longAnswer)) <= trivialAssistantRunes {
		longAnswer += "Giải pháp là dùng index composite trên (tenant_id, created_at). "
	}

	cases := []struct {
		name string
		msgs []provider.Message
		want bool
	}{
		{
			name: "tán gẫu ngắn hai đầu → bỏ qua",
			msgs: exchange("Xin chào", "Chào bạn! Tôi giúp gì được?"),
			want: false,
		},
		{
			name: "cảm ơn → bỏ qua",
			msgs: exchange("cảm ơn nhé", "Không có gì!"),
			want: false,
		},
		{
			name: "câu ngắn NHƯNG có từ khoá fact → phải học",
			msgs: exchange("tôi tên An", "Chào An!"),
			want: true,
		},
		{
			name: "câu ngắn có 'thích' → phải học",
			msgs: exchange("tôi thích Go", "Ghi nhận!"),
			want: true,
		},
		{
			name: "câu user dài → phải học",
			msgs: exchange(
				"Dự án của tôi dùng Fastify với Postgres, quy ước đặt tên bảng là snake_case nhé",
				"Rõ rồi.",
			),
			want: true,
		},
		{
			name: "user ngắn + trả lời dài nhưng KHÔNG có từ khoá fact → bỏ qua (siết gate)",
			msgs: exchange("sao chậm?", longAnswer),
			want: false, // trước đây true — điều kiện "assistant dài" đã bị bỏ vì gần như luôn đúng, vô hiệu hoá gate
		},
		{
			name: "user ngắn CÓ từ khoá fact + trả lời dài → vẫn phải học",
			msgs: exchange("tôi thích Go", longAnswer),
			want: true,
		},
		{
			name: "không có tin nhắn user → không học",
			msgs: []provider.Message{{Role: provider.RoleAssistant, Content: "xin chào"}},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := worthLearning(tc.msgs); got != tc.want {
				t.Errorf("worthLearning() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLastByRole_LayTinNhanGanNhat(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "câu cũ"},
		{Role: provider.RoleAssistant, Content: "trả lời cũ"},
		{Role: provider.RoleUser, Content: "câu mới"},
		{Role: provider.RoleAssistant, Content: "trả lời mới"},
	}
	user, assistant := lastByRole(msgs)
	if user != "câu mới" {
		t.Errorf("user = %q, want %q", user, "câu mới")
	}
	if assistant != "trả lời mới" {
		t.Errorf("assistant = %q, want %q", assistant, "trả lời mới")
	}
}

// gateSpyProvider đếm số lần bị gọi — dùng để chứng minh lượt tán gẫu KHÔNG
// tạo ra request LLM nào.
type gateSpyProvider struct {
	calls atomic.Int64
}

func (g *gateSpyProvider) Name() string { return "spy" }

func (g *gateSpyProvider) Generate(_ context.Context, _ provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	g.calls.Add(1)
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: `{"user_facts":[],"knowledge_items":[]}`}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

// Learner phải im lặng bỏ qua lượt tán gẫu, KHÔNG gọi provider.
func TestLearnFromConversation_KhongGoiProviderKhiTanGau(t *testing.T) {
	spy := &gateSpyProvider{}
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-a")

	l := NewLearner(NewStore(), nil, spy, "deepseek-v4-flash", nil)
	l.LearnFromConversation(ctx, exchange("Xin chào", "Chào bạn!"), "conv-1")

	// Reflection chạy trong goroutine nền — chờ một nhịp để nếu nó CÓ gọi thì
	// test vẫn bắt được, thay vì pass giả vì chưa kịp chạy.
	time.Sleep(150 * time.Millisecond)

	if n := spy.calls.Load(); n != 0 {
		t.Errorf("provider bị gọi %d lần cho lượt tán gẫu — đang tốn token vô ích", n)
	}
}

// Ngược lại: lượt có fact thật thì PHẢI học (gate không được chặn oan).
func TestLearnFromConversation_VanHocKhiCoFact(t *testing.T) {
	spy := &gateSpyProvider{}
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-a")

	l := NewLearner(NewStore(), nil, spy, "deepseek-v4-flash", nil)
	l.LearnFromConversation(ctx, exchange("tôi tên An", "Chào An!"), "conv-1")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if spy.calls.Load() > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("lượt có fact nhưng learner không gọi provider — gate chặn oan")
}
