package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
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
	ownerTenants       []string
	registry           *tools.Registry
	maxToolOutput      int
	maxTotalToolOutput int
	allowDestructive   bool
}

func (e *toolsOnlyEngine) getRegistry() *tools.Registry   { return e.registry }
func (e *toolsOnlyEngine) getMaxToolOutput() int          { return e.maxToolOutput }
func (e *toolsOnlyEngine) getMaxTotalToolOutput() int     { return e.maxTotalToolOutput }
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

// countingTool đếm số lần Execute() thực sự chạy — dùng để khoá chắc rằng
// dedup KHÔNG chỉ ẩn kết quả mà THỰC SỰ tránh gọi lại tool. An toàn với
// RunParallelStreaming (chạy goroutine) nhờ atomic.
type countingTool struct {
	name   string
	kind   tools.Kind
	output string
	calls  atomic.Int32
}

func (c *countingTool) Name() string            { return c.name }
func (c *countingTool) Description() string     { return "counting " + c.name }
func (c *countingTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (c *countingTool) Kind() tools.Kind        { return c.kind }
func (c *countingTool) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	c.calls.Add(1)
	return tools.Result{Content: c.output}, nil
}

// stateWithSameToolCallArgs dựng State có 1 assistant message với N tool call
// CÙNG tên + CÙNG args (khác stateWithToolCalls — hàm đó tạo args rỗng cho tất
// cả nên vô tình cũng trùng args, nhưng ở đây tường minh hoá cho rõ ý định test).
func stateWithSameToolCallArgs(toolName, args string, count int) *State {
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})
	tcs := make([]provider.ToolCall, count)
	for i := range tcs {
		tcs[i] = provider.ToolCall{ID: fmt.Sprintf("c%d", i), Name: toolName, Args: json.RawMessage(args)}
	}
	s.Messages = append(s.Messages, provider.Message{Role: provider.RoleAssistant, ToolCalls: tcs})
	return s
}

// TestNodeTools_DedupesIdenticalReadCalls khoá RANH GIỚI CHI PHÍ: log dev thật
// cho thấy model tự gọi "notes.search notes.search" (2-3 lần, args giống hệt)
// trong CÙNG một lượt phản hồi — trả tiền + thời gian chạy y hệt nhiều lần một
// cách vô ích, và mỗi bản sao lại lặp lại TOÀN BỘ nội dung kết quả trong context
// gửi cho model ở các step sau.
func TestNodeTools_DedupesIdenticalReadCalls(t *testing.T) {
	ct := &countingTool{name: "notes.search", kind: tools.KindRead, output: "kết quả tìm kiếm dài"}
	eng := &toolsOnlyEngine{registry: regWith(ct)}
	s := stateWithSameToolCallArgs("notes.search", `{"query":"go"}`, 3)

	var events []Event
	next, err := nodeTools(context.Background(), eng, s, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if next != NodeModel {
		t.Errorf("next = %q, want %q", next, NodeModel)
	}

	if got := ct.calls.Load(); got != 1 {
		t.Fatalf("Execute() chạy %d lần, want 1 — dedup phải tránh chạy lại tool trùng lặp", got)
	}

	// Vẫn phải có 3 Observation (1 cho mỗi ToolCall.ID gốc) — protocol yêu cầu
	// mỗi tool_call_id có 1 message role=tool tương ứng.
	if len(s.Scratchpad) != 3 {
		t.Fatalf("Scratchpad = %d, want 3 (mỗi tool_call_id gốc phải có observation riêng)", len(s.Scratchpad))
	}
	if s.Scratchpad[0].Output != "kết quả tìm kiếm dài" {
		t.Errorf("bản đại diện (đầu tiên) phải có output ĐẦY ĐỦ, got %q", s.Scratchpad[0].Output)
	}
	for i, obs := range s.Scratchpad[1:] {
		if obs.Output == "kết quả tìm kiếm dài" {
			t.Errorf("bản sao [%d] không được lặp lại toàn bộ nội dung (lãng phí context token): %q", i+1, obs.Output)
		}
		if !strings.Contains(obs.Output, "Trùng") {
			t.Errorf("bản sao [%d] phải có ghi chú giải thích là trùng lặp: %q", i+1, obs.Output)
		}
	}
	// Mỗi CallID gốc phải khớp đúng thứ tự.
	for i, want := range []string{"c0", "c1", "c2"} {
		if s.Scratchpad[i].CallID != want {
			t.Errorf("Scratchpad[%d].CallID = %q, want %q", i, s.Scratchpad[i].CallID, want)
		}
	}

	if hasEvent(events, "tool_start") == nil {
		t.Error("thiếu event tool_start")
	}
	// Phải có đủ 3 tool_end (đại diện + 2 bản sao), không chỉ 1.
	endCount := 0
	for _, e := range events {
		if e.Type == "tool_end" {
			endCount++
		}
	}
	if endCount != 3 {
		t.Errorf("số event tool_end = %d, want 3 (UI phải thấy đủ 3 tool call, dù chỉ 1 lần thực thi thật)", endCount)
	}
}

// Args KHÁC NHAU thì KHÔNG được dedupe — mỗi call phải thực thi riêng.
func TestNodeTools_DoesNotDedupeDifferentArgs(t *testing.T) {
	ct := &countingTool{name: "notes.search", kind: tools.KindRead, output: "kết quả"}
	eng := &toolsOnlyEngine{registry: regWith(ct)}

	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})
	s.Messages = append(s.Messages, provider.Message{
		Role: provider.RoleAssistant,
		ToolCalls: []provider.ToolCall{
			{ID: "c0", Name: "notes.search", Args: json.RawMessage(`{"query":"go"}`)},
			{ID: "c1", Name: "notes.search", Args: json.RawMessage(`{"query":"python"}`)},
		},
	})

	if _, err := nodeTools(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if got := ct.calls.Load(); got != 2 {
		t.Errorf("Execute() chạy %d lần, want 2 (args khác nhau không được dedupe)", got)
	}
}

// KindWrite KHÔNG bị dedupe dù args giống hệt — bảo toàn hành vi cũ cho tool có
// side-effect (có thể model cố ý gọi lặp lại).
func TestNodeTools_DoesNotDedupeWriteTools(t *testing.T) {
	ct := &countingTool{name: "memory.save", kind: tools.KindWrite, output: "đã lưu"}
	eng := &toolsOnlyEngine{registry: regWith(ct)}
	s := stateWithSameToolCallArgs("memory.save", `{"key":"x","value":"y"}`, 2)

	if _, err := nodeTools(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}
	if got := ct.calls.Load(); got != 2 {
		t.Errorf("Execute() chạy %d lần, want 2 (KindWrite không được dedupe)", got)
	}
}

// TestNodeTools_TotalToolOutputBudgetAccumulatesAcrossCalls khoá ngân sách TỔNG:
// nhiều tool call trong CÙNG 1 lượt gọi nodeTools, MỖI cái riêng lẻ đều dưới
// trần per-call, nhưng CỘNG DỒN vượt ngân sách tổng → phải bị siết lại. Log dev
// thật: 46.542 input token ở step 4 không đến từ 1 tool call vượt trần, mà từ
// nhiều tool call cộng dồn.
func TestNodeTools_TotalToolOutputBudgetAccumulatesAcrossCalls(t *testing.T) {
	big := strings.Repeat("a", 1000)
	eng := &toolsOnlyEngine{
		registry: regWith(
			&stubTool{name: "tool.a", kind: tools.KindRead, output: big},
			&stubTool{name: "tool.b", kind: tools.KindRead, output: big},
			&stubTool{name: "tool.c", kind: tools.KindRead, output: big},
		),
		maxToolOutput:      10000, // per-call cap rộng, không phải nguyên nhân cắt
		maxTotalToolOutput: 1500,  // ngân sách TỔNG chỉ đủ ~1.5 tool call
	}
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})
	s.Messages = append(s.Messages, provider.Message{
		Role: provider.RoleAssistant,
		ToolCalls: []provider.ToolCall{
			{ID: "c0", Name: "tool.a"},
			{ID: "c1", Name: "tool.b"},
			{ID: "c2", Name: "tool.c"},
		},
	})

	if _, err := nodeTools(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeTools: %v", err)
	}

	if len(s.Scratchpad) != 3 {
		t.Fatalf("Scratchpad = %d, want 3", len(s.Scratchpad))
	}
	// Call đầu (a) chưa chạm ngân sách → giữ nguyên đầy đủ.
	if s.Scratchpad[0].Output != big {
		t.Errorf("tool.a (call đầu) phải giữ nguyên đầy đủ khi ngân sách còn nhiều")
	}
	// Các call sau phải bị siết dần — không được giữ nguyên 1000 ký tự đầy đủ.
	for i := 1; i < 3; i++ {
		if s.Scratchpad[i].Output == big {
			t.Errorf("Scratchpad[%d] (%s) phải bị cắt bớt vì ngân sách tổng đã gần/hết, nhưng vẫn giữ nguyên đầy đủ", i, s.Scratchpad[i].Name)
		}
	}
	if s.ToolOutputRunesUsed <= 0 {
		t.Error("State.ToolOutputRunesUsed phải được cộng dồn sau khi chạy tool")
	}
}

// Ngân sách phải SỐNG SÓT qua nhiều lượt gọi nodeTools trên CÙNG một State —
// mô phỏng đúng cơ chế nhiều "step" trong 1 lượt chạy agent thật (log dev: step
// 1→2→3→4, mỗi step gọi thêm tool và ngân sách phải cộng dồn xuyên suốt).
func TestNodeTools_TotalToolOutputBudgetPersistsAcrossMultipleNodeToolsCalls(t *testing.T) {
	big := strings.Repeat("b", 800)
	reg := regWith(&stubTool{name: "tool.x", kind: tools.KindRead, output: big})
	eng := &toolsOnlyEngine{registry: reg, maxTotalToolOutput: 1000}

	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})

	callToolX := func(id string) {
		s.Messages = append(s.Messages, provider.Message{
			Role:      provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{ID: id, Name: "tool.x"}},
		})
		if _, err := nodeTools(context.Background(), eng, s, func(Event) {}); err != nil {
			t.Fatalf("nodeTools: %v", err)
		}
	}

	callToolX("step1") // 800 ký tự, ngân sách còn 200
	usedAfterStep1 := s.ToolOutputRunesUsed
	if usedAfterStep1 == 0 {
		t.Fatal("ngân sách phải được cộng dồn sau step 1")
	}

	callToolX("step2") // ngân sách gần hết → phải bị siết mạnh hơn nhiều so với step 1
	last := s.Scratchpad[len(s.Scratchpad)-1]
	if last.Output == big {
		t.Error("step 2 phải bị cắt vì ngân sách tổng đã gần cạn từ step 1, nhưng vẫn giữ nguyên đầy đủ")
	}
	if s.ToolOutputRunesUsed < usedAfterStep1 {
		t.Error("ToolOutputRunesUsed phải tăng thêm (hoặc giữ nguyên), không được giảm giữa các step")
	}
}

// truncateRunes(s, maxRunes<=0) chưa được test trực tiếp — mọi call site hiện
// tại (toolResultPreview, destructiveBlockedMessage) luôn truyền hằng số dương,
// nên nhánh "không giới hạn" của bản thân hàm chưa từng chạy trong test suite.
func TestTruncateRunes_ZeroOrNegativeMeansNoLimit(t *testing.T) {
	long := strings.Repeat("x", 1000)
	for _, maxRunes := range []int{0, -1, -100} {
		if got := truncateRunes(long, maxRunes); got != long {
			t.Errorf("truncateRunes(_, %d) phải trả nguyên văn (không giới hạn), got độ dài %d", maxRunes, len(got))
		}
	}
}

func TestDestructiveBlockedMessage_MultipleCallsPluralizes(t *testing.T) {
	msg := destructiveBlockedMessage([]provider.ToolCall{
		{Name: "shell.exec"}, {Name: "shell.exec"},
	})
	if !strings.Contains(msg, "2 công cụ") {
		t.Errorf("2 call phải dùng câu số nhiều '2 công cụ': %q", msg)
	}
}

// Call KHÔNG có args (Args rỗng/"{}"): không được hiện "với tham số: “" trống
// rỗng gây rối mắt.
func TestDestructiveBlockedMessage_OmitsEmptyArgs(t *testing.T) {
	msg := destructiveBlockedMessage([]provider.ToolCall{{Name: "shell.exec", Args: nil}})
	if strings.Contains(msg, "với tham số") {
		t.Errorf("call không có args không được hiện 'với tham số': %q", msg)
	}

	msgEmptyObj := destructiveBlockedMessage([]provider.ToolCall{{Name: "shell.exec", Args: json.RawMessage(`{}`)}})
	if strings.Contains(msgEmptyObj, "với tham số") {
		t.Errorf("args=\"{}\" không được hiện 'với tham số': %q", msgEmptyObj)
	}
}

// Call CÓ args thật: phải hiện "với tham số" kèm nội dung.
func TestDestructiveBlockedMessage_ShowsNonEmptyArgs(t *testing.T) {
	msg := destructiveBlockedMessage([]provider.ToolCall{
		{Name: "shell.exec", Args: json.RawMessage(`{"cmd":"rm -rf /tmp/x"}`)},
	})
	if !strings.Contains(msg, "với tham số") || !strings.Contains(msg, "rm -rf") {
		t.Errorf("args thật phải hiện đầy đủ trong thông báo: %q", msg)
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
