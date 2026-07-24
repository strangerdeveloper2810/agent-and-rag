package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// fakeEngine là engine tối thiểu chỉ để test node model (có provider + registry).
type fakeEngine struct {
	prov     provider.Provider
	registry *tools.Registry
}

func (e *fakeEngine) getProvider() provider.Provider { return e.prov }
func (e *fakeEngine) getRegistry() *tools.Registry   { return e.registry }

func TestNodeModel_TextOnly(t *testing.T) {
	// Kịch bản đơn giản nhất: LLM trả về text, không tool call.
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Xin chào!"},
		provider.StreamChunk{Kind: provider.ChunkText, Text: " Tôi là AI."},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 10, OutputTokens: 5}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{
		UserMessage: "Hello",
		MaxSteps:    12,
	})

	var events []Event
	emit := func(e Event) { events = append(events, e) }

	next, err := nodeModel(context.Background(), eng, s, emit)
	if err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}
	if next != NodeEnd {
		t.Fatalf("next = %q, want %q", next, NodeEnd)
	}

	// Kiểm tra events: phải có text, done.
	if len(events) < 1 {
		t.Fatal("expected at least 1 event")
	}
	texts := collectTexts(events)
	combined := strings.Join(texts, "")
	if combined != "Xin chào! Tôi là AI." {
		t.Errorf("combined text = %q, want %q", combined, "Xin chào! Tôi là AI.")
	}

	// Kiểm tra state: phải có assistant message, step=1, usage được cộng.
	if s.Step != 1 {
		t.Errorf("Step = %d, want 1", s.Step)
	}
	last := s.LastAssistant()
	if last == nil {
		t.Fatal("LastAssistant() = nil")
	}
	if last.Role != provider.RoleAssistant {
		t.Errorf("last.Role = %q, want assistant", last.Role)
	}
	if last.Content != "Xin chào! Tôi là AI." {
		t.Errorf("last.Content = %q", last.Content)
	}
	if s.Usage.InputTokens != 10 || s.Usage.OutputTokens != 5 {
		t.Errorf("Usage = %+v, want {10 5}", s.Usage)
	}
}

func TestNodeModel_WithToolCall(t *testing.T) {
	// LLM trả về 1 tool_call → router phải trả về NodeTools.
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "c1", Name: "echo", Args: nil,
		}},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5, OutputTokens: 3}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "echo test", MaxSteps: 12})

	next, err := nodeModel(context.Background(), eng, s, nilEmit)
	if err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}
	if next != NodeTools {
		t.Fatalf("next = %q, want %q (tool call → tools)", next, NodeTools)
	}

	// Kiểm tra assistant message có tool call.
	last := s.LastAssistant()
	if last == nil || len(last.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %+v", last)
	}
	if last.ToolCalls[0].Name != "echo" {
		t.Errorf("tool name = %q, want echo", last.ToolCalls[0].Name)
	}
}

func TestNodeModel_ProviderError(t *testing.T) {
	// Provider trả về chunk lỗi → nodeModel phải trả về error.
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkError, Err: context.DeadlineExceeded},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "test", MaxSteps: 12})

	_, err := nodeModel(context.Background(), eng, s, nilEmit)
	if err == nil {
		t.Fatal("expected error from provider, got nil")
	}
}

// --- helpers cho test ---

var nilEmit EmitFunc = func(Event) {}

func collectTexts(events []Event) []string {
	var texts []string
	for _, e := range events {
		if e.Type == "text" {
			texts = append(texts, e.Text)
		}
	}
	return texts
}
