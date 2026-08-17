package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// stubTool là tool test có thể cấu hình Kind, output và lỗi.
type stubTool struct {
	name   string
	kind   tools.Kind
	output string
	err    error
}

func (s *stubTool) Name() string            { return s.name }
func (s *stubTool) Description() string     { return "stub " + s.name }
func (s *stubTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (s *stubTool) Kind() tools.Kind        { return s.kind }
func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	if s.err != nil {
		return tools.Result{}, s.err
	}
	return tools.Result{Content: s.output}, nil
}

// toolsOnlyEngine chỉ cần registry (interface toolsEngine).
type toolsOnlyEngine struct{ registry *tools.Registry }

func (e *toolsOnlyEngine) getRegistry() *tools.Registry { return e.registry }

func regWith(ts ...tools.Tool) *tools.Registry {
	r := tools.NewRegistry()
	for _, t := range ts {
		r.Register(t)
	}
	return r
}

// stateWithToolCalls dựng State có 1 assistant message kèm các tool call.
func stateWithToolCalls(names ...string) *State {
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})
	tcs := make([]provider.ToolCall, 0, len(names))
	for i, name := range names {
		tcs = append(tcs, provider.ToolCall{ID: fmt.Sprintf("c%d", i), Name: name})
	}
	s.Messages = append(s.Messages, provider.Message{Role: provider.RoleAssistant, ToolCalls: tcs})
	return s
}

func TestNodeTools_NoToolCallsGoesBackToModel(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith()}
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})

	next, err := nodeTools(context.Background(), eng, s, func(Event) {})
	if err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if next != NodeModel {
		t.Errorf("next = %q, want %q", next, NodeModel)
	}
}

func TestNodeTools_RunsReadToolAndRecordsObservation(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith(&stubTool{name: "echo", kind: tools.KindRead, output: "kết quả"})}
	s := stateWithToolCalls("echo")

	var events []Event
	next, err := nodeTools(context.Background(), eng, s, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if next != NodeModel {
		t.Errorf("next = %q, want %q", next, NodeModel)
	}

	if hasEvent(events, "tool_start") == nil {
		t.Error("thiếu event tool_start")
	}
	end := hasEvent(events, "tool_end")
	if end == nil {
		t.Fatal("thiếu event tool_end")
	}
	if end.Text != "kết quả" || end.Message != "" {
		t.Errorf("tool_end = %+v, want Text=kết quả", end)
	}

	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Output != "kết quả" {
		t.Errorf("Scratchpad = %+v", s.Scratchpad)
	}
	// Kết quả tool phải được thêm vào Messages dưới dạng role=tool.
	last := s.Messages[len(s.Messages)-1]
	if string(last.Role) != "tool" || last.Content != "kết quả" {
		t.Errorf("message cuối = %+v, want role=tool", last)
	}
}

func TestNodeTools_ToolErrorEmitsErrorEnd(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith(&stubTool{
		name: "boom", kind: tools.KindRead, err: errors.New("hỏng rồi"),
	})}
	s := stateWithToolCalls("boom")

	var events []Event
	if _, err := nodeTools(context.Background(), eng, s, func(e Event) { events = append(events, e) }); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}

	end := hasEvent(events, "tool_end")
	if end == nil {
		t.Fatal("thiếu event tool_end")
	}
	if !strings.Contains(end.Message, "hỏng rồi") {
		t.Errorf("tool_end.Message = %q, want chứa 'hỏng rồi'", end.Message)
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Error == "" {
		t.Errorf("Scratchpad phải ghi lỗi: %+v", s.Scratchpad)
	}
}

// Tool KindDestructive phải dừng chờ xác nhận (HITL), không chạy.
func TestNodeTools_DestructiveToolInterrupts(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith(&stubTool{name: "task.delete", kind: tools.KindDestructive})}
	s := stateWithToolCalls("task.delete")

	var events []Event
	next, err := nodeTools(context.Background(), eng, s, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if next != NodeInterrupt {
		t.Errorf("next = %q, want %q", next, NodeInterrupt)
	}
	if s.Interrupt == nil || s.Interrupt.Tool != "task.delete" {
		t.Fatalf("Interrupt = %+v", s.Interrupt)
	}
	if s.Interrupt.Reason != "confirm_destructive" {
		t.Errorf("Reason = %q, want confirm_destructive", s.Interrupt.Reason)
	}

	ev := hasEvent(events, "interrupt")
	if ev == nil {
		t.Fatal("thiếu event interrupt")
	}
	// Tool huỷ diệt không được chạy → không có observation.
	if len(s.Scratchpad) != 0 {
		t.Errorf("Scratchpad = %+v, want rỗng (chưa xác nhận)", s.Scratchpad)
	}
}

// Tool KindWrite được phép chạy thẳng, không cần xác nhận.
func TestNodeTools_WriteToolRuns(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith(&stubTool{name: "memory.save", kind: tools.KindWrite, output: "đã lưu"})}
	s := stateWithToolCalls("memory.save")

	next, err := nodeTools(context.Background(), eng, s, func(Event) {})
	if err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if next != NodeModel || s.Interrupt != nil {
		t.Errorf("next = %q Interrupt = %+v, want model / nil", next, s.Interrupt)
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Output != "đã lưu" {
		t.Errorf("Scratchpad = %+v", s.Scratchpad)
	}
}

// Tool không có trong registry vẫn được chạy để registry báo lỗi "unknown tool".
func TestNodeTools_UnknownToolStillReported(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith()}
	s := stateWithToolCalls("không-tồn-tại")

	var events []Event
	if _, err := nodeTools(context.Background(), eng, s, func(e Event) { events = append(events, e) }); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}

	end := hasEvent(events, "tool_end")
	if end == nil {
		t.Fatal("thiếu event tool_end")
	}
	if end.Message == "" {
		t.Error("tool lạ phải trả lỗi trong tool_end.Message")
	}
}

// Trộn tool an toàn + tool huỷ diệt: tool an toàn vẫn chạy, đồng thời dừng chờ xác nhận.
func TestNodeTools_MixedSafeAndDestructive(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith(
		&stubTool{name: "echo", kind: tools.KindRead, output: "ok"},
		&stubTool{name: "task.delete", kind: tools.KindDestructive},
	)}
	s := stateWithToolCalls("echo", "task.delete")

	next, err := nodeTools(context.Background(), eng, s, func(Event) {})
	if err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if next != NodeInterrupt {
		t.Errorf("next = %q, want %q", next, NodeInterrupt)
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Name != "echo" {
		t.Errorf("Scratchpad = %+v, want chỉ có echo", s.Scratchpad)
	}
}

func TestToolResultPreview(t *testing.T) {
	if got := toolResultPreview("  gọn  "); got != "gọn" {
		t.Errorf("preview = %q, want %q (phải trim)", got, "gọn")
	}

	long := strings.Repeat("a", toolResultPreviewMax+50)
	got := toolResultPreview(long)
	if !strings.HasSuffix(got, "…") {
		t.Error("output dài phải kết thúc bằng …")
	}
	if len([]rune(got)) != toolResultPreviewMax+1 {
		t.Errorf("độ dài preview = %d rune, want %d", len([]rune(got)), toolResultPreviewMax+1)
	}

	if got := toolResultPreview(""); got != "" {
		t.Errorf("preview(\"\") = %q, want rỗng", got)
	}
}

func TestAppendObservation(t *testing.T) {
	s := newState(RunInput{UserMessage: "hi"})

	s.AppendObservation(Observation{CallID: "c1", Name: "echo", Output: "ok"})
	if len(s.Scratchpad) != 1 {
		t.Fatalf("Scratchpad len = %d, want 1", len(s.Scratchpad))
	}
	last := s.Messages[len(s.Messages)-1]
	if last.ToolCallID != "c1" || last.Content != "ok" {
		t.Errorf("message = %+v", last)
	}

	// Observation lỗi phải ghi "ERROR: ..." vào content cho LLM thấy.
	s.AppendObservation(Observation{CallID: "c2", Name: "boom", Error: "toang"})
	last = s.Messages[len(s.Messages)-1]
	if !strings.HasPrefix(last.Content, "ERROR: ") || !strings.Contains(last.Content, "toang") {
		t.Errorf("content = %q, want ERROR: toang", last.Content)
	}
}
