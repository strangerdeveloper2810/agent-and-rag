// Package tools định nghĩa Tool interface + registry. Tool là provider-agnostic;
// registry sinh []provider.ToolDef cho LLM, adapter dịch tiếp sang định dạng riêng.
package tools

import (
	"context"
	"encoding/json"
)

// Kind phân loại tool để phục vụ guardrail / HITL.
type Kind int

const (
	KindRead        Kind = iota // an toàn: ragSearch, listDocuments, readDocument, listTasks, recallMemory
	KindWrite                   // tạo/sửa: createTask, updateTask, saveMemory
	KindDestructive             // phá huỷ: deleteTask → cần HITL xác nhận
)

// Result là kết quả một lần chạy tool.
type Result struct {
	Content string          // đưa lại cho LLM (thường JSON string)
	Meta    json.RawMessage // metadata phụ (vd citation) — tuỳ chọn
}

// Tool là 1 công cụ agent có thể gọi.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON Schema cho args
	Kind() Kind
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}
