package provider

import "context"

// Provider là lớp trừu tượng cho một nhà cung cấp LLM. Adapter (gemini, anthropic)
// hiện thực interface này. Generate trả về channel StreamChunk và PHẢI tôn trọng
// ctx (cancel/timeout) — đóng channel khi xong hoặc khi ctx bị huỷ.
type Provider interface {
	Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
	Name() string
}
