package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func fillMessages(n int) []provider.Message {
	msgs := make([]provider.Message, n)
	for i := range msgs {
		msgs[i] = provider.Message{Role: provider.RoleUser, Content: fmt.Sprintf("msg-%d", i)}
	}
	return msgs
}

func TestSummarizeNode_AtThresholdNoChange(t *testing.T) {
	msgs := fillMessages(summarizeThreshold)
	s := &agent.State{Messages: msgs}
	emit, events := collectEmit()

	next, err := SummarizeNode(nil, "")(context.Background(), s, emit)

	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	if len(s.Messages) != summarizeThreshold {
		t.Fatalf("len(Messages) = %d, want %d (không đổi)", len(s.Messages), summarizeThreshold)
	}
	if len(*events) != 0 {
		t.Fatalf("events = %v, want 0", *events)
	}
}

// Không có provider (nil) → agent.SummarizeMessages luôn thất bại → note
// PHẢI nói THẬT là đã lược bỏ, không giả vờ đã tóm tắt như bản cũ.
func TestSummarizeNode_OneMessageOver_HonestFallbackWhenNoProvider(t *testing.T) {
	s := &agent.State{Messages: fillMessages(summarizeThreshold + 1)}
	emit, events := collectEmit()

	next, err := SummarizeNode(nil, "")(context.Background(), s, emit)

	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	// 1 note + 15 tin cuối.
	if len(s.Messages) != summarizeThreshold+1 {
		t.Fatalf("len(Messages) = %d, want %d", len(s.Messages), summarizeThreshold+1)
	}
	// role=user (không phải RoleSystem) — Anthropic adapter bỏ qua hoàn toàn
	// mọi message role=system nằm trong Messages, sẽ làm mất nội dung âm thầm.
	if s.Messages[0].Role != provider.RoleUser {
		t.Fatalf("Messages[0].Role = %q, want user", s.Messages[0].Role)
	}
	if !strings.Contains(s.Messages[0].Content, "1 tin nhắn") {
		t.Fatalf("Messages[0].Content = %q, want chứa số lượng rút gọn", s.Messages[0].Content)
	}
	if !strings.Contains(s.Messages[0].Content, "không tóm tắt được") {
		t.Fatalf("Messages[0].Content = %q, want fallback trung thực", s.Messages[0].Content)
	}
	if s.Messages[1].Content != "msg-1" {
		t.Fatalf("Messages[1].Content = %q, want msg-1 (giữ 15 tin cuối)", s.Messages[1].Content)
	}
	if len(*events) != 1 || !strings.Contains((*events)[0].Message, "summarized") {
		t.Fatalf("events = %v, want 1 event summarized", *events)
	}
}

// Có provider trả tóm tắt thật → note PHẢI chứa đúng nội dung đó, không phải
// placeholder lược bỏ.
func TestSummarizeNode_OneMessageOver_RealSummaryOnSuccess(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Người dùng đã hỏi 1 câu trước đó."},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	s := &agent.State{Messages: fillMessages(summarizeThreshold + 1)}
	emit, events := collectEmit()

	next, err := SummarizeNode(fake, "fast-model")(context.Background(), s, emit)

	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	if !strings.Contains(s.Messages[0].Content, "Người dùng đã hỏi 1 câu trước đó.") {
		t.Fatalf("Messages[0].Content = %q, want chứa tóm tắt thật", s.Messages[0].Content)
	}
	if strings.Contains(s.Messages[0].Content, "không tóm tắt được") {
		t.Fatalf("Messages[0].Content = %q, không được lẫn fallback khi đã tóm tắt thành công", s.Messages[0].Content)
	}
	_ = events
}

func TestSummarizeNode_ManyExcess(t *testing.T) {
	total := summarizeThreshold + 15
	s := &agent.State{Messages: fillMessages(total)}
	emit, events := collectEmit()

	_, err := SummarizeNode(nil, "")(context.Background(), s, emit)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(s.Messages) != summarizeThreshold+1 {
		t.Fatalf("len(Messages) = %d, want %d", len(s.Messages), summarizeThreshold+1)
	}
	if !strings.Contains(s.Messages[0].Content, "15 tin nhắn") {
		t.Fatalf("Messages[0].Content = %q, want chứa 15", s.Messages[0].Content)
	}
	if s.Messages[1].Content != "msg-15" {
		t.Fatalf("Messages[1].Content = %q, want msg-15", s.Messages[1].Content)
	}
	// Tin cuối cùng phải được giữ nguyên.
	if s.Messages[len(s.Messages)-1].Content != fmt.Sprintf("msg-%d", total-1) {
		t.Fatalf("tin cuối = %q, want msg-%d", s.Messages[len(s.Messages)-1].Content, total-1)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %v, want 1", *events)
	}
}
