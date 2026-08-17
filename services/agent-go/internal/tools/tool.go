// Package tools định nghĩa Tool interface + registry. Tool là provider-agnostic;
// registry sinh []provider.ToolDef cho LLM, adapter dịch tiếp sang định dạng riêng.
package tools

import (
	"context"
	"encoding/json"
	"time"
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

// TimeoutTool là interface TUỲ CHỌN — tool nào cần deadline riêng (network
// call, shell command...) thì implement thêm. Registry.runOne bọc ctx bằng
// context.WithTimeout trước khi gọi Execute nếu tool thoả interface này và
// Timeout() > 0.
//
// Cơ chế này HỢP TÁC (cooperative), không phải hard-kill: nó chỉ hữu ích với
// tool tự tôn trọng ctx.Done() bên trong Execute (vd exec.CommandContext,
// http.NewRequestWithContext) — giống mọi context.Context khác trong Go,
// không có cách ép 1 goroutine đang chạy dừng lại nếu nó không tự kiểm tra ctx.
type TimeoutTool interface {
	Tool
	Timeout() time.Duration
}
