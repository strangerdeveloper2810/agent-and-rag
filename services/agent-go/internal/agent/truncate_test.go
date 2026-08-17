package agent

import (
	"context"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// hasEvent reports whether events contains an event of the given type.
func hasEvent(events []Event, typ string) *Event {
	for i := range events {
		if events[i].Type == typ {
			return &events[i]
		}
	}
	return nil
}

func TestNodeModel_TruncatedByLength(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Câu trả lời rất dài"},
		provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishLength},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "viết dài vào", MaxSteps: 12})

	var events []Event
	emit := func(e Event) { events = append(events, e) }

	if _, err := nodeModel(context.Background(), eng, s, emit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	if !s.Truncated {
		t.Error("s.Truncated = false, want true")
	}

	ev := hasEvent(events, "truncated")
	if ev == nil {
		t.Fatal("no truncated event emitted")
	}
	if !ev.Truncated {
		t.Error("truncated event has Truncated = false")
	}
	if ev.Message == "" {
		t.Error("truncated event has empty Message")
	}

	// Text đã stream vẫn phải được giữ nguyên trong assistant message.
	last := s.LastAssistant()
	if last == nil || last.Content != "Câu trả lời rất dài" {
		t.Errorf("assistant content = %+v, want partial text preserved", last)
	}
}

func TestNodeModel_NotTruncatedOnStop(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "xong"},
		provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishStop},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})

	var events []Event
	emit := func(e Event) { events = append(events, e) }

	if _, err := nodeModel(context.Background(), eng, s, emit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}
	if s.Truncated {
		t.Error("s.Truncated = true, want false for finish reason stop")
	}
	if ev := hasEvent(events, "truncated"); ev != nil {
		t.Error("truncated event emitted for a normal stop")
	}
}

func TestNodeModel_MissingFinishReasonIsNotTruncated(t *testing.T) {
	// Provider cũ không set FinishReason → không được coi là bị cắt.
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "xong"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})

	if _, err := nodeModel(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}
	if s.Truncated {
		t.Error("s.Truncated = true, want false when provider reports no finish reason")
	}
}

func TestDoneEvent_CarriesTruncatedFlag(t *testing.T) {
	e := DoneEvent(provider.Usage{InputTokens: 1, OutputTokens: 2}, 3, true)
	if e.Type != "done" || !e.Truncated {
		t.Errorf("DoneEvent = %+v, want type=done truncated=true", e)
	}

	e = DoneEvent(provider.Usage{}, 0, false)
	if e.Truncated {
		t.Error("DoneEvent(..., false).Truncated = true")
	}
}

func TestEngineRun_EmitsTruncatedThenDone(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "dở dang"},
		provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishLength},
	)

	eng := NewEngine(fake, tools.NewRegistry())

	var events []Event
	if _, err := eng.Run(context.Background(), RunInput{UserMessage: "hi"}, func(e Event) {
		events = append(events, e)
	}); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if hasEvent(events, "truncated") == nil {
		t.Error("engine did not forward truncated event")
	}
	done := hasEvent(events, "done")
	if done == nil {
		t.Fatal("no done event")
	}
	if !done.Truncated {
		t.Error("done event Truncated = false, want true")
	}
}
