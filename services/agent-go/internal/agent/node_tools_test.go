package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
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
type toolsOnlyEngine struct {
	ownerTenants     []string
	registry         *tools.Registry
	maxToolOutput    int
	allowDestructive bool
}

func (e *toolsOnlyEngine) getRegistry() *tools.Registry   { return e.registry }
func (e *toolsOnlyEngine) getMaxToolOutput() int          { return e.maxToolOutput }
func (e *toolsOnlyEngine) getAllowDestructiveTools() bool { return e.allowDestructive }
func (e *toolsOnlyEngine) getOwnerTenants() []string      { return e.ownerTenants }

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

// TestNodeTools_DestructiveEmitsExplanation khoá bug UX: khi guardrails chặn
// tool destructive, engine dừng ở NodeInterrupt và vẫn emit done bình thường
// nhưng KHÔNG có text nào — user nhận một bubble RỖNG HOÀN TOÀN, không lỗi,
// không lý do, không cách nào tiếp tục (FE cũng đang bỏ qua event interrupt).
func TestNodeTools_DestructiveEmitsExplanation(t *testing.T) {
	eng := &toolsOnlyEngine{registry: regWith(&stubTool{name: "shell.exec", kind: tools.KindDestructive})}
	s := stateWithToolCalls("shell.exec")

	var events []Event
	if _, err := nodeTools(context.Background(), eng, s, func(e Event) { events = append(events, e) }); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}

	txt := hasEvent(events, "text")
	if txt == nil {
		t.Fatal("thiếu event text giải thích — user sẽ nhận bubble rỗng")
	}
	for _, want := range []string{"shell.exec", "ALLOW_DESTRUCTIVE_TOOLS"} {
		if !strings.Contains(txt.Text, want) {
			t.Errorf("text giải thích thiếu %q:\n%s", want, txt.Text)
		}
	}
}

// Khi người dùng chủ động bật ALLOW_DESTRUCTIVE_TOOLS, tool destructive được
// chạy thẳng (không interrupt, không thông báo chặn).
func TestNodeTools_AllowDestructiveRunsTool(t *testing.T) {
	eng := &toolsOnlyEngine{
		registry:         regWith(&stubTool{name: "shell.exec", kind: tools.KindDestructive, output: "hello"}),
		allowDestructive: true,
	}
	s := stateWithToolCalls("shell.exec")

	var events []Event
	next, err := nodeTools(context.Background(), eng, s, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if next != NodeModel {
		t.Errorf("next = %q, want %q (không interrupt khi đã cho phép)", next, NodeModel)
	}
	if s.Interrupt != nil {
		t.Errorf("Interrupt = %+v, want nil", s.Interrupt)
	}
	if hasEvent(events, "interrupt") != nil {
		t.Error("không được emit interrupt khi ALLOW_DESTRUCTIVE_TOOLS=true")
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Output != "hello" {
		t.Errorf("Scratchpad = %+v, want tool đã chạy", s.Scratchpad)
	}
}

// TestNodeTools_CapsToolOutput khoá chốt an toàn tập trung: cfg.MaxToolOutput
// từng là config CHẾT (không nơi nào đọc), và file.search/rag.search không cắt
// output gì cả → có thể đẩy hàng MB vào context.
func TestNodeTools_CapsToolOutput(t *testing.T) {
	huge := strings.Repeat("á", 5000) // ký tự multi-byte để bắt luôn lỗi cắt theo byte
	eng := &toolsOnlyEngine{
		registry:      regWith(&stubTool{name: "file.search", kind: tools.KindRead, output: huge}),
		maxToolOutput: 1000,
	}
	s := stateWithToolCalls("file.search")

	if _, err := nodeTools(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}

	if len(s.Scratchpad) != 1 {
		t.Fatalf("Scratchpad = %+v", s.Scratchpad)
	}
	got := s.Scratchpad[0].Output
	if !utf8.ValidString(got) {
		t.Error("output bị cắt giữa ký tự multi-byte (phải cắt theo rune)")
	}
	if !strings.Contains(got, "output bị cắt") {
		t.Error("phải có ghi chú tường minh để LLM biết dữ liệu chưa đầy đủ")
	}
	// 1000 rune nội dung + ghi chú; chắc chắn phải ngắn hơn nhiều so với 5000.
	if n := len([]rune(got)); n >= 5000 {
		t.Errorf("output = %d rune, want bị cắt xuống quanh 1000", n)
	}
}

// maxToolOutput = 0 → không giới hạn (giữ hành vi cũ cho caller không cấu hình).
func TestNodeTools_ZeroMaxToolOutputMeansUnlimited(t *testing.T) {
	huge := strings.Repeat("x", 3000)
	eng := &toolsOnlyEngine{
		registry:      regWith(&stubTool{name: "echo", kind: tools.KindRead, output: huge}),
		maxToolOutput: 0,
	}
	s := stateWithToolCalls("echo")

	if _, err := nodeTools(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if s.Scratchpad[0].Output != huge {
		t.Errorf("output dài %d, want giữ nguyên %d khi maxToolOutput=0", len(s.Scratchpad[0].Output), len(huge))
	}
}

// TestNodeTools_BlocksPrivilegedToolForNonOwner khoá RANH GIỚI BẢO MẬT: tool
// đặc quyền (file.*, shell.exec, git) tác động lên MÁY CHẠY AGENT với
// AllowedPaths mặc định gồm $HOME của server, và KHÔNG scope theo tenant. Khi mở
// JARVIS cho nhiều người dùng, để lọt nghĩa là bất kỳ ai cũng đọc được .env
// chứa toàn bộ API key của server.
//
// Đây là lớp chặn THỨ HAI (node_model đã ẩn khỏi tool list). Vẫn cần vì từ step
// 1 trở đi FilterToolDefs trả TOÀN BỘ registry, và model có thể tự bịa tên tool.
func TestNodeTools_BlocksPrivilegedToolForNonOwner(t *testing.T) {
	eng := &toolsOnlyEngine{
		registry: regWith(&stubTool{name: "file.read", kind: tools.KindRead, output: "NỘI DUNG BÍ MẬT CỦA SERVER"}),
		// ownerTenants rỗng → chỉ tenant "default" là chủ (fail closed).
	}
	s := stateWithToolCalls("file.read")
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "nguoi-dung-la")

	var events []Event
	if _, err := nodeTools(ctx, eng, s, func(e Event) { events = append(events, e) }); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}

	// Tool KHÔNG được chạy.
	for _, obs := range s.Scratchpad {
		if strings.Contains(obs.Output, "BÍ MẬT") {
			t.Fatalf("tool đặc quyền đã CHẠY với người dùng thường — rò rỉ dữ liệu server: %+v", obs)
		}
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Error == "" {
		t.Fatalf("phải ghi observation lỗi để LLM biết mà chuyển hướng: %+v", s.Scratchpad)
	}
	end := hasEvent(events, "tool_end")
	if end == nil || end.Message == "" {
		t.Error("phải emit tool_end kèm lý do bị chặn")
	}
}

// Chủ hệ thống thì vẫn chạy được bình thường.
func TestNodeTools_AllowsPrivilegedToolForOwner(t *testing.T) {
	eng := &toolsOnlyEngine{
		registry:     regWith(&stubTool{name: "file.read", kind: tools.KindRead, output: "nội dung file"}),
		ownerTenants: []string{"chu-he-thong"},
	}
	s := stateWithToolCalls("file.read")
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "chu-he-thong")

	if _, err := nodeTools(ctx, eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Output != "nội dung file" {
		t.Errorf("chủ hệ thống phải chạy được tool đặc quyền: %+v", s.Scratchpad)
	}
}

// Chế độ local (không header X-Tenant-ID → tenant "default") vẫn dùng được, để
// không phá trải nghiệm chạy máy cá nhân.
func TestNodeTools_LocalDefaultTenantIsOwner(t *testing.T) {
	eng := &toolsOnlyEngine{
		registry: regWith(&stubTool{name: "shell.exec", kind: tools.KindRead, output: "ok"}),
	}
	s := stateWithToolCalls("shell.exec")

	if _, err := nodeTools(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Output != "ok" {
		t.Errorf("chế độ local phải chạy được tool đặc quyền: %+v", s.Scratchpad)
	}
}

// Tool an toàn vẫn chạy bình thường cho người dùng thường (không chặn oan).
func TestNodeTools_NonOwnerKeepsSafeTools(t *testing.T) {
	eng := &toolsOnlyEngine{
		registry: regWith(
			&stubTool{name: "file.read", kind: tools.KindRead, output: "bí mật"},
			&stubTool{name: "rag.list", kind: tools.KindRead, output: `{"count":3}`},
		),
	}
	s := stateWithToolCalls("file.read", "rag.list")
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "nguoi-dung-la")

	if _, err := nodeTools(ctx, eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}

	var sawRagList, sawSecret bool
	for _, obs := range s.Scratchpad {
		if obs.Name == "rag.list" && strings.Contains(obs.Output, "count") {
			sawRagList = true
		}
		if strings.Contains(obs.Output, "bí mật") {
			sawSecret = true
		}
	}
	if !sawRagList {
		t.Errorf("tool an toàn rag.list bị chặn oan: %+v", s.Scratchpad)
	}
	if sawSecret {
		t.Error("tool đặc quyền vẫn chạy dù người dùng không phải chủ")
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
