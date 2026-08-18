package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// --- SummarizeMessages ---

func TestSummarizeMessages_NilProviderFails(t *testing.T) {
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hi"}}
	if _, ok := SummarizeMessages(context.Background(), nil, "model", msgs); ok {
		t.Error("prov=nil phải fail (ok=false)")
	}
}

func TestSummarizeMessages_EmptyModelFails(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "tóm tắt"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hi"}}
	if _, ok := SummarizeMessages(context.Background(), fake, "", msgs); ok {
		t.Error("model rỗng phải fail (ok=false)")
	}
}

func TestSummarizeMessages_EmptyMessagesFails(t *testing.T) {
	fake := provider.NewFake(provider.StreamChunk{Kind: provider.ChunkDone})
	if _, ok := SummarizeMessages(context.Background(), fake, "model", nil); ok {
		t.Error("msgs rỗng phải fail (ok=false)")
	}
}

// Chỉ message không có Content lẫn ToolCalls (vd role=tool content rỗng) →
// transcript build ra rỗng toàn bộ → fail sớm, không gọi Generate().
func TestSummarizeMessages_AllBlankMessagesFails(t *testing.T) {
	fake := provider.NewFake(provider.StreamChunk{Kind: provider.ChunkDone})
	msgs := []provider.Message{
		{Role: provider.RoleTool, Content: ""},
		{Role: provider.RoleAssistant, Content: ""},
	}
	if _, ok := SummarizeMessages(context.Background(), fake, "model", msgs); ok {
		t.Error("toàn bộ message rỗng phải fail (ok=false)")
	}
}

func TestSummarizeMessages_SuccessReturnsRealSummary(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Người dùng "},
		provider.StreamChunk{Kind: provider.ChunkText, Text: "tên Linh, hỏi về giá."},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "tôi tên Linh"},
		{Role: provider.RoleAssistant, Content: "chào Linh"},
	}

	summary, ok := SummarizeMessages(context.Background(), fake, "fast-model", msgs)
	if !ok {
		t.Fatal("expected ok=true khi provider trả text thật")
	}
	if summary != "Người dùng tên Linh, hỏi về giá." {
		t.Errorf("summary = %q", summary)
	}
	if fake.LastRequest.Options.Model != "fast-model" {
		t.Errorf("Options.Model = %q, want fast-model", fake.LastRequest.Options.Model)
	}
	if fake.LastRequest.Options.ThinkingLevel != provider.ThinkingOff {
		t.Errorf("ThinkingLevel = %q, want off (task nén không cần suy luận dài)", fake.LastRequest.Options.ThinkingLevel)
	}
	if !strings.Contains(fake.LastRequest.Messages[0].Content, "tôi tên Linh") {
		t.Errorf("request messages = %+v, want chứa transcript gốc", fake.LastRequest.Messages)
	}
}

// Tool call không có Content (bình thường) → transcript phải mô tả bằng tên
// tool thay vì để trống, để LLM tóm tắt còn hiểu được đã có hành động gì.
func TestSummarizeMessages_DescribesToolCallsInTranscript(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "đã gọi rag.search"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	msgs := []provider.Message{
		{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "rag.search"},
			},
		},
	}

	if _, ok := SummarizeMessages(context.Background(), fake, "model", msgs); !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(fake.LastRequest.Messages[0].Content, "rag.search") {
		t.Errorf("transcript gửi LLM = %q, want chứa tên tool đã gọi", fake.LastRequest.Messages[0].Content)
	}
}

func TestSummarizeMessages_GenerateErrorFails(t *testing.T) {
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hi"}}
	if _, ok := SummarizeMessages(context.Background(), &erroringProvider{}, "model", msgs); ok {
		t.Error("Generate() trả lỗi phải fail (ok=false)")
	}
}

func TestSummarizeMessages_ChunkErrorMidStreamFails(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "phần đầu"},
		provider.StreamChunk{Kind: provider.ChunkError, Err: errors.New("network lỗi giữa stream")},
	)
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hi"}}
	if summary, ok := SummarizeMessages(context.Background(), fake, "model", msgs); ok {
		t.Errorf("ChunkError giữa stream phải fail (ok=false), got summary=%q", summary)
	}
}

func TestSummarizeMessages_EmptyResponseFails(t *testing.T) {
	fake := provider.NewFake(provider.StreamChunk{Kind: provider.ChunkDone})
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hi"}}
	if _, ok := SummarizeMessages(context.Background(), fake, "model", msgs); ok {
		t.Error("response rỗng (không text) phải fail (ok=false)")
	}
}

// ctx cha hết hạn giữa lúc đọc stream → phải fail, KHÔNG panic, KHÔNG trả
// summary rỗng coi như thành công.
func TestSummarizeMessages_ParentContextTimeoutFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hi"}}
	if _, ok := SummarizeMessages(ctx, &slowProvider{delay: 200 * time.Millisecond}, "model", msgs); ok {
		t.Error("ctx hết hạn giữa stream phải fail (ok=false)")
	}
}

// Transcript quá dài (> maxCompactionInputRunes) không được panic khi cắt —
// đặc biệt phải cắt theo RUNE (không phải byte) để không chẻ giữa ký tự
// multi-byte tiếng Việt.
func TestSummarizeMessages_TruncatesLongTranscriptSafely(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	long := strings.Repeat("ệ", maxCompactionInputRunes+500)
	msgs := []provider.Message{{Role: provider.RoleUser, Content: long}}

	if _, ok := SummarizeMessages(context.Background(), fake, "model", msgs); !ok {
		t.Fatal("expected ok=true")
	}
	// Không assert độ dài chính xác — chỉ cần không panic và request vẫn hợp lệ.
	if len(fake.LastRequest.Messages) == 0 {
		t.Fatal("request rỗng sau khi cắt transcript dài")
	}
}

// erroringProvider luôn trả lỗi từ Generate() (khác lỗi giữa stream).
type erroringProvider struct{}

func (p *erroringProvider) Name() string { return "erroring" }
func (p *erroringProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	return nil, errors.New("provider down")
}

// slowProvider chỉ trả lời sau `delay`, tôn trọng ctx.Done() — dùng để mô
// phỏng timeout mà không phải chờ thật đủ compactionTimeout trong test.
type slowProvider struct{ delay time.Duration }

func (p *slowProvider) Name() string { return "slow" }
func (p *slowProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk)
	go func() {
		defer close(ch)
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

// --- SafeDropBoundary ---

func TestSafeDropBoundary_ZeroOrNegativeReturnsZero(t *testing.T) {
	msgs := []provider.Message{{Role: provider.RoleUser}}
	if got := SafeDropBoundary(msgs, 0); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
	if got := SafeDropBoundary(msgs, -5); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestSafeDropBoundary_ClampsToLenMinusOne(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser}, {Role: provider.RoleUser}, {Role: provider.RoleUser},
	}
	if got := SafeDropBoundary(msgs, 10); got != len(msgs)-1 {
		t.Errorf("got %d, want %d (luôn giữ ít nhất 1 tin)", got, len(msgs)-1)
	}
}

func TestSafeDropBoundary_NoShiftWhenBoundaryIsNotTool(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser}, {Role: provider.RoleAssistant}, {Role: provider.RoleUser},
	}
	if got := SafeDropBoundary(msgs, 1); got != 1 {
		t.Errorf("got %d, want 1 (không cần dịch)", got)
	}
}

func TestSafeDropBoundary_ShiftsPastOrphanedToolResult(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "echo"}}},
		{Role: provider.RoleTool, ToolCallID: "c1"}, // sẽ mồ côi nếu dropCount=2
		{Role: provider.RoleUser},
	}
	got := SafeDropBoundary(msgs, 2)
	if got != 3 {
		t.Errorf("got %d, want 3 (dịch qua role=tool mồ côi)", got)
	}
	if msgs[got].Role == provider.RoleTool {
		t.Errorf("Messages[%d].Role = tool, boundary vẫn rơi vào tool result mồ côi", got)
	}
}

// Toàn bộ đuôi là role=tool liên tiếp (pathological) → vẫn phải dừng lại ở
// len-1 để luôn giữ ít nhất 1 tin, không loop vô hạn / vượt biên mảng.
func TestSafeDropBoundary_StopsAtLastMessageEvenIfAllToolTail(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser},
		{Role: provider.RoleTool, ToolCallID: "c1"},
		{Role: provider.RoleTool, ToolCallID: "c2"},
		{Role: provider.RoleTool, ToolCallID: "c3"},
	}
	got := SafeDropBoundary(msgs, 1)
	if got != len(msgs)-1 {
		t.Errorf("got %d, want %d (dừng ở message cuối)", got, len(msgs)-1)
	}
}
