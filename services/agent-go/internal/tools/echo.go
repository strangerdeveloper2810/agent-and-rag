package tools

import (
	"context"
	"encoding/json"
)

// echoTool là tool tối giản dùng cho học/test: trả nguyên args nhận được.
// Kind=KindRead vì nó không gây side-effect.
type echoTool struct{}

// NewEchoTool tạo một echo tool.
func NewEchoTool() Tool { return echoTool{} }

func (echoTool) Name() string { return "echo" }

func (echoTool) Description() string {
	return "Trả lại nguyên văn args đầu vào (dùng để học/test luồng tool)."
}

func (echoTool) Schema() json.RawMessage {
	// Nhận bất kỳ object nào; không ràng buộc field.
	return json.RawMessage(`{"type":"object","additionalProperties":true}`)
}

func (echoTool) Kind() Kind { return KindRead }

func (echoTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	return Result{Content: string(args)}, nil
}
