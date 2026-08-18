package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// --- canonicalToolCallKey ---

func TestCanonicalToolCallKey_SameArgsSameKey(t *testing.T) {
	a := canonicalToolCallKey("notes.search", json.RawMessage(`{"query":"go"}`))
	b := canonicalToolCallKey("notes.search", json.RawMessage(`{"query":"go"}`))
	if a != b {
		t.Errorf("2 call giống hệt phải ra cùng khoá: %q vs %q", a, b)
	}
}

// Model không đảm bảo thứ tự key JSON ổn định giữa 2 lần sinh — 2 args tương
// đương nhưng khác thứ tự key vẫn phải được coi là TRÙNG LẶP.
func TestCanonicalToolCallKey_DifferentKeyOrderSameKey(t *testing.T) {
	a := canonicalToolCallKey("rag.search", json.RawMessage(`{"query":"go","limit":5}`))
	b := canonicalToolCallKey("rag.search", json.RawMessage(`{"limit":5,"query":"go"}`))
	if a != b {
		t.Errorf("thứ tự key khác nhau nhưng args tương đương phải ra cùng khoá: %q vs %q", a, b)
	}
}

func TestCanonicalToolCallKey_DifferentArgsDifferentKey(t *testing.T) {
	a := canonicalToolCallKey("notes.search", json.RawMessage(`{"query":"go"}`))
	b := canonicalToolCallKey("notes.search", json.RawMessage(`{"query":"python"}`))
	if a == b {
		t.Error("args khác nhau không được ra cùng khoá")
	}
}

func TestCanonicalToolCallKey_DifferentToolNameDifferentKey(t *testing.T) {
	a := canonicalToolCallKey("notes.search", json.RawMessage(`{"query":"go"}`))
	b := canonicalToolCallKey("rag.search", json.RawMessage(`{"query":"go"}`))
	if a == b {
		t.Error("tên tool khác nhau không được ra cùng khoá dù args giống hệt")
	}
}

// JSON hỏng không được crash — fallback về chuỗi gốc.
func TestCanonicalToolCallKey_InvalidJSONFallsBackGracefully(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("canonicalToolCallKey panic với JSON hỏng: %v", r)
		}
	}()
	a := canonicalToolCallKey("x", json.RawMessage(`{bad json`))
	b := canonicalToolCallKey("x", json.RawMessage(`{bad json`))
	if a != b {
		t.Error("cùng 1 chuỗi JSON hỏng phải fallback ra cùng khoá (deterministic)")
	}
}

// --- dedupeSafeCalls ---

func regForDedup() *tools.Registry {
	return regWith(
		&stubTool{name: "notes.search", kind: tools.KindRead, output: "kết quả"},
		&stubTool{name: "rag.search", kind: tools.KindRead, output: "khác"},
		&stubTool{name: "memory.save", kind: tools.KindWrite, output: "đã lưu"},
		&stubTool{name: "task.delete", kind: tools.KindDestructive, output: "đã xoá"},
	)
}

func tc(id, name, args string) provider.ToolCall {
	return provider.ToolCall{ID: id, Name: name, Args: json.RawMessage(args)}
}

func TestDedupeSafeCalls_GroupsIdenticalReadCalls(t *testing.T) {
	reg := regForDedup()
	calls := []provider.ToolCall{
		tc("c1", "notes.search", `{"query":"go"}`),
		tc("c2", "notes.search", `{"query":"go"}`),
		tc("c3", "notes.search", `{"query":"go"}`),
	}

	dedup := dedupeSafeCalls(reg, calls)

	if len(dedup.exec) != 1 {
		t.Fatalf("exec = %d, want 1 (3 call giống hệt chỉ thực thi 1 lần)", len(dedup.exec))
	}
	if dedup.exec[0].ID != "c1" {
		t.Errorf("đại diện = %q, want c1 (call đầu tiên của nhóm)", dedup.exec[0].ID)
	}
	if len(dedup.order) != 1 {
		t.Fatalf("order = %d, want 1 nhóm", len(dedup.order))
	}
	members := dedup.groups[dedup.order[0]]
	if len(members) != 3 {
		t.Fatalf("nhóm phải chứa cả 3 call gốc, got %d", len(members))
	}
	for i, want := range []string{"c1", "c2", "c3"} {
		if members[i].ID != want {
			t.Errorf("members[%d].ID = %q, want %q (phải giữ đúng thứ tự xuất hiện)", i, members[i].ID, want)
		}
	}
}

func TestDedupeSafeCalls_DifferentArgsNotGrouped(t *testing.T) {
	reg := regForDedup()
	calls := []provider.ToolCall{
		tc("c1", "notes.search", `{"query":"go"}`),
		tc("c2", "notes.search", `{"query":"python"}`),
	}

	dedup := dedupeSafeCalls(reg, calls)

	if len(dedup.exec) != 2 {
		t.Errorf("exec = %d, want 2 (args khác nhau, không dedupe)", len(dedup.exec))
	}
}

// KindWrite/KindDestructive KHÔNG bao giờ bị dedupe — dù args giống hệt, có thể
// là hành động cố ý lặp lại (side-effect).
func TestDedupeSafeCalls_WriteAndDestructiveNeverDeduped(t *testing.T) {
	reg := regForDedup()

	tests := []struct {
		name string
		tool string
	}{
		{"KindWrite", "memory.save"},
		{"KindDestructive", "task.delete"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := []provider.ToolCall{
				tc("c1", tt.tool, `{"x":1}`),
				tc("c2", tt.tool, `{"x":1}`),
			}
			dedup := dedupeSafeCalls(reg, calls)
			if len(dedup.exec) != 2 {
				t.Errorf("%s: exec = %d, want 2 (không được dedupe)", tt.tool, len(dedup.exec))
			}
		})
	}
}

// Tool không có trong registry (sẽ bị báo lỗi "not found" ở tầng Execute) vẫn
// phải qua được dedupeSafeCalls an toàn — không tra được Kind thì coi như
// không-dedupable (mỗi call 1 nhóm riêng theo ID).
func TestDedupeSafeCalls_UnknownToolNotDeduped(t *testing.T) {
	reg := regForDedup()
	calls := []provider.ToolCall{
		tc("c1", "khong-ton-tai", `{}`),
		tc("c2", "khong-ton-tai", `{}`),
	}
	dedup := dedupeSafeCalls(reg, calls)
	if len(dedup.exec) != 2 {
		t.Errorf("exec = %d, want 2 (tool lạ không dedupe)", len(dedup.exec))
	}
}

func TestDedupeSafeCalls_MixedBatch(t *testing.T) {
	reg := regForDedup()
	calls := []provider.ToolCall{
		tc("c1", "notes.search", `{"query":"go"}`),
		tc("c2", "rag.search", `{"query":"go"}`), // khác tool, cùng args
		tc("c3", "notes.search", `{"query":"go"}`),
		tc("c4", "memory.save", `{"key":"x"}`),
	}
	dedup := dedupeSafeCalls(reg, calls)
	if len(dedup.exec) != 3 {
		t.Fatalf("exec = %d, want 3 (notes.search dedupe được, rag.search + memory.save riêng)", len(dedup.exec))
	}
}

func TestDedupeSafeCalls_EmptyInput(t *testing.T) {
	dedup := dedupeSafeCalls(regForDedup(), nil)
	if len(dedup.exec) != 0 || len(dedup.order) != 0 {
		t.Errorf("input rỗng phải ra kết quả rỗng, got exec=%d order=%d", len(dedup.exec), len(dedup.order))
	}
}

// --- duplicateToolResultNote ---

func TestDuplicateToolResultNote_MentionsToolName(t *testing.T) {
	note := duplicateToolResultNote("notes.search")
	if !strings.Contains(note, "notes.search") {
		t.Errorf("ghi chú phải nhắc tên tool: %q", note)
	}
	if len(note) > 300 {
		t.Errorf("ghi chú phải NGẮN (mục đích tiết kiệm token), got %d ký tự", len(note))
	}
}

// --- applyToolOutputBudget ---

func TestApplyToolOutputBudget_NoTotalBudgetOnlyPerCall(t *testing.T) {
	out, used, truncated := applyToolOutputBudget(strings.Repeat("a", 100), 50, 0, 0)
	if truncated {
		t.Error("totalBudget=0 nghĩa là không giới hạn tổng — không được báo truncated vì lý do ngân sách")
	}
	if len([]rune(out)) <= 50 {
		t.Error("vẫn phải áp perCallMax dù không có ngân sách tổng")
	}
	if used != len([]rune(out)) {
		t.Errorf("used = %d, want khớp độ dài output thật = %d", used, len([]rune(out)))
	}
}

func TestApplyToolOutputBudget_UnderBudgetPassesThrough(t *testing.T) {
	out, used, truncated := applyToolOutputBudget("nội dung ngắn", 1000, 5000, 0)
	if truncated {
		t.Error("dưới ngân sách không được báo truncated")
	}
	if out != "nội dung ngắn" {
		t.Errorf("output = %q, want giữ nguyên", out)
	}
	if used != len([]rune("nội dung ngắn")) {
		t.Errorf("used = %d, want %d", used, len([]rune("nội dung ngắn")))
	}
}

func TestApplyToolOutputBudget_ExceedsRemainingGetsCutToFit(t *testing.T) {
	content := strings.Repeat("x", 5000)                                       // đủ lớn để phần ghi chú giải thích không lấn át phép so sánh
	out, used, truncated := applyToolOutputBudget(content, 100000, 5000, 4000) // còn 1000 ký tự ngân sách
	if !truncated {
		t.Fatal("vượt ngân sách còn lại phải báo truncated")
	}
	// Nội dung thật giữ lại phải khớp đúng phần "remaining" (1000), phần dư ra
	// trong `used` chỉ là ghi chú giải thích ngắn — used phải NHỎ HƠN content gốc
	// (5000), nếu không thì việc cắt theo ngân sách vô nghĩa.
	if used >= len(content) {
		t.Errorf("used = %d, want < %d (content gốc) — nếu không việc cắt theo ngân sách không tiết kiệm được gì", used, len(content))
	}
	if !strings.Contains(out, "ngân sách") {
		t.Errorf("output phải có ghi chú giải thích lý do cắt: %q", out)
	}
}

func TestApplyToolOutputBudget_BudgetAlreadyExhausted(t *testing.T) {
	out, used, truncated := applyToolOutputBudget("nội dung bất kỳ dài bao nhiêu cũng vậy", 1000, 100, 100)
	if !truncated {
		t.Fatal("ngân sách đã cạn từ trước phải báo truncated")
	}
	if strings.Contains(out, "bất kỳ") {
		t.Error("khi ngân sách đã cạn, KHÔNG được đưa bất kỳ phần nội dung thật nào vào — chỉ ghi chú")
	}
	if used <= 0 {
		t.Error("used phải > 0 (độ dài của chính ghi chú)")
	}
}

func TestApplyToolOutputBudget_BudgetExceededBeyondTotal(t *testing.T) {
	// alreadyUsed > totalBudget (có thể xảy ra do làm tròn/nhiều tool cộng dồn) —
	// không được panic hay tính remaining âm gây lỗi cắt chuỗi.
	out, used, truncated := applyToolOutputBudget("nội dung", 1000, 100, 150)
	if !truncated {
		t.Error("alreadyUsed vượt cả totalBudget vẫn phải coi là hết ngân sách")
	}
	if used <= 0 {
		t.Errorf("used = %d, want > 0", used)
	}
	_ = out // chỉ cần không panic
}

// perCallMax và totalBudget tương tác đúng: perCallMax cắt trước, rồi mới áp
// tiếp totalBudget lên phần ĐÃ CẮT (không phải bản gốc).
func TestApplyToolOutputBudget_PerCallCapAppliesBeforeTotalBudget(t *testing.T) {
	content := strings.Repeat("y", 1000)
	out, used, _ := applyToolOutputBudget(content, 50, 100000, 0) // perCallMax=50 << totalBudget
	if len([]rune(out)) > 50+200 {                                // 50 + dư cho ghi chú capToolOutput
		t.Errorf("perCallMax phải được áp trước, output quá dài: %d ký tự", len([]rune(out)))
	}
	if used > 50+200 {
		t.Errorf("used = %d, want tương ứng với output đã cắt theo perCallMax", used)
	}
}

func TestApplyToolOutputBudget_UnicodeSafe(t *testing.T) {
	// Chuỗi tiếng Việt nhiều ký tự multi-byte — không được cắt giữa ký tự.
	content := strings.Repeat("Xin chào các bạn, đây là nội dung tiếng Việt có dấu ", 50)
	out, _, truncated := applyToolOutputBudget(content, 100000, 200, 0)
	if !truncated {
		t.Fatal("content dài hơn ngân sách 200 rune phải bị cắt")
	}
	if !utf8.ValidString(out) {
		t.Errorf("output bị cắt giữa ký tự multi-byte, không còn là UTF-8 hợp lệ: %q", out)
	}
}
