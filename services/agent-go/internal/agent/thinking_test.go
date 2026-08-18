package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
)

// --- ClassifyTask ---

func TestClassifyTask_Complex(t *testing.T) {
	for _, in := range []string{
		"research thị trường", "phân tích rủi ro", "thiết kế kiến trúc",
		"debug lỗi này", "tối ưu query", "tranh chấp hợp đồng",
		"quy định nghị định mới", "security vulnerability CVE-2024-1",
	} {
		if got := ClassifyTask(in, false, 0); got != provider.ThinkingMedium {
			t.Errorf("ClassifyTask(%q) = %q, want MEDIUM", in, got)
		}
	}
}

func TestClassifyTask_Medium(t *testing.T) {
	for _, in := range []string{
		"giải thích cho tôi", "so sánh 2 cái", "tìm file này",
		"viết giúp email", "dịch câu này", "tóm tắt tài liệu",
	} {
		if got := ClassifyTask(in, false, 0); got != provider.ThinkingLow {
			t.Errorf("ClassifyTask(%q) = %q, want LOW", in, got)
		}
	}
}

func TestClassifyTask_SimpleIsOff(t *testing.T) {
	if got := ClassifyTask("chào", false, 0); got != provider.ThinkingOff {
		t.Errorf("ClassifyTask(chào) = %q, want OFF", got)
	}
}

func TestClassifyTask_ContextSignals(t *testing.T) {
	// Đã từng gọi tool → giữ thinking LOW.
	if got := ClassifyTask("ừ", true, 0); got != provider.ThinkingLow {
		t.Errorf("hasToolCalls: got %q, want LOW", got)
	}
	// Nhiều bước → LOW.
	if got := ClassifyTask("ừ", false, 3); got != provider.ThinkingLow {
		t.Errorf("stepCount>2: got %q, want LOW", got)
	}
	// Message dài (>200 ký tự) → LOW.
	long := strings.Repeat("a", 201)
	if got := ClassifyTask(long, false, 0); got != provider.ThinkingLow {
		t.Errorf("message dài: got %q, want LOW", got)
	}
	// Message trung bình (30–200 ký tự), không keyword → OFF.
	mid := strings.Repeat("b", 50)
	if got := ClassifyTask(mid, false, 0); got != provider.ThinkingOff {
		t.Errorf("message trung bình: got %q, want OFF", got)
	}
}

// --- ResolveThinking ---

func TestResolveThinking_Disabled(t *testing.T) {
	cfg := DynamicThinkingConfig{Enabled: false}
	if got := ResolveThinking(cfg, provider.ThinkingHigh, "phân tích", false, 0); got != provider.ThinkingHigh {
		t.Errorf("tắt dynamic phải giữ nguyên static: got %q", got)
	}
}

func TestResolveThinking_DefaultOffUsesClassified(t *testing.T) {
	cfg := DynamicThinkingConfig{Enabled: true, DefaultOff: true}

	if got := ResolveThinking(cfg, provider.ThinkingHigh, "chào", false, 0); got != provider.ThinkingOff {
		t.Errorf("task đơn giản: got %q, want OFF", got)
	}
	if got := ResolveThinking(cfg, provider.ThinkingOff, "phân tích kiến trúc", false, 0); got != provider.ThinkingMedium {
		t.Errorf("task phức tạp: got %q, want MEDIUM", got)
	}
}

// DefaultOff=false: task đơn giản vẫn được nâng lên LOW (không bao giờ OFF).
func TestResolveThinking_FloorsAtLow(t *testing.T) {
	cfg := DynamicThinkingConfig{Enabled: true, DefaultOff: false}

	if got := ResolveThinking(cfg, provider.ThinkingOff, "chào", false, 0); got != provider.ThinkingLow {
		t.Errorf("task đơn giản: got %q, want LOW", got)
	}
	if got := ResolveThinking(cfg, provider.ThinkingOff, "phân tích", false, 0); got != provider.ThinkingMedium {
		t.Errorf("task phức tạp: got %q, want MEDIUM", got)
	}
}

// --- BuildSystemPrompt ---

func TestBuildSystemPrompt_Sections(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil)

	for _, section := range []string{"[DANH TÍNH", "[QUY TẮC]", "[CÔNG CỤ]", "[NGỮ CẢNH]"} {
		if !strings.Contains(prompt, section) {
			t.Errorf("thiếu mục %s", section)
		}
	}
	if !strings.Contains(prompt, "J.A.R.V.I.S.") {
		t.Error("thiếu danh tính JARVIS")
	}
	// Không có skill/memory → không in các mục đó.
	if strings.Contains(prompt, "[KỸ NĂNG]") {
		t.Error("không có skill nhưng vẫn in mục [KỸ NĂNG]")
	}
	if strings.Contains(prompt, "[BỘ NHỚ]") {
		t.Error("không có memory nhưng vẫn in mục [BỘ NHỚ]")
	}
}

func TestBuildSystemPrompt_WithSkillsAndMemories(t *testing.T) {
	prompt := BuildSystemPrompt(
		[]string{"user tên là Trinh", "thích cà phê"},
		[]skills.SkillSummary{{Name: "pdf", Description: "đọc PDF"}},
	)

	// Chỉ tên skill, không description — xem buildSkillCatalogue (skill do code
	// Go chọn nên description trong prompt là token trả không mua được gì).
	if !strings.Contains(prompt, "[KỸ NĂNG]") || !strings.Contains(prompt, "pdf") {
		t.Error("thiếu mục kỹ năng")
	}
	if strings.Contains(prompt, "đọc PDF") {
		t.Error("description của skill vẫn bị gửi trong system prompt")
	}
	if !strings.Contains(prompt, "[BỘ NHỚ]") || !strings.Contains(prompt, "user tên là Trinh") {
		t.Error("thiếu mục bộ nhớ")
	}

	// Thứ tự: phần ổn định (kỹ năng, công cụ) phải nằm TRƯỚC phần động (bộ nhớ)
	// để prompt cache ăn được tiền tố.
	if strings.Index(prompt, "[CÔNG CỤ]") > strings.Index(prompt, "[BỘ NHỚ]") {
		t.Error("[CÔNG CỤ] phải đứng trước [BỘ NHỚ] để tận dụng prompt cache")
	}
}

// --- Event helpers ---

func TestEventHelpers(t *testing.T) {
	if e := TextEvent("x"); e.Type != "text" || e.Text != "x" {
		t.Errorf("TextEvent = %+v", e)
	}
	if e := StepEvent(NodeModel); e.Type != "step" || e.Node != "model" {
		t.Errorf("StepEvent = %+v", e)
	}
	if e := ErrorEvent("toang"); e.Type != "error" || e.Message != "toang" {
		t.Errorf("ErrorEvent = %+v", e)
	}
	if e := ToolStartEvent("echo"); e.Type != "tool_start" || e.Name != "echo" {
		t.Errorf("ToolStartEvent = %+v", e)
	}
	if e := CitationEvent(`[{"title":"a"}]`); e.Type != "citation" || e.Text == "" {
		t.Errorf("CitationEvent = %+v", e)
	}
	if e := InterruptEvent("confirm_destructive", "task.delete"); e.Type != "interrupt" ||
		e.Name != "task.delete" || e.Message != "confirm_destructive" {
		t.Errorf("InterruptEvent = %+v", e)
	}
	if e := MemoryEvent("nhớ rồi"); e.Type != "memory" || e.Message != "nhớ rồi" {
		t.Errorf("MemoryEvent = %+v", e)
	}
	if e := TruncatedEvent(); e.Type != "truncated" || !e.Truncated || e.Message != TruncatedMessage {
		t.Errorf("TruncatedEvent = %+v", e)
	}
}

func TestToolEndEvent(t *testing.T) {
	ok := ToolEndEvent("echo", true, "kết quả")
	if ok.Type != "tool_end" || ok.Text != "kết quả" || ok.Message != "" {
		t.Errorf("ToolEndEvent(ok) = %+v", ok)
	}

	bad := ToolEndEvent("echo", false, "lỗi rồi")
	if bad.Message != "lỗi rồi" || bad.Text != "" {
		t.Errorf("ToolEndEvent(err) = %+v", bad)
	}
}

func TestPlanAndReflectEvents(t *testing.T) {
	p := PlanEvent([]string{"a", "b", "c"})
	if p.Type != "plan" || p.Node != "plan" || !strings.Contains(p.Text, "3") {
		t.Errorf("PlanEvent = %+v", p)
	}

	r := ReflectEvent(2, 5)
	if r.Type != "reflect" || r.Node != "reflect" ||
		!strings.Contains(r.Message, "2") || !strings.Contains(r.Message, "5") {
		t.Errorf("ReflectEvent = %+v", r)
	}
}

func TestUsageEvent(t *testing.T) {
	e := UsageEvent(3, 4, 10, 20)
	if e.Type != "usage" || e.Usage == nil {
		t.Fatalf("UsageEvent = %+v", e)
	}
	if e.Usage.InputTokens != 3 || e.Usage.OutputTokens != 4 {
		t.Errorf("per-step usage = %+v, want {3 4}", e.Usage)
	}
	if e.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", e.TotalTokens)
	}
}

// --- trimContext / estimateTokens ---

func TestEstimateTokens(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: strings.Repeat("a", 100)},
	}
	// (100 nội dung + 4 role) / 4 = 26
	if got := estimateTokens(msgs); got != 26 {
		t.Errorf("estimateTokens = %d, want 26", got)
	}

	if got := estimateTokens(nil); got != 0 {
		t.Errorf("estimateTokens(nil) = %d, want 0", got)
	}
}

func TestTrimContext_NoTrimNeeded(t *testing.T) {
	s := &State{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}}

	if got := trimContext(context.Background(), nil, "", s, 0); got != 0 {
		t.Errorf("maxTokens=0 (không giới hạn) = %d, want 0", got)
	}
	if got := trimContext(context.Background(), nil, "", s, 100000); got != 0 {
		t.Errorf("ít message = %d, want 0", got)
	}
	if len(s.Messages) != 1 {
		t.Errorf("Messages bị đổi: %d", len(s.Messages))
	}
}

// Nhiều hơn keepLast message (vượt check đầu) nhưng nội dung NGẮN nên
// estimateTokens vẫn <= maxTokens → không cần trim, không được gọi LLM.
func TestTrimContext_ManyShortMessagesUnderBudget(t *testing.T) {
	msgs := make([]provider.Message, keepLast+5)
	for i := range msgs {
		msgs[i] = provider.Message{Role: provider.RoleUser, Content: "hi"}
	}
	s := &State{Messages: msgs}

	if got := trimContext(context.Background(), nil, "", s, 100000); got != 0 {
		t.Errorf("got %d, want 0 (dưới budget, không cần trim)", got)
	}
	if len(s.Messages) != keepLast+5 {
		t.Errorf("Messages bị đổi dù không cần trim: %d", len(s.Messages))
	}
}

// Không có provider (nil) → SummarizeMessages luôn thất bại → trimContext
// PHẢI rơi vào fallback TRUNG THỰC (nói rõ đã lược bỏ, không tóm tắt được),
// khác với hành vi cũ luôn chèn placeholder giả vờ "đã được tóm tắt".
func TestTrimContext_DropsOldMessages_HonestFallbackWhenNoProvider(t *testing.T) {
	msgs := make([]provider.Message, 30)
	for i := range msgs {
		msgs[i] = provider.Message{Role: provider.RoleUser, Content: strings.Repeat("x", 400)}
	}
	s := &State{Messages: msgs}

	trimmed := trimContext(context.Background(), nil, "", s, 100)
	if trimmed <= 0 {
		t.Fatalf("trimmed = %d, want > 0", trimmed)
	}
	// Giữ lại placeholder + keepLast message cuối.
	if len(s.Messages) != keepLast+1 {
		t.Errorf("Messages len = %d, want %d", len(s.Messages), keepLast+1)
	}
	if !strings.Contains(s.Messages[0].Content, "không tóm tắt được") {
		t.Errorf("message đầu = %q, want fallback trung thực (không tóm tắt được)", s.Messages[0].Content)
	}
	if !strings.Contains(s.Messages[0].Content, "lược bỏ") {
		t.Errorf("message đầu = %q, want nói rõ đã LƯỢC BỎ (không giả vờ đã tóm tắt)", s.Messages[0].Content)
	}
}

// Có provider trả về tóm tắt thật → trimContext PHẢI dùng đúng nội dung đó,
// không phải placeholder giả.
func TestTrimContext_DropsOldMessages_RealSummaryOnSuccess(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Người dùng tên Linh, đã hỏi về giá sản phẩm X."},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	msgs := make([]provider.Message, 30)
	for i := range msgs {
		msgs[i] = provider.Message{Role: provider.RoleUser, Content: strings.Repeat("x", 400)}
	}
	s := &State{Messages: msgs}

	trimmed := trimContext(context.Background(), fake, "fast-model", s, 100)
	if trimmed <= 0 {
		t.Fatalf("trimmed = %d, want > 0", trimmed)
	}
	if !strings.Contains(s.Messages[0].Content, "Người dùng tên Linh") {
		t.Errorf("message đầu = %q, want chứa tóm tắt thật từ provider", s.Messages[0].Content)
	}
	if strings.Contains(s.Messages[0].Content, "không tóm tắt được") {
		t.Errorf("message đầu = %q, không được lẫn text fallback khi đã tóm tắt thành công", s.Messages[0].Content)
	}
	// Request gửi cho provider phải dùng đúng model rẻ/nhanh được truyền vào.
	if fake.LastRequest.Options.Model != "fast-model" {
		t.Errorf("Options.Model = %q, want fast-model", fake.LastRequest.Options.Model)
	}
}

func TestTrimContext_CountsToolCallChars(t *testing.T) {
	msgs := make([]provider.Message, 20)
	for i := range msgs {
		msgs[i] = provider.Message{
			Role:       provider.RoleAssistant,
			ToolCallID: "call-id",
			ToolCalls: []provider.ToolCall{
				{ID: "c", Name: "echo", Args: []byte(strings.Repeat("y", 200))},
			},
		}
	}
	s := &State{Messages: msgs}

	if trimmed := trimContext(context.Background(), nil, "", s, 10); trimmed <= 0 {
		t.Errorf("trimmed = %d, want > 0 (phải tính cả tool call)", trimmed)
	}
}

// Boundary giữa dropped/kept KHÔNG được rơi ngay sau 1 cặp tool_call/
// tool_result bị chẻ đôi: nếu tin nhắn đầu tiên còn giữ lại là role=tool mồ
// côi (tool_call sinh ra nó đã bị drop), provider sẽ từ chối request. Dựng 30
// message dài với đúng 1 cặp tool_call(assistant)/tool_result(tool) nằm NGAY
// tại ranh giới cắt tự nhiên (index len-keepLast) để buộc SafeDropBoundary
// phải dịch chuyển.
func TestTrimContext_NeverSplitsToolCallPair(t *testing.T) {
	total := 30
	msgs := make([]provider.Message, total)
	for i := range msgs {
		msgs[i] = provider.Message{Role: provider.RoleUser, Content: strings.Repeat("x", 400)}
	}
	naiveCut := total - keepLast
	// Đặt cặp tool_call/tool_result NGAY tại ranh giới tự nhiên: message cuối
	// cùng bị drop (index naiveCut-1) là assistant tool_call, message đầu tiên
	// được giữ (index naiveCut) là tool_result tương ứng — chẻ đôi cặp này nếu
	// không có SafeDropBoundary.
	msgs[naiveCut-1] = provider.Message{
		Role: provider.RoleAssistant,
		ToolCalls: []provider.ToolCall{
			{ID: "call-x", Name: "echo"},
		},
	}
	msgs[naiveCut] = provider.Message{
		Role:       provider.RoleTool,
		ToolCallID: "call-x",
		Content:    "kết quả",
	}
	s := &State{Messages: msgs}

	if trimmed := trimContext(context.Background(), nil, "", s, 100); trimmed <= 0 {
		t.Fatalf("trimmed = %d, want > 0", trimmed)
	}

	// Tin nhắn ĐẦU TIÊN sau placeholder không được là role=tool mồ côi.
	if len(s.Messages) < 2 {
		t.Fatalf("Messages quá ngắn: %d", len(s.Messages))
	}
	if s.Messages[1].Role == provider.RoleTool {
		t.Errorf("Messages[1] = %+v, không được là role=tool mồ côi (chẻ cặp tool_call/tool_result)", s.Messages[1])
	}
}
