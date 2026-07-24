// Package anthropic là adapter Claude cho lớp trừu tượng provider.Provider.
// Nó dịch Message/ToolDef chuẩn hoá ↔ định dạng của anthropic-sdk-go và gom
// SSE stream của Anthropic về provider.StreamChunk chuẩn.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Client hiện thực provider.Provider bằng anthropic-sdk-go.
type Client struct {
	sdk   sdk.Client
	model string
}

// New tạo Client. model là model mặc định (vd "claude-opus-4-8"); có thể ghi đè
// qua GenerateRequest.Options.Model cho từng request.
func New(apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("anthropic: apiKey rỗng")
	}
	if model == "" {
		return nil, errors.New("anthropic: model rỗng")
	}
	c := sdk.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.anthropic.com"),
	)
	return &Client{sdk: c, model: model}, nil
}

// Name trả về định danh provider.
func (c *Client) Name() string { return "anthropic" }

// defaultMaxTokens dùng khi Options.MaxTokens <= 0.
const defaultMaxTokens = 4096

// toAnthropicMessages dịch danh sách Message chuẩn hoá sang []sdk.MessageParam.
//
// Quy tắc map:
//   - RoleUser      → user message chứa 1 text block.
//   - RoleAssistant → assistant message: text block (nếu có Content) + mỗi ToolCall
//     thành 1 tool_use block (id/name/input).
//   - RoleTool      → user message chứa 1 tool_result block (khớp ToolCallID).
//   - RoleSystem    → BỎ QUA (system prompt đi qua GenerateRequest.System, không
//     nằm trong mảng messages của Anthropic).
//
// Hàm thuần: không I/O, không gọi mạng — test được trực tiếp.
func toAnthropicMessages(msgs []provider.Message) []sdk.MessageParam {
	out := make([]sdk.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case provider.RoleUser:
			blocks := make([]sdk.ContentBlockParamUnion, 0, 1+len(m.Attachments))
			if m.Content != "" {
				blocks = append(blocks, sdk.NewTextBlock(m.Content))
			}
			for _, att := range m.Attachments {
				if att.Type == "image" {
					blocks = append(blocks, sdk.NewImageBlockBase64(att.MimeType, att.Data))
				}
			}
			if len(blocks) == 0 {
				blocks = append(blocks, sdk.NewTextBlock(""))
			}
			out = append(out, sdk.NewUserMessage(blocks...))

		case provider.RoleAssistant:
			blocks := make([]sdk.ContentBlockParamUnion, 0, 1+len(m.ToolCalls))
			if m.Content != "" {
				blocks = append(blocks, sdk.NewTextBlock(m.Content))
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, sdk.NewToolUseBlock(tc.ID, toolInput(tc.Args), tc.Name))
			}
			if len(blocks) == 0 {
				// Assistant rỗng: giữ 1 text block rỗng để message hợp lệ.
				blocks = append(blocks, sdk.NewTextBlock(""))
			}
			out = append(out, sdk.NewAssistantMessage(blocks...))

		case provider.RoleTool:
			// tool_result là content của 1 user message theo API Anthropic.
			out = append(out, sdk.NewUserMessage(
				sdk.NewToolResultBlock(m.ToolCallID, m.Content, false),
			))

		case provider.RoleSystem:
			// System đi qua GenerateRequest.System — bỏ qua ở đây.
			continue
		}
	}
	return out
}

// toolInput chuẩn hoá args của tool_call thành giá trị marshal được. json.RawMessage
// rỗng/nil sẽ hỏng khi marshal, nên thay bằng object rỗng.
func toolInput(args json.RawMessage) any {
	if len(args) == 0 {
		return json.RawMessage("{}")
	}
	return args
}

// toAnthropicTools dịch []provider.ToolDef sang []sdk.ToolUnionParam. Mỗi ToolDef
// mang JSON Schema đầy đủ; ta rút "properties" + "required" vào ToolInputSchemaParam.
//
// Hàm thuần: không I/O — test được trực tiếp.
func toAnthropicTools(tools []provider.ToolDef) []sdk.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	out := make([]sdk.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		schema := sdk.ToolInputSchemaParam{}
		if len(t.Schema) > 0 {
			var parsed struct {
				Properties json.RawMessage `json:"properties"`
				Required   []string        `json:"required"`
			}
			// Bỏ qua lỗi parse: schema hỏng → tool vẫn khai báo với schema trống,
			// để lỗi lộ ra ở tầng gọi API thay vì nuốt im lặng ở đây.
			_ = json.Unmarshal(t.Schema, &parsed)
			if len(parsed.Properties) > 0 {
				schema.Properties = parsed.Properties
			}
			schema.Required = parsed.Required
		}
		if schema.Properties == nil {
			schema.Properties = json.RawMessage("{}")
		}

		tp := sdk.ToolParam{
			Name:        sanitizeToolName(t.Name),
			InputSchema: schema,
		}
		if t.Description != "" {
			tp.Description = sdk.String(t.Description)
		}
		out = append(out, sdk.ToolUnionParam{OfTool: &tp})
	}
	return out
}

// buildParams gom GenerateRequest thành MessageNewParams (thuần, không gọi mạng).

func sanitizeToolName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func unsanitizeToolName(name string) string {
	return strings.ReplaceAll(name, "_", ".")
}

func (c *Client) buildParams(req provider.GenerateRequest) sdk.MessageNewParams {
	model := c.model
	if req.Options.Model != "" {
		model = req.Options.Model
	}
	maxTokens := int64(req.Options.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	params := sdk.MessageNewParams{
		Model:     sdk.Model(model),
		MaxTokens: maxTokens,
		Messages:  toAnthropicMessages(req.Messages),
	}
	if req.System != "" {
		sys := sdk.TextBlockParam{Text: req.System}
		if req.Options.Cache {
			// Đánh dấu prompt-cache trên khối system ổn định.
			sys.CacheControl = sdk.NewCacheControlEphemeralParam()
		}
		params.System = []sdk.TextBlockParam{sys}
	}
	if tools := toAnthropicTools(req.Tools); len(tools) > 0 {
		params.Tools = tools
	}
	return params
}

// Generate mở SSE stream tới Anthropic và gom về channel StreamChunk chuẩn hoá.
// Tôn trọng ctx (cancel/timeout) và luôn đóng channel khi xong.
func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	params := c.buildParams(req)

	out := make(chan provider.StreamChunk)
	go func() {
		defer close(out)

		stream := c.sdk.Messages.NewStreaming(ctx, params)
		acc := sdk.Message{}

		for stream.Next() {
			event := stream.Current()
			if err := acc.Accumulate(event); err != nil {
				send(ctx, out, provider.StreamChunk{Kind: provider.ChunkError, Err: err})
				return
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta.Text != "" {
					if !send(ctx, out, provider.StreamChunk{Kind: provider.ChunkText, Text: event.Delta.Text}) {
						return
					}
				}
			case "content_block_stop":
				// Khi 1 block tool_use hoàn tất, phát ToolCall với input đã gom đủ.
				idx := int(event.Index)
				if idx >= 0 && idx < len(acc.Content) {
					if blk := acc.Content[idx]; blk.Type == "tool_use" {
						args := blk.Input
						if len(args) == 0 {
							args = json.RawMessage("{}")
						}
						tc := provider.ToolCall{ID: blk.ID, Name: unsanitizeToolName(blk.Name), Args: args}
						if !send(ctx, out, provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: &tc}) {
							return
						}
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			send(ctx, out, provider.StreamChunk{Kind: provider.ChunkError, Err: err})
			return
		}

		usage := provider.Usage{
			InputTokens:  int(acc.Usage.InputTokens),
			OutputTokens: int(acc.Usage.OutputTokens),
		}
		if !send(ctx, out, provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &usage}) {
			return
		}
		send(ctx, out, provider.StreamChunk{Kind: provider.ChunkDone})
	}()

	return out, nil
}

// send gửi chunk nhưng tôn trọng ctx: trả false nếu ctx bị huỷ (để goroutine thoát).
func send(ctx context.Context, out chan<- provider.StreamChunk, chunk provider.StreamChunk) bool {
	select {
	case out <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

// Bảo đảm Client thoả interface tại thời điểm compile.
var _ provider.Provider = (*Client)(nil)
