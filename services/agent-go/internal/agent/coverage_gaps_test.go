package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// EnablePlanning gắn node plan/reflect vào engine. Nó TẮT mặc định vì tốn thêm
// một LLM call trước token đầu tiên, nên test khoá lại đúng hai điều: bật thì
// plan thật sự chạy, và không bật thì không tốn call nào.
func TestEnablePlanning_BatThiNodePlanChay(t *testing.T) {
	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{
		// Lượt 1 = node plan: trả JSON array các bước.
		{
			{Kind: provider.ChunkText, Text: `["buoc 1","buoc 2"]`},
			{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 100, OutputTokens: 10}},
			{Kind: provider.ChunkDone},
		},
		// Lượt 2 = node model: trả lời cho user.
		{
			{Kind: provider.ChunkText, Text: "xong"},
			{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 200, OutputTokens: 5}},
			{Kind: provider.ChunkDone},
		},
	}}

	eng := NewEngine(prov, tools.NewRegistry())
	eng.EnablePlanning()

	var planEvents int
	emit := func(e Event) {
		if e.Type == "plan" {
			planEvents++
		}
	}

	// Câu phải đủ "phức tạp" để isComplexRequest bật node plan.
	_, err := eng.Run(context.Background(), RunInput{
		// LƯU Ý: isComplexRequest chỉ khớp từ khoá TIẾNG ANH (complexKeywords:
		// plan/roadmap/steps/...), nên câu tiếng Việt tương đương KHÔNG kích hoạt
		// node plan. Dùng "roadmap" ở đây để test đúng nhánh có plan.
		UserMessage: "give me a roadmap for the payment system",
		MaxSteps:    5,
	}, emit)
	if err != nil {
		t.Fatalf("engine.Run lỗi: %v", err)
	}

	if planEvents == 0 {
		t.Error("EnablePlanning() rồi mà node plan không chạy (không có event plan)")
	}
	if prov.calls < 2 {
		t.Errorf("số lượt gọi provider = %d, want >= 2 (plan + model)", prov.calls)
	}
}

func TestEnablePlanning_KhongBatThiKhongTonThemCall(t *testing.T) {
	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{{
		{Kind: provider.ChunkText, Text: "xong"},
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 200, OutputTokens: 5}},
		{Kind: provider.ChunkDone},
	}}}

	eng := NewEngine(prov, tools.NewRegistry())
	// KHÔNG gọi EnablePlanning.

	if _, err := eng.Run(context.Background(), RunInput{
		// LƯU Ý: isComplexRequest chỉ khớp từ khoá TIẾNG ANH (complexKeywords:
		// plan/roadmap/steps/...), nên câu tiếng Việt tương đương KHÔNG kích hoạt
		// node plan. Dùng "roadmap" ở đây để test đúng nhánh có plan.
		UserMessage: "give me a roadmap for the payment system",
		MaxSteps:    5,
	}, func(Event) {}); err != nil {
		t.Fatalf("engine.Run lỗi: %v", err)
	}

	if prov.calls != 1 {
		t.Errorf("số lượt gọi provider = %d, want 1 — planning tắt thì không được tốn call thêm", prov.calls)
	}
}

func TestLastUserContent_KhongCoTinNhanUser(t *testing.T) {
	s := &State{Messages: []provider.Message{
		{Role: provider.RoleSystem, Content: "system"},
		{Role: provider.RoleAssistant, Content: "chào bạn"},
	}}
	if got := s.LastUserContent(); got != "" {
		t.Errorf("LastUserContent() = %q, want rỗng", got)
	}
}

func TestLastUserContent_LayTinNhanUserGanNhat(t *testing.T) {
	s := &State{Messages: []provider.Message{
		{Role: provider.RoleUser, Content: "câu cũ"},
		{Role: provider.RoleAssistant, Content: "trả lời"},
		{Role: provider.RoleUser, Content: "câu mới"},
	}}
	if got := s.LastUserContent(); got != "câu mới" {
		t.Errorf("LastUserContent() = %q, want %q", got, "câu mới")
	}
}

// canonicalToolCallKey chuẩn hoá args để hai tool call giống nhau về NGHĨA (khác
// thứ tự field, khác khoảng trắng) được coi là một — nhờ đó circuit breaker phát
// hiện lặp và tool không bị chạy hai lần.
func TestCanonicalToolCallKey(t *testing.T) {
	a := canonicalToolCallKey("echo", json.RawMessage(`{"text":"hi","n":1}`))
	b := canonicalToolCallKey("echo", json.RawMessage(`{ "n": 1, "text": "hi" }`))
	if a != b {
		t.Errorf("cùng args khác thứ tự/khoảng trắng phải cho cùng khoá:\n a = %s\n b = %s", a, b)
	}

	// Args KHÔNG phải JSON hợp lệ: vẫn phải trả khoá dùng được, không panic.
	broken := canonicalToolCallKey("echo", json.RawMessage(`{not json`))
	if !strings.HasPrefix(broken, "echo|") {
		t.Errorf("args lỗi phải fallback về name|raw, got %q", broken)
	}

	// Args khác nhau thật thì khoá phải khác.
	if canonicalToolCallKey("echo", json.RawMessage(`{"text":"a"}`)) ==
		canonicalToolCallKey("echo", json.RawMessage(`{"text":"b"}`)) {
		t.Error("args khác nhau mà cho cùng khoá — dedup sẽ ăn oan tool call hợp lệ")
	}

	// Cùng args nhưng khác tool thì khoá phải khác.
	if canonicalToolCallKey("echo", json.RawMessage(`{"x":1}`)) ==
		canonicalToolCallKey("other", json.RawMessage(`{"x":1}`)) {
		t.Error("hai tool khác nhau cho cùng khoá")
	}
}
