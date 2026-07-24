package orchestrator

import (
	"context"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// newFakeEngine tạo engine với FakeProvider cho test.
func newFakeEngine(name string, chunks ...provider.StreamChunk) *agent.Engine {
	prov := provider.NewFake(chunks...)
	reg := tools.NewRegistry()
	reg.Register(tools.NewEchoTool())
	return agent.NewEngine(prov, reg)
}

func TestOrchestrator_RouteByKeyword(t *testing.T) {
	orch := New()

	// General agent — keyword: "chat", "help", "note"
	genEngine := newFakeEngine("general",
		provider.StreamChunk{Kind: provider.ChunkText, Text: "General agent here."},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	orch.Register(&AgentSpec{
		Name:            "general",
		Description:     "Handles general chat and daily tasks",
		Engine:          genEngine,
		TriggerKeywords: []string{"chat", "help", "note", "nhật ký"},
		SystemPrompt:    "You are a general assistant.",
	})

	// Code agent — keyword: "code", "bug", "debug", "git"
	codeEngine := newFakeEngine("code",
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Code agent here."},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	orch.Register(&AgentSpec{
		Name:            "code",
		Description:     "Handles code analysis, debugging, git operations",
		Engine:          codeEngine,
		TriggerKeywords: []string{"code", "bug", "debug", "git", "lỗi", "fix"},
		SystemPrompt:    "You are a code review expert.",
	})

	tests := []struct {
		name      string
		input     string
		wantAgent string
	}{
		{"code keyword bug", "tìm bug trong code này", "code"},
		{"code keyword fix", "fix lỗi giúp tôi", "code"},
		{"code keyword git", "git log của repo", "code"},
		{"general keyword chat", "chúng ta chat đi", "general"},
		{"general keyword help", "help tôi với", "general"},
		{"no keyword → default", "hôm nay trời đẹp quá", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := orch.route(tt.input)
			if spec == nil {
				t.Fatal("route returned nil")
			}
			if spec.Name != tt.wantAgent {
				t.Errorf("route(%q) = %q, want %q", tt.input, spec.Name, tt.wantAgent)
			}
		})
	}
}

func TestOrchestrator_Run(t *testing.T) {
	orch := New()

	genEngine := newFakeEngine("general",
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Xin chào!"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	orch.Register(&AgentSpec{
		Name:            "general",
		Engine:          genEngine,
		TriggerKeywords: []string{"hello"},
	})

	var events []agent.Event
	emit := func(e agent.Event) { events = append(events, e) }

	input := agent.RunInput{
		UserMessage: "hello",
		MaxSteps:    5,
	}

	_, err := orch.Run(context.Background(), input, emit)
	if err != nil {
		t.Fatal(err)
	}

	// Should have agent event + text + done
	foundAgent := false
	for _, e := range events {
		if e.Type == "agent" {
			foundAgent = true
			if e.Node != "general" {
				t.Errorf("agent event Node = %q, want general", e.Node)
			}
		}
	}
	if !foundAgent {
		t.Error("expected agent event indicating which agent ran")
	}
}

func TestOrchestrator_ListAgents(t *testing.T) {
	orch := New()

	orch.Register(&AgentSpec{
		Name:    "a",
		Engine:  newFakeEngine("a", provider.StreamChunk{Kind: provider.ChunkDone}),
	})
	orch.Register(&AgentSpec{
		Name:    "b",
		Engine:  newFakeEngine("b", provider.StreamChunk{Kind: provider.ChunkDone}),
	})

	list := orch.ListAgents()
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Name != "a" || list[1].Name != "b" {
		t.Errorf("order wrong: %v", list)
	}
}
