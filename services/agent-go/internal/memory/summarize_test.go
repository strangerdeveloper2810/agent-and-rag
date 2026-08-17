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

	next, err := SummarizeNode()(context.Background(), s, emit)

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

func TestSummarizeNode_OneMessageOver(t *testing.T) {
	s := &agent.State{Messages: fillMessages(summarizeThreshold + 1)}
	emit, events := collectEmit()

	next, err := SummarizeNode()(context.Background(), s, emit)

	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	// 1 system note + 15 tin cuối.
	if len(s.Messages) != summarizeThreshold+1 {
		t.Fatalf("len(Messages) = %d, want %d", len(s.Messages), summarizeThreshold+1)
	}
	if s.Messages[0].Role != provider.RoleSystem {
		t.Fatalf("Messages[0].Role = %q, want system", s.Messages[0].Role)
	}
	if !strings.Contains(s.Messages[0].Content, "1 tin nhắn") {
		t.Fatalf("Messages[0].Content = %q, want chứa số lượng rút gọn", s.Messages[0].Content)
	}
	if s.Messages[1].Content != "msg-1" {
		t.Fatalf("Messages[1].Content = %q, want msg-1 (giữ 15 tin cuối)", s.Messages[1].Content)
	}
	if len(*events) != 1 || !strings.Contains((*events)[0].Message, "summarized") {
		t.Fatalf("events = %v, want 1 event summarized", *events)
	}
}

func TestSummarizeNode_ManyExcess(t *testing.T) {
	total := summarizeThreshold + 15
	s := &agent.State{Messages: fillMessages(total)}
	emit, events := collectEmit()

	_, err := SummarizeNode()(context.Background(), s, emit)
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
