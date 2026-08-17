package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// --- isComplexRequest / extractJSONArray ---

func TestIsComplexRequest(t *testing.T) {
	complex := []string{
		"lập plan cho dự án", "give me a timeline", "steps to deploy",
		"break down this task", "vẽ roadmap", "chia thành phases",
		"MILESTONES của quý 4", "sequence of actions",
	}
	for _, in := range complex {
		if !isComplexRequest(in) {
			t.Errorf("isComplexRequest(%q) = false, want true", in)
		}
	}

	simple := []string{"chào bạn", "2+2 bằng mấy", "thời tiết hôm nay"}
	for _, in := range simple {
		if isComplexRequest(in) {
			t.Errorf("isComplexRequest(%q) = true, want false", in)
		}
	}
}

func TestExtractJSONArray(t *testing.T) {
	cases := map[string]string{
		`["a","b"]`:                     `["a","b"]`,
		"đây là plan: [\"a\"] xong nhé": `["a"]`,
		"```json\n[\"a\",\"b\"]\n```":   `["a","b"]`,
		"không có mảng":                 "không có mảng",
		"]":                             "]",
		"[":                             "[",
	}
	for in, want := range cases {
		if got := extractJSONArray(in); got != want {
			t.Errorf("extractJSONArray(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- nodePlan ---

func planEngine(chunks ...provider.StreamChunk) *fakeEngine {
	return &fakeEngine{prov: provider.NewFake(chunks...), registry: tools.NewRegistry()}
}

func TestNodePlan_SkipsSimpleRequest(t *testing.T) {
	eng := planEngine()
	s := newState(RunInput{UserMessage: "chào bạn"})

	next, err := nodePlan(context.Background(), eng, s, func(Event) {})
	if err != nil {
		t.Fatalf("nodePlan: %v", err)
	}
	if next != NodeModel {
		t.Errorf("next = %q, want %q", next, NodeModel)
	}
	if len(s.Plan) != 0 {
		t.Errorf("Plan = %v, want rỗng (request đơn giản)", s.Plan)
	}
}

func TestNodePlan_SkipsWhenPlanExists(t *testing.T) {
	eng := planEngine()
	s := newState(RunInput{UserMessage: "lập plan"})
	s.Plan = []string{"đã có"}

	next, err := nodePlan(context.Background(), eng, s, func(Event) {})
	if err != nil {
		t.Fatalf("nodePlan: %v", err)
	}
	if next != NodeModel || len(s.Plan) != 1 {
		t.Errorf("next = %q Plan = %v", next, s.Plan)
	}
}

func TestNodePlan_ParsesPlan(t *testing.T) {
	eng := planEngine(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "```json\n[\"Bước 1\",\"Bước 2\"]\n```"},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5, OutputTokens: 6}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	s := newState(RunInput{UserMessage: "lập plan triển khai"})

	var events []Event
	next, err := nodePlan(context.Background(), eng, s, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("nodePlan: %v", err)
	}
	if next != NodeModel {
		t.Errorf("next = %q, want %q", next, NodeModel)
	}
	if len(s.Plan) != 2 || s.Plan[0] != "Bước 1" {
		t.Errorf("Plan = %v", s.Plan)
	}
	if s.PlanStep != 0 {
		t.Errorf("PlanStep = %d, want 0", s.PlanStep)
	}
	if s.TotalTokens != 11 {
		t.Errorf("TotalTokens = %d, want 11", s.TotalTokens)
	}
	if hasEvent(events, "plan") == nil {
		t.Error("thiếu event plan")
	}
}

func TestNodePlan_DegradesOnBadOutput(t *testing.T) {
	cases := map[string][]provider.StreamChunk{
		"text rỗng": {
			{Kind: provider.ChunkDone},
		},
		"không phải JSON": {
			{Kind: provider.ChunkText, Text: "tôi không biết"},
			{Kind: provider.ChunkDone},
		},
		"mảng rỗng": {
			{Kind: provider.ChunkText, Text: "[]"},
			{Kind: provider.ChunkDone},
		},
		"stream lỗi": {
			{Kind: provider.ChunkError, Err: context.DeadlineExceeded},
		},
	}

	for name, chunks := range cases {
		t.Run(name, func(t *testing.T) {
			eng := planEngine(chunks...)
			s := newState(RunInput{UserMessage: "lập plan triển khai"})

			next, err := nodePlan(context.Background(), eng, s, func(Event) {})
			if err != nil {
				t.Fatalf("nodePlan phải degrade êm, got err = %v", err)
			}
			if next != NodeModel {
				t.Errorf("next = %q, want %q", next, NodeModel)
			}
			if len(s.Plan) != 0 {
				t.Errorf("Plan = %v, want rỗng", s.Plan)
			}
		})
	}
}

func TestNodePlan_ProviderErrorSkips(t *testing.T) {
	eng := &fakeEngine{prov: &failingProvider{err: context.DeadlineExceeded}, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "lập plan triển khai"})

	next, err := nodePlan(context.Background(), eng, s, func(Event) {})
	if err != nil {
		t.Fatalf("nodePlan: %v", err)
	}
	if next != NodeModel || len(s.Plan) != 0 {
		t.Errorf("next = %q Plan = %v", next, s.Plan)
	}
}

// --- nodeReflect ---

func TestNodeReflect_NoPlanGoesToExtract(t *testing.T) {
	s := newState(RunInput{UserMessage: "hi"})

	next, err := nodeReflect(context.Background(), s, func(Event) {})
	if err != nil {
		t.Fatalf("nodeReflect: %v", err)
	}
	if next != NodeExtract {
		t.Errorf("next = %q, want %q", next, NodeExtract)
	}
}

func TestNodeReflect_AdvancesPlanAndInjectsNextStep(t *testing.T) {
	s := newState(RunInput{UserMessage: "hi"})
	s.Plan = []string{"B1", "B2", "B3"}
	before := len(s.Messages)

	var events []Event
	next, err := nodeReflect(context.Background(), s, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("nodeReflect: %v", err)
	}
	if next != NodeModel {
		t.Errorf("next = %q, want %q", next, NodeModel)
	}
	if s.PlanStep != 1 {
		t.Errorf("PlanStep = %d, want 1", s.PlanStep)
	}
	if len(s.Messages) != before+1 {
		t.Fatalf("Messages len = %d, want %d (thêm 1 chỉ dẫn bước kế)", len(s.Messages), before+1)
	}
	last := s.Messages[len(s.Messages)-1]
	if last.Role != provider.RoleUser || !strings.Contains(last.Content, "B2") {
		t.Errorf("message chèn = %+v, want nhắc bước B2", last)
	}
	if hasEvent(events, "reflect") == nil {
		t.Error("thiếu event reflect")
	}
}

func TestNodeReflect_LastStepFinishesPlan(t *testing.T) {
	s := newState(RunInput{UserMessage: "hi"})
	s.Plan = []string{"B1", "B2"}
	s.PlanStep = 1

	next, err := nodeReflect(context.Background(), s, func(Event) {})
	if err != nil {
		t.Fatalf("nodeReflect: %v", err)
	}
	if next != NodeExtract {
		t.Errorf("next = %q, want %q (plan xong)", next, NodeExtract)
	}
	if s.PlanStep != 2 {
		t.Errorf("PlanStep = %d, want 2", s.PlanStep)
	}
}

func TestNodeReflect_PlanStepBeyondEnd(t *testing.T) {
	s := newState(RunInput{UserMessage: "hi"})
	s.Plan = []string{"B1"}
	s.PlanStep = 5

	next, err := nodeReflect(context.Background(), s, func(Event) {})
	if err != nil {
		t.Fatalf("nodeReflect: %v", err)
	}
	if next != NodeExtract {
		t.Errorf("next = %q, want %q", next, NodeExtract)
	}
}
