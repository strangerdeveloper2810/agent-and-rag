package agent

import (
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Event là 1 sự kiện phát ra trong lúc engine chạy (→ được transport ghi ra SSE).
// Mỗi event có Type và các trường liên quan; client (UI) dựa vào Type để render.
type Event struct {
	Type        string          `json:"type"`                  // step|text|tool_start|tool_end|error|done|citation|interrupt|usage|truncated
	Node        string          `json:"node,omitempty"`        // node hiện tại (khi Type=step)
	Text        string          `json:"text,omitempty"`        // token (khi Type=text)
	Name        string          `json:"name,omitempty"`        // tên tool (tool_start/tool_end)
	Message     string          `json:"message,omitempty"`     // thông báo lỗi (Type=error) hoặc lý do interrupt
	Usage       *provider.Usage `json:"usage,omitempty"`       // per-step usage (Type=usage) hoặc cumulative (Type=done)
	TotalTokens int             `json:"totalTokens,omitempty"` // cumulative total (input+output) across all steps
	Truncated   bool            `json:"truncated,omitempty"`   // true khi Type=truncated hoặc Type=done của lượt bị cắt

	// AskUser & Suggestions
	Questions   []ClarifyQuestion `json:"questions,omitempty"`   // Danh sách câu hỏi làm rõ (Type=ask_user)
	Suggestions []string          `json:"suggestions,omitempty"` // Gợi ý câu hỏi tiếp theo (Type=suggestions)

	// ContextTokens/ContextBudget (Type=done): kích thước ước tính (token) của
	// s.Messages ở CUỐI lượt chạy — đây chính là kích thước history mà client
	// sẽ gửi lại ở lượt chat kế tiếp. FE dùng tỉ lệ ContextTokens/ContextBudget
	// để tự quyết định khi nào gợi ý user bắt đầu chat mới (Tier 4), thay vì Go
	// hardcode 1 ngưỡng cố định — ContextBudget=0 nghĩa là không giới hạn
	// (MAX_CONTEXT_TOKENS chưa cấu hình), FE nên bỏ qua gợi ý trong trường hợp đó.
	ContextTokens int `json:"contextTokens,omitempty"`
	ContextBudget int `json:"contextBudget,omitempty"`
}

// ClarifyOption đại diện cho một phương án lựa chọn trong câu hỏi làm rõ.
type ClarifyOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

// ClarifyQuestion đại diện cho một câu hỏi làm rõ (tương tự agent-toolkit ask_user).
type ClarifyQuestion struct {
	ID          string          `json:"id,omitempty"`
	Prompt      string          `json:"prompt"`
	Header      string          `json:"header,omitempty"`
	Options     []ClarifyOption `json:"options,omitempty"`
	MultiSelect bool            `json:"multiSelect,omitempty"`
}

// --- Helpers dựng nhanh event ---

func TextEvent(text string) Event { return Event{Type: "text", Text: text} }
func StepEvent(node NodeID) Event { return Event{Type: "step", Node: string(node)} }
func ErrorEvent(msg string) Event { return Event{Type: "error", Message: msg} }

// AskUserEvent phát khi Agent đặt 1-4 câu hỏi làm rõ / lựa chọn phương án cho user.
func AskUserEvent(questions []ClarifyQuestion) Event {
	return Event{Type: "ask_user", Questions: questions}
}

// SuggestionsEvent phát danh sách gợi ý câu hỏi tiếp theo (follow-up chips).
func SuggestionsEvent(suggestions []string) Event {
	return Event{Type: "suggestions", Suggestions: suggestions}
}

// DoneEvent tạo event kết thúc với cumulative usage và total tokens.
// truncated=true khi câu trả lời cuối bị cắt vì chạm giới hạn output token.
func DoneEvent(u provider.Usage, totalTokens int, truncated bool) Event {
	return Event{Type: "done", Usage: &u, TotalTokens: totalTokens, Truncated: truncated}
}

// TruncatedMessage là thông báo mặc định khi câu trả lời bị cắt giữa chừng.
const TruncatedMessage = "Câu trả lời bị cắt do chạm giới hạn độ dài tối đa."

// TruncatedEvent phát ngay khi model dừng vì chạm max output tokens — UI dựa
// vào event này để hiện chỉ báo + nút "Tiếp tục".
func TruncatedEvent() Event {
	return Event{Type: "truncated", Message: TruncatedMessage, Truncated: true}
}

// UsageEvent tạo event thông báo token usage của một model call.
func UsageEvent(stepIn, stepOut, totalIn, totalOut int) Event {
	return Event{
		Type:        "usage",
		Usage:       &provider.Usage{InputTokens: stepIn, OutputTokens: stepOut},
		TotalTokens: totalIn + totalOut,
	}
}

// ToolStartEvent phát khi bắt đầu chạy 1 tool.
func ToolStartEvent(name string) Event {
	return Event{Type: "tool_start", Name: name}
}

// ToolEndEvent phát khi tool chạy xong. ok=true → Text là preview kết quả;
// ok=false → Message là lỗi.
func ToolEndEvent(name string, ok bool, detail string) Event {
	e := Event{Type: "tool_end", Name: name}
	if ok {
		e.Text = detail
	} else {
		e.Message = detail
	}
	return e
}

// CitationEvent phát danh sách nguồn tham khảo (RAG). sources là JSON string.
func CitationEvent(sources string) Event {
	return Event{Type: "citation", Text: sources}
}

// InterruptEvent phát khi engine dừng chờ HITL.
func InterruptEvent(reason, tool string) Event {
	return Event{Type: "interrupt", Name: tool, Message: reason}
}

// MemoryEvent phát khi node memory (recall/extract/summarize) thực hiện thao tác.
func MemoryEvent(detail string) Event {
	return Event{Type: "memory", Message: detail}
}

// PlanEvent phát khi node plan tạo ra kế hoạch các bước.
func PlanEvent(steps []string) Event {
	return Event{Type: "plan", Text: fmt.Sprintf("plan: %d steps", len(steps)), Node: "plan"}
}

// ReflectEvent phát khi node reflect đánh giá tiến độ plan.
func ReflectEvent(step, total int) Event {
	return Event{Type: "reflect", Message: fmt.Sprintf("plan step %d of %d", step, total), Node: "reflect"}
}
