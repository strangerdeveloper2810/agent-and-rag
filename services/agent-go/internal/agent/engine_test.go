package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// collectEvents chạy engine và gom toàn bộ event phát ra.
func collectEvents(t *testing.T, e *Engine, in RunInput) ([]Event, provider.Usage, error) {
	t.Helper()
	var events []Event
	usage, err := e.Run(context.Background(), in, func(ev Event) { events = append(events, ev) })
	return events, usage, err
}

func eventTypes(events []Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

// --- Setters ---

func TestEngine_Setters(t *testing.T) {
	fake := provider.NewFake()
	reg := tools.NewRegistry()
	e := NewEngine(fake, reg)

	if e.Provider() != fake {
		t.Error("Provider() không trả provider đã inject")
	}
	if e.Registry() != reg {
		t.Error("Registry() không trả registry đã inject")
	}

	e.SetSystemPrompt("xin chào")
	if e.getSystemPrompt() != "xin chào" {
		t.Errorf("getSystemPrompt = %q", e.getSystemPrompt())
	}

	e.SetMaxContextTokens(500)
	if e.getMaxContextTokens() != 500 {
		t.Errorf("getMaxContextTokens = %d, want 500", e.getMaxContextTokens())
	}

	cfg := DynamicThinkingConfig{Enabled: true, DefaultOff: true}
	e.SetDynamicThinking(cfg)
	if e.getDynamicThinking() != cfg {
		t.Errorf("getDynamicThinking = %+v", e.getDynamicThinking())
	}

	loader := &skills.Loader{}
	e.SetSkillLoader(loader)
	if e.getSkillLoader() != loader {
		t.Error("getSkillLoader không trả loader đã set")
	}

	e.SetCircuitBreaker(guardrails.NewCircuitBreaker(3))
	if e.circuitBreaker == nil {
		t.Error("SetCircuitBreaker không gán")
	}

	noop := func(context.Context, *State, EmitFunc) (NodeID, error) { return NodeEnd, nil }
	e.SetMemoryNodes(noop, noop, noop)
	if e.recallFn == nil || e.extractFn == nil || e.summarizeFn == nil {
		t.Error("SetMemoryNodes không gán đủ 3 node")
	}

	e.SetPlanningNodes(noop, noop)
	if e.planFn == nil || e.reflectFn == nil {
		t.Error("SetPlanningNodes không gán đủ 2 node")
	}

	e.SetMaxOutputTokens(4096)
	if e.getMaxOutputTokens() != 4096 {
		t.Errorf("getMaxOutputTokens = %d, want 4096", e.getMaxOutputTokens())
	}

	e.SetMaxToolOutput(12345)
	if e.getMaxToolOutput() != 12345 {
		t.Errorf("getMaxToolOutput = %d, want 12345", e.getMaxToolOutput())
	}

	e.SetMaxTotalToolOutput(99999)
	if e.getMaxTotalToolOutput() != 99999 {
		t.Errorf("getMaxTotalToolOutput = %d, want 99999", e.getMaxTotalToolOutput())
	}

	e.SetFastModel("deepseek-v4-flash")
	if e.getFastModel() != "deepseek-v4-flash" {
		t.Errorf("getFastModel = %q, want deepseek-v4-flash", e.getFastModel())
	}

	e.SetAllowDestructiveTools(true)
	if !e.getAllowDestructiveTools() {
		t.Error("SetAllowDestructiveTools(true) không gán")
	}

	e.SetOwnerTenants([]string{"tenant-a", "tenant-b"})
	if got := e.getOwnerTenants(); len(got) != 2 || got[0] != "tenant-a" || got[1] != "tenant-b" {
		t.Errorf("getOwnerTenants = %v, want [tenant-a tenant-b]", got)
	}
}

// NewEngine phải khởi tạo sẵn maxToolOutput/maxTotalToolOutput mặc định hợp lý,
// không phải 0 (0 nghĩa là "không giới hạn" — không phải giá trị an toàn mặc
// định cho một chốt bảo vệ context).
func TestEngine_DefaultToolOutputBudgets(t *testing.T) {
	e := NewEngine(provider.NewFake(), tools.NewRegistry())
	if e.getMaxToolOutput() != defaultMaxToolOutput {
		t.Errorf("getMaxToolOutput mặc định = %d, want %d", e.getMaxToolOutput(), defaultMaxToolOutput)
	}
	if e.getMaxTotalToolOutput() != defaultMaxTotalToolOutput {
		t.Errorf("getMaxTotalToolOutput mặc định = %d, want %d", e.getMaxTotalToolOutput(), defaultMaxTotalToolOutput)
	}
}

func TestEngine_DefaultMaxContextTokens(t *testing.T) {
	e := NewEngine(provider.NewFake(), tools.NewRegistry())
	if e.getMaxContextTokens() != 100000 {
		t.Errorf("mặc định = %d, want 100000", e.getMaxContextTokens())
	}
}

// --- Run e2e ---

func TestEngineRun_TextOnlyEmitsStepsThenDone(t *testing.T) {
	e := NewEngine(provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "chào sir"},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 10, OutputTokens: 4}},
		provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishStop},
	), tools.NewRegistry())

	events, usage, err := collectEvents(t, e, RunInput{UserMessage: "hi"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if usage.InputTokens != 10 || usage.OutputTokens != 4 {
		t.Errorf("usage = %+v, want {10 4}", usage)
	}

	types := eventTypes(events)
	if types[len(types)-1] != "done" {
		t.Errorf("event cuối = %q, want done", types[len(types)-1])
	}

	done := hasEvent(events, "done")
	if done.TotalTokens != 14 {
		t.Errorf("done.TotalTokens = %d, want 14", done.TotalTokens)
	}
	if done.Truncated {
		t.Error("done.Truncated = true cho lượt bình thường")
	}

	// Không có memory node → recall/summarize/plan bị skip, engine vẫn đi tới model.
	var sawModelStep bool
	for _, ev := range events {
		if ev.Type == "step" && ev.Node == string(NodeModel) {
			sawModelStep = true
		}
	}
	if !sawModelStep {
		t.Error("thiếu step model")
	}
}

// Vòng đầy đủ: model gọi tool → tools chạy → model trả lời cuối.
func TestEngineRun_ToolLoop(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&stubTool{name: "echo", kind: tools.KindRead, output: "pong"})

	// FakeProvider phát lại CÙNG kịch bản mỗi lần gọi, nên dùng scriptProvider
	// để lượt 1 gọi tool, lượt 2 trả lời text.
	prov := &scriptProvider{turns: [][]provider.StreamChunk{
		{
			{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "c1", Name: "echo", Args: json.RawMessage(`{}`)}},
			{Kind: provider.ChunkDone, FinishReason: provider.FinishToolCalls},
		},
		{
			{Kind: provider.ChunkText, Text: "tool nói pong"},
			{Kind: provider.ChunkDone, FinishReason: provider.FinishStop},
		},
	}}

	e := NewEngine(prov, reg)
	events, _, err := collectEvents(t, e, RunInput{UserMessage: "gọi echo"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if hasEvent(events, "tool_start") == nil || hasEvent(events, "tool_end") == nil {
		t.Fatalf("thiếu event tool: %v", eventTypes(events))
	}
	var text strings.Builder
	for _, ev := range events {
		if ev.Type == "text" {
			text.WriteString(ev.Text)
		}
	}
	if text.String() != "tool nói pong" {
		t.Errorf("text = %q, want %q", text.String(), "tool nói pong")
	}
	if prov.calls != 2 {
		t.Errorf("số lần gọi LLM = %d, want 2", prov.calls)
	}
}

func TestEngineRun_ProviderErrorReturnsError(t *testing.T) {
	e := NewEngine(&failingProvider{err: errors.New("hết quota")}, tools.NewRegistry())

	events, _, err := collectEvents(t, e, RunInput{UserMessage: "hi"})
	if err == nil {
		t.Fatal("Run = nil error, want lỗi provider")
	}
	if hasEvent(events, "error") == nil {
		t.Error("thiếu event error")
	}
	if hasEvent(events, "done") != nil {
		t.Error("không được phát done khi lỗi")
	}
}

func TestEngineRun_ContextCancelled(t *testing.T) {
	e := NewEngine(provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "x"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	), tools.NewRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.Run(ctx, RunInput{UserMessage: "hi"}, func(Event) {})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// MaxSteps là chốt an toàn: model liên tục gọi tool vẫn phải dừng.
func TestEngineRun_MaxStepsStopsLoop(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&stubTool{name: "echo", kind: tools.KindRead, output: "pong"})

	prov := &loopingToolProvider{}
	e := NewEngine(prov, reg)

	events, _, err := collectEvents(t, e, RunInput{UserMessage: "loop", MaxSteps: 3})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hasEvent(events, "done") == nil {
		t.Error("thiếu done")
	}
	if prov.calls > 4 {
		t.Errorf("gọi LLM %d lần, want <= 4 (MaxSteps=3)", prov.calls)
	}
}

// Circuit breaker: cùng tool + cùng args lặp lại → engine dừng có kiểm soát.
func TestEngineRun_CircuitBreakerStopsRepeatedToolCall(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&stubTool{name: "echo", kind: tools.KindRead, output: "pong"})

	e := NewEngine(&loopingToolProvider{}, reg)
	e.SetCircuitBreaker(guardrails.NewCircuitBreaker(2))

	events, _, err := collectEvents(t, e, RunInput{UserMessage: "loop", MaxSteps: 20})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hasEvent(events, "error") == nil {
		t.Error("circuit breaker phải phát event error")
	}
	if hasEvent(events, "done") == nil {
		t.Error("engine vẫn phải kết thúc bằng done")
	}
}

// oneShotToolProvider gọi tool đúng 1 lần rồi trả text — mô phỏng một request
// BÌNH THƯỜNG (không hề lặp), dùng để chứng minh breaker không được rò state
// từ request trước sang.
type oneShotToolProvider struct{ calls int }

func (p *oneShotToolProvider) Name() string { return "one-shot" }
func (p *oneShotToolProvider) Generate(context.Context, provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	p.calls++
	ch := make(chan provider.StreamChunk, 2)
	if p.calls == 1 {
		ch <- provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "c1", Name: "echo", Args: json.RawMessage(`{"same":"args"}`),
		}}
	} else {
		ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: "xong"}
	}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

// TestEngineRun_CircuitBreakerIsPerRun khoá bug rò state giữa các request:
// engine từng dùng THẲNG một instance CircuitBreaker chia sẻ cho cả 3 agent và
// toàn bộ process, và Reset() không hề được gọi trong production. Hệ quả: 3
// request KHÁC NHAU (khác user) tình cờ gọi cùng tool + cùng args thì request
// thứ 3 bị chặn "stuck loop" oan — tool không chạy, câu trả lời rỗng.
//
// Test chạy 3 lượt Run độc lập trên cùng engine, mỗi lượt chỉ gọi tool 1 lần.
// Không lượt nào được phát event error.
func TestEngineRun_CircuitBreakerIsPerRun(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&stubTool{name: "echo", kind: tools.KindRead, output: "pong"})

	e := NewEngine(&oneShotToolProvider{}, reg)
	e.SetCircuitBreaker(guardrails.NewCircuitBreaker(2))

	for i := 1; i <= 3; i++ {
		// Provider mới mỗi lượt để mỗi Run đều là "gọi tool đúng 1 lần".
		e.prov = &oneShotToolProvider{}
		events, _, err := collectEvents(t, e, RunInput{UserMessage: "cùng câu hỏi", MaxSteps: 20})
		if err != nil {
			t.Fatalf("Run lượt %d: %v", i, err)
		}
		if ev := hasEvent(events, "error"); ev != nil {
			t.Fatalf("lượt %d bị chặn oan bởi circuit breaker (state rò từ lượt trước): %+v", i, ev)
		}
		if hasEvent(events, "done") == nil {
			t.Fatalf("lượt %d không kết thúc bằng done", i)
		}
	}
}

func TestEngine_DispatchUnknownNode(t *testing.T) {
	e := NewEngine(provider.NewFake(), tools.NewRegistry())

	_, err := e.dispatch(context.Background(), NodeID("lạ hoắc"), newState(RunInput{}), func(Event) {})
	if err == nil {
		t.Fatal("dispatch node lạ = nil error, want lỗi")
	}
	if !strings.Contains(err.Error(), "unknown node") {
		t.Errorf("err = %q, want chứa unknown node", err)
	}
}

func TestEngine_DispatchNodeInterrupt(t *testing.T) {
	e := NewEngine(provider.NewFake(), tools.NewRegistry())
	next, err := e.dispatch(context.Background(), NodeInterrupt, newState(RunInput{}), func(Event) {})
	if err != nil {
		t.Fatalf("dispatch NodeInterrupt: %v", err)
	}
	if next != NodeEnd {
		t.Errorf("dispatch NodeInterrupt next = %q, want %q", next, NodeEnd)
	}
}

// Memory node được inject phải được gọi đúng thứ tự recall → summarize → ... → extract.
func TestEngine_MemoryNodesDispatched(t *testing.T) {
	var order []string
	mk := func(name string, next NodeID) Node {
		return func(_ context.Context, _ *State, _ EmitFunc) (NodeID, error) {
			order = append(order, name)
			return next, nil
		}
	}

	e := NewEngine(provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	), tools.NewRegistry())
	e.SetMemoryNodes(
		mk("recall", NodeSummarize),
		mk("extract", NodeEnd),
		mk("summarize", NodeModel),
	)

	if _, _, err := collectEvents(t, e, RunInput{UserMessage: "hi"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"recall", "summarize", "extract"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestEngine_DispatchNodeReflectFallback(t *testing.T) {
	e := NewEngine(provider.NewFake(), tools.NewRegistry())

	next, err := e.dispatch(context.Background(), NodeReflect, newState(RunInput{}), func(Event) {})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if next != NodeExtract {
		t.Errorf("reflect không cài đặt → next = %q, want %q", next, NodeExtract)
	}
}

// EnablePlanning gắn node plan/reflect nội bộ (mặc định TẮT để tiết kiệm 1 LLM call).
func TestEngine_EnablePlanning(t *testing.T) {
	e := NewEngine(provider.NewFake(), tools.NewRegistry())
	if e.planFn != nil || e.reflectFn != nil {
		t.Fatal("planning phải TẮT mặc định")
	}

	e.EnablePlanning()
	if e.planFn == nil || e.reflectFn == nil {
		t.Fatal("EnablePlanning không gán node")
	}
}

// --- Provider giả cho e2e ---

// scriptProvider trả kịch bản khác nhau cho từng lượt gọi.
type scriptProvider struct {
	turns [][]provider.StreamChunk
	calls int
}

func (p *scriptProvider) Name() string { return "script" }
func (p *scriptProvider) Generate(ctx context.Context, _ provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	idx := p.calls
	p.calls++
	if idx >= len(p.turns) {
		idx = len(p.turns) - 1
	}
	ch := make(chan provider.StreamChunk, len(p.turns[idx]))
	for _, c := range p.turns[idx] {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// failingProvider luôn lỗi ngay ở Generate.
type failingProvider struct{ err error }

func (p *failingProvider) Name() string { return "failing" }
func (p *failingProvider) Generate(context.Context, provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	return nil, p.err
}

// loopingToolProvider luôn gọi cùng 1 tool với cùng args (kịch bản kẹt vòng lặp).
type loopingToolProvider struct{ calls int }

func (p *loopingToolProvider) Name() string { return "looping" }
func (p *loopingToolProvider) Generate(context.Context, provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	p.calls++
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
		ID: "loop", Name: "echo", Args: json.RawMessage(`{"same":"args"}`),
	}}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}
