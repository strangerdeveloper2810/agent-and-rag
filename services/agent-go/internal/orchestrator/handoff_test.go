package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

func textEngine(text string) *agent.Engine {
	return newFakeEngine("x",
		provider.StreamChunk{Kind: provider.ChunkText, Text: text},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
}

func twoAgentOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	o := New()
	o.Register(&AgentSpec{Name: "general", Engine: textEngine("general trả lời"), TriggerKeywords: []string{"chat"}})
	o.Register(&AgentSpec{Name: "code", Engine: textEngine("code trả lời"), TriggerKeywords: []string{"bug"}})
	return o
}

// --- SetDefault ---

func TestSetDefault(t *testing.T) {
	o := twoAgentOrchestrator(t)

	// Agent đăng ký đầu tiên là default.
	if o.defaultAgent != "general" {
		t.Errorf("default = %q, want general", o.defaultAgent)
	}

	if err := o.SetDefault("code"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if o.defaultAgent != "code" {
		t.Errorf("default = %q, want code", o.defaultAgent)
	}

	// Input không match keyword nào → rơi về default mới.
	if spec := o.route("xin chào"); spec.Name != "code" {
		t.Errorf("route fallback = %q, want code", spec.Name)
	}
}

func TestSetDefault_UnknownAgent(t *testing.T) {
	o := twoAgentOrchestrator(t)

	err := o.SetDefault("không-có")
	if err == nil {
		t.Fatal("SetDefault agent lạ = nil error, want lỗi")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("err = %q, want chứa not registered", err)
	}
	if o.defaultAgent != "general" {
		t.Errorf("default bị đổi thành %q dù SetDefault lỗi", o.defaultAgent)
	}
}

// --- GetAgent ---

func TestGetAgent(t *testing.T) {
	o := twoAgentOrchestrator(t)

	if spec := o.GetAgent("code"); spec == nil || spec.Name != "code" {
		t.Errorf("GetAgent(code) = %+v", spec)
	}
	if spec := o.GetAgent("không-có"); spec != nil {
		t.Errorf("GetAgent(lạ) = %+v, want nil", spec)
	}
}

// Register cùng tên 2 lần: ghi đè spec, KHÔNG nhân đôi thứ tự.
func TestRegister_OverwritesSameName(t *testing.T) {
	o := New()
	o.Register(&AgentSpec{Name: "a", Engine: textEngine("v1")})
	o.Register(&AgentSpec{Name: "a", Engine: textEngine("v2"), Description: "bản mới"})

	if len(o.ListAgents()) != 1 {
		t.Errorf("ListAgents len = %d, want 1", len(o.ListAgents()))
	}
	if o.GetAgent("a").Description != "bản mới" {
		t.Error("Register không ghi đè spec cũ")
	}
}

// --- Delegate ---

func TestDelegate_RunsTargetAgent(t *testing.T) {
	o := twoAgentOrchestrator(t)

	res, err := o.Delegate(context.Background(), HandoffRequest{
		From:    "general",
		To:      "code",
		Context: "user đang debug",
		Task:    "sửa lỗi nil pointer",
	})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}
	if res.Agent != "code" {
		t.Errorf("Agent = %q, want code", res.Agent)
	}
	if res.Result != "code trả lời" {
		t.Errorf("Result = %q, want %q", res.Result, "code trả lời")
	}
}

func TestDelegate_UnknownTarget(t *testing.T) {
	o := twoAgentOrchestrator(t)

	_, err := o.Delegate(context.Background(), HandoffRequest{From: "general", To: "không-có", Task: "x"})
	if err == nil {
		t.Fatal("Delegate tới agent lạ = nil error, want lỗi")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %q, want chứa not found", err)
	}
}

func TestDelegate_PropagatesEngineError(t *testing.T) {
	o := New()
	// Engine không có chunk nào + ctx huỷ → Engine.Run trả lỗi.
	o.Register(&AgentSpec{Name: "code", Engine: agent.NewEngine(provider.NewFake(), tools.NewRegistry())})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := o.Delegate(ctx, HandoffRequest{From: "general", To: "code", Task: "x"})
	if err == nil {
		t.Fatal("Delegate = nil error, want lỗi từ engine")
	}
	if !strings.Contains(err.Error(), "handoff") {
		t.Errorf("err = %q, want chứa handoff", err)
	}
}

// --- truncate ---

func TestTruncate(t *testing.T) {
	if got := truncate("ngắn", 100); got != "ngắn" {
		t.Errorf("truncate = %q, want giữ nguyên", got)
	}

	long := strings.Repeat("a", 150)
	got := truncate(long, 100)
	if len(got) != 103 || !strings.HasSuffix(got, "...") {
		t.Errorf("truncate độ dài = %d, want 103 và kết thúc bằng ...", len(got))
	}

	if got := truncate("", 10); got != "" {
		t.Errorf("truncate(\"\") = %q, want rỗng", got)
	}
}

// --- Run: emit event agent ---

func TestRun_EmitsAgentEvent(t *testing.T) {
	o := twoAgentOrchestrator(t)

	var events []agent.Event
	if _, err := o.Run(context.Background(), agent.RunInput{UserMessage: "có bug rồi"},
		func(e agent.Event) { events = append(events, e) }); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(events) == 0 || events[0].Type != "agent" {
		t.Fatalf("event đầu = %+v, want type agent", events)
	}
	if events[0].Node != "code" {
		t.Errorf("agent event Node = %q, want code (keyword 'bug')", events[0].Node)
	}
}

func TestDelegate_DepthLimitDefault(t *testing.T) {
	o := twoAgentOrchestrator(t)

	// Depth = default max (4) → chặn ngay, không chạy engine.
	_, err := o.Delegate(context.Background(), HandoffRequest{From: "general", To: "code", Task: "x", Depth: 4})
	if err == nil {
		t.Fatal("Delegate ở depth = max = nil error, want DelegationDepthExceededError")
	}
	var depthErr *DelegationDepthExceededError
	if !errors.As(err, &depthErr) {
		t.Fatalf("err = %v (%T), want *DelegationDepthExceededError", err, err)
	}
	if depthErr.Max != defaultMaxDelegationDepth {
		t.Errorf("Max = %d, want %d", depthErr.Max, defaultMaxDelegationDepth)
	}

	// Depth = max-1 vẫn phải chạy được bình thường.
	res, err := o.Delegate(context.Background(), HandoffRequest{From: "general", To: "code", Task: "x", Depth: 3})
	if err != nil {
		t.Fatalf("Delegate ở depth = max-1: %v", err)
	}
	if res.Agent != "code" {
		t.Errorf("Agent = %q, want code", res.Agent)
	}
}

func TestSetMaxDelegationDepth(t *testing.T) {
	o := twoAgentOrchestrator(t)
	o.SetMaxDelegationDepth(1)

	if _, err := o.Delegate(context.Background(), HandoffRequest{From: "general", To: "code", Task: "x", Depth: 1}); err == nil {
		t.Fatal("Delegate ở depth = 1 sau SetMaxDelegationDepth(1) = nil error, want lỗi")
	}
	if _, err := o.Delegate(context.Background(), HandoffRequest{From: "general", To: "code", Task: "x", Depth: 0}); err != nil {
		t.Fatalf("Delegate ở depth = 0: %v", err)
	}
}

func TestSetMaxDelegationDepth_NonPositiveResetsToDefault(t *testing.T) {
	o := twoAgentOrchestrator(t)
	o.SetMaxDelegationDepth(1)
	o.SetMaxDelegationDepth(0) // reset về default

	if o.maxDelegationDepth != defaultMaxDelegationDepth {
		t.Errorf("maxDelegationDepth = %d, want %d", o.maxDelegationDepth, defaultMaxDelegationDepth)
	}
}
