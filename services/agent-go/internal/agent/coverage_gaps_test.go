package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/observability"
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

// TestLangSmithTracingAndCustomSkills_Coverage covers the branches in Engine.Run, node_model, and node_tools
// when LangSmith tracing, Persona presets, and CustomSkills are supplied.
func TestLangSmithTracingAndCustomSkills_Coverage(t *testing.T) {
	// Setup test mock LangSmith client
	cfg := config.Config{
		LangSmithTracing: true,
		LangSmithAPIKey:  "test-key",
		LangSmithProject: "test-proj",
	}
	observability.InitLangSmith(cfg)
	defer func() {
		observability.InitLangSmith(config.Config{})
	}()

	reg := tools.NewRegistry()
	reg.Register(&stubTool{name: "echo", output: "echo output"})

	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{
		// Step 1: Model calls tool
		{
			{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "call-1", Name: "echo", Args: json.RawMessage(`{"text":"hello"}`)}},
			{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 50, OutputTokens: 10}},
			{Kind: provider.ChunkDone},
		},
		// Step 2: Model finishes with answer
		{
			{Kind: provider.ChunkText, Text: "done!"},
			{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 70, OutputTokens: 15}},
			{Kind: provider.ChunkDone},
		},
	}}

	eng := NewEngine(prov, reg)

	in := RunInput{
		ConversationID:     "conv-123",
		UserMessage:        "test question",
		Lang:               "en",
		PersonaPreset:      "coder",
		Formality:          "formal",
		Verbosity:          "concise",
		CustomInstructions: "always write clean code",
		CustomSkills: []CustomSkill{
			{
				Name:        "golang-expert",
				Description: "expert in Go",
				WhenToUse:   "when writing Go",
				Content:     "use standard library where possible",
			},
		},
		MaxSteps: 5,
	}

	_, err := eng.Run(context.Background(), in, func(Event) {})
	if err != nil {
		t.Fatalf("eng.Run failed: %v", err)
	}
}

func TestNodeModel_ChunkError_WithLangSmith(t *testing.T) {
	cfg := config.Config{
		LangSmithTracing: true,
		LangSmithAPIKey:  "test-key",
		LangSmithProject: "test-proj",
	}
	observability.InitLangSmith(cfg)
	defer func() {
		observability.InitLangSmith(config.Config{})
	}()

	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{
		{
			{Kind: provider.ChunkError, Err: context.Canceled},
		},
	}}

	eng := NewEngine(prov, tools.NewRegistry())
	_, err := eng.Run(context.Background(), RunInput{UserMessage: "hello"}, func(Event) {})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestNodeTools_AskUserEvent(t *testing.T) {
	askPayload := `{"questions":[{"prompt":"Target OS?","header":"OS","options":[{"label":"Linux","recommended":true},{"label":"Windows"}],"multi_select":false}]}`

	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{
		{
			{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID:   "call_ask_1",
				Name: "ask_user",
				Args: json.RawMessage(askPayload),
			}},
			{Kind: provider.ChunkDone},
		},
		{
			{Kind: provider.ChunkText, Text: "Đã gửi câu hỏi"},
			{Kind: provider.ChunkDone},
		},
	}}

	reg := tools.NewRegistry()
	reg.Register(tools.NewAskUserTool())

	var askUserEvents []Event
	var suggestionsEvents []Event
	emit := func(e Event) {
		if e.Type == "ask_user" {
			askUserEvents = append(askUserEvents, e)
		}
		if e.Type == "suggestions" {
			suggestionsEvents = append(suggestionsEvents, e)
		}
	}

	// Test SuggestionsEvent helper
	emit(SuggestionsEvent([]string{"suggestion 1", "suggestion 2"}))

	eng := NewEngine(prov, reg)
	_, err := eng.Run(context.Background(), RunInput{UserMessage: "brainstorm app"}, emit)
	if err != nil {
		t.Fatalf("eng.Run failed: %v", err)
	}

	if len(askUserEvents) == 0 {
		t.Fatal("expected at least 1 ask_user event")
	}
	if len(askUserEvents[0].Questions) != 1 {
		t.Errorf("expected 1 question, got %d", len(askUserEvents[0].Questions))
	}
	if askUserEvents[0].Questions[0].Prompt != "Target OS?" {
		t.Errorf("prompt = %q, want 'Target OS?'", askUserEvents[0].Questions[0].Prompt)
	}
	if len(suggestionsEvents) != 1 {
		t.Errorf("expected 1 suggestions event, got %d", len(suggestionsEvents))
	}
}
