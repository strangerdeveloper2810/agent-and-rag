package agent

import "github.com/ai-agent-tut/agent-go/internal/provider"

// Event là 1 sự kiện phát ra trong lúc engine chạy (→ được transport ghi ra SSE).
type Event struct {
	Type    string          `json:"type"`              // step|text|tool_start|tool_end|error|done|citation|interrupt
	Node    string          `json:"node,omitempty"`    // node hiện tại (khi Type=step)
	Text    string          `json:"text,omitempty"`    // token (khi Type=text)
	Name    string          `json:"name,omitempty"`    // tên tool (tool_start/tool_end)
	Message string          `json:"message,omitempty"` // thông báo lỗi (Type=error)
	Usage   *provider.Usage `json:"usage,omitempty"`   // khi Type=done
}

// EmitFunc là callback engine dùng để phát Event ra ngoài.
type EmitFunc func(Event)

// Helper dựng nhanh vài event thường dùng.
func TextEvent(text string) Event      { return Event{Type: "text", Text: text} }
func StepEvent(node NodeID) Event      { return Event{Type: "step", Node: string(node)} }
func ErrorEvent(msg string) Event      { return Event{Type: "error", Message: msg} }
func DoneEvent(u provider.Usage) Event { return Event{Type: "done", Usage: &u} }
