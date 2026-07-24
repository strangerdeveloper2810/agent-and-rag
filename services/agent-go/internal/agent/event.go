package agent

import "github.com/ai-agent-tut/agent-go/internal/provider"

// Event là 1 sự kiện phát ra trong lúc engine chạy (→ được transport ghi ra SSE).
// Mỗi event có Type và các trường liên quan; client (UI) dựa vào Type để render.
type Event struct {
	Type        string          `json:"type"`                  // step|text|tool_start|tool_end|error|done|citation|interrupt|usage
	Node        string          `json:"node,omitempty"`        // node hiện tại (khi Type=step)
	Text        string          `json:"text,omitempty"`        // token (khi Type=text)
	Name        string          `json:"name,omitempty"`        // tên tool (tool_start/tool_end)
	Message     string          `json:"message,omitempty"`     // thông báo lỗi (Type=error) hoặc lý do interrupt
	Usage       *provider.Usage `json:"usage,omitempty"`       // per-step usage (Type=usage) hoặc cumulative (Type=done)
	TotalTokens int             `json:"totalTokens,omitempty"` // cumulative total (input+output) across all steps
}

// --- Helpers dựng nhanh event ---

func TextEvent(text string) Event     { return Event{Type: "text", Text: text} }
func StepEvent(node NodeID) Event     { return Event{Type: "step", Node: string(node)} }
func ErrorEvent(msg string) Event     { return Event{Type: "error", Message: msg} }

// DoneEvent tạo event kết thúc với cumulative usage và total tokens.
func DoneEvent(u provider.Usage, totalTokens int) Event {
	return Event{Type: "done", Usage: &u, TotalTokens: totalTokens}
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

// ToolEndEvent phát khi tool chạy xong (ok=true) hoặc lỗi (ok=false, Message=err).
func ToolEndEvent(name string, ok bool, detail string) Event {
	e := Event{Type: "tool_end", Name: name}
	if !ok {
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
