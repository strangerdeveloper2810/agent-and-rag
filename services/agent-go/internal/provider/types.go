// Package provider định nghĩa lớp trừu tượng LLM provider (provider-agnostic).
// Adapter cho từng nhà cung cấp (Gemini, Anthropic) nằm ở sub-package.
package provider

import "encoding/json"

// Role của một message trong hội thoại.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Attachment represents a file or image attached to a user message.
// Type is "image" (Data is base64-encoded bytes) or "file" (Data is plain text).
type Attachment struct {
	Type     string `json:"type"`     // "image" or "file"
	Name     string `json:"name"`     // filename
	Data     string `json:"data"`     // base64 for images, text content for files
	MimeType string `json:"mimeType"` // e.g. "image/png", "text/plain"
}

// Message là 1 message CHUẨN HOÁ, không phụ thuộc provider. Adapter dịch qua/lại
// định dạng riêng của từng provider.
type Message struct {
	Role        Role
	Content     string
	ToolCalls   []ToolCall   // khi assistant yêu cầu gọi tool
	ToolCallID  string       // khi Role=tool: id của tool_call tương ứng
	Attachments []Attachment // image/file attachments on user messages (multimodal)
}

// ToolDef mô tả 1 tool cho LLM (tên + mô tả + JSON Schema cho args).
type ToolDef struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

// ToolCall là yêu cầu gọi tool do LLM sinh ra.
type ToolCall struct {
	ID               string
	Name             string
	Args             json.RawMessage
	ThoughtSignature []byte // Gemini thought_signature (required for multi-turn tool use)
}

// ChunkKind phân loại một mẩu trong luồng stream trả về.
type ChunkKind int

const (
	ChunkText ChunkKind = iota
	ChunkToolCall
	ChunkUsage
	ChunkDone
	ChunkError
)

// StreamChunk là 1 mẩu trong luồng streaming đã chuẩn hoá.
type StreamChunk struct {
	Kind     ChunkKind
	Text     string
	ToolCall *ToolCall
	Usage    *Usage
	Err      error
}

// Usage đếm token cho quan sát chi phí.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// ThinkingLevel điều khiển mức "suy luận" (Gemini 3.x). OFF = không truyền.
type ThinkingLevel string

const (
	ThinkingOff    ThinkingLevel = "OFF"
	ThinkingLow    ThinkingLevel = "LOW"
	ThinkingMedium ThinkingLevel = "MEDIUM"
	ThinkingHigh   ThinkingLevel = "HIGH"
)

// ProviderOptions là tuỳ chọn sinh cho mỗi request.
type ProviderOptions struct {
	Model         string
	MaxTokens     int
	ThinkingLevel ThinkingLevel
	Cache         bool // đánh dấu phần ổn định (system+tools+context) để prompt-cache
}

// GenerateRequest là request chuẩn hoá gửi cho Provider.
type GenerateRequest struct {
	System   string
	Messages []Message
	Tools    []ToolDef
	Options  ProviderOptions
}

// BuildToolResultMessage tạo message role=tool từ kết quả 1 tool_call.
// (Hàm thuần — test được không cần I/O.)
func BuildToolResultMessage(callID, result string) Message {
	return Message{Role: RoleTool, ToolCallID: callID, Content: result}
}
