// Package gemini là adapter cho Google Gemini (google.golang.org/genai), hiện thực
// provider.Provider. Adapter dịch Message/ToolDef chuẩn hoá ↔ kiểu riêng của genai và
// gom stream của genai về provider.StreamChunk.
package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"google.golang.org/genai"
)

// Client bọc *genai.Client, giữ model + mức thinking mặc định cho provider này.
type Client struct {
	client   *genai.Client
	model    string
	thinking provider.ThinkingLevel
}

// đảm bảo Client thoả interface provider.Provider tại compile-time.
var _ provider.Provider = (*Client)(nil)

// New tạo Client Gemini dùng Backend Gemini API (không phải Vertex). apiKey bắt buộc.
func New(apiKey, model string, thinking provider.ThinkingLevel) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("gemini: apiKey is required")
	}
	if model == "" {
		return nil, errors.New("gemini: model is required")
	}
	gc, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: new client: %w", err)
	}
	return &Client{client: gc, model: model, thinking: thinking}, nil
}

// Name trả về tên provider.
func (c *Client) Name() string { return "gemini" }

// ---------------------------------------------------------------------------
// Hàm dịch THUẦN (không I/O, test được)
// ---------------------------------------------------------------------------

// toGeminiContents dịch danh sách Message chuẩn hoá sang []*genai.Content.
//
// Ánh xạ role: user→"user", assistant→"model", tool→"user" (Gemini gửi kết quả tool
// bằng FunctionResponse trong content role "user"), system→"user" (genai contents chỉ
// nhận user/model; system nên đi qua SystemInstruction, nhưng nếu lọt vào messages thì
// coi như text của user để không mất dữ liệu).
func toGeminiContents(msgs []provider.Message) []*genai.Content {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*genai.Content, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case provider.RoleAssistant:
			parts := make([]*genai.Part, 0, len(m.ToolCalls)+1)
			if m.Content != "" {
				parts = append(parts, &genai.Part{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   tc.ID,
						Name: tc.Name,
						Args: rawToMap(tc.Args),
					},
				})
			}
			out = append(out, &genai.Content{Role: genai.RoleModel, Parts: parts})

		case provider.RoleTool:
			out = append(out, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						ID:       m.ToolCallID,
						Response: map[string]any{"output": m.Content},
					},
				}},
			})

		default: // RoleUser, RoleSystem, và bất kỳ role lạ nào
			out = append(out, &genai.Content{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{{Text: m.Content}},
			})
		}
	}
	return out
}

// toGeminiTools dịch []ToolDef sang []*genai.Tool. Gom mọi FunctionDeclaration vào 1 Tool
// theo quy ước Gemini. JSON Schema đưa thẳng vào ParametersJsonSchema (đã map[string]any).
// Trả nil nếu không có tool nào.
func toGeminiTools(tools []provider.ToolDef) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}
	decls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		decl := &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
		}
		if len(t.Schema) > 0 {
			decl.ParametersJsonSchema = rawToMap(t.Schema)
		}
		decls = append(decls, decl)
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

// mapThinkingLevel dịch mức thinking chuẩn hoá sang *genai.ThinkingConfig.
// OFF (hoặc rỗng) → nil (không set); LOW/MEDIUM/HIGH → ThinkingConfig tương ứng.
func mapThinkingLevel(l provider.ThinkingLevel) *genai.ThinkingConfig {
	switch l {
	case provider.ThinkingLow:
		return &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow}
	case provider.ThinkingMedium:
		return &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelMedium}
	case provider.ThinkingHigh:
		return &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelHigh}
	default: // ThinkingOff, "" → không set
		return nil
	}
}

// rawToMap giải mã json.RawMessage thành map[string]any. Rỗng/lỗi → map rỗng (không panic),
// giữ hành vi dịch ổn định cho args/schema.
func rawToMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// ---------------------------------------------------------------------------
// Generate — wire streaming của genai
// ---------------------------------------------------------------------------

// Generate gọi genai GenerateContentStream và gom kết quả về channel StreamChunk chuẩn hoá.
// Tôn trọng ctx: mọi thao tác gửi channel đều huỷ được, và channel luôn được đóng khi xong.
func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	if c.client == nil {
		return nil, errors.New("gemini: client not initialized")
	}

	model := c.model
	if req.Options.Model != "" {
		model = req.Options.Model
	}

	config := &genai.GenerateContentConfig{}
	if req.System != "" {
		config.SystemInstruction = genai.NewContentFromText(req.System, genai.RoleUser)
	}
	if req.Options.MaxTokens > 0 {
		config.MaxOutputTokens = int32(req.Options.MaxTokens)
	}
	if tools := toGeminiTools(req.Tools); tools != nil {
		config.Tools = tools
	}

	level := c.thinking
	if req.Options.ThinkingLevel != "" {
		level = req.Options.ThinkingLevel
	}
	if tc := mapThinkingLevel(level); tc != nil {
		config.ThinkingConfig = tc
	}

	contents := toGeminiContents(req.Messages)

	out := make(chan provider.StreamChunk)
	go func() {
		defer close(out)

		emit := func(chunk provider.StreamChunk) bool {
			select {
			case out <- chunk:
				return true
			case <-ctx.Done():
				return false
			}
		}

		for resp, err := range c.client.Models.GenerateContentStream(ctx, model, contents, config) {
			if err != nil {
				emit(provider.StreamChunk{Kind: provider.ChunkError, Err: err})
				return
			}
			if resp == nil {
				continue
			}

			// Duyệt trực tiếp các part của candidate đầu tiên: text + function call.
			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					switch {
					case part == nil:
						continue
					case part.FunctionCall != nil:
						args, _ := json.Marshal(part.FunctionCall.Args)
						if !emit(provider.StreamChunk{
							Kind: provider.ChunkToolCall,
							ToolCall: &provider.ToolCall{
								ID:   part.FunctionCall.ID,
								Name: part.FunctionCall.Name,
								Args: args,
							},
						}) {
							return
						}
					case part.Text != "" && !part.Thought:
						if !emit(provider.StreamChunk{Kind: provider.ChunkText, Text: part.Text}) {
							return
						}
					}
				}
			}

			if u := resp.UsageMetadata; u != nil {
				if !emit(provider.StreamChunk{
					Kind: provider.ChunkUsage,
					Usage: &provider.Usage{
						InputTokens:  int(u.PromptTokenCount),
						OutputTokens: int(u.CandidatesTokenCount),
					},
				}) {
					return
				}
			}
		}

		emit(provider.StreamChunk{Kind: provider.ChunkDone})
	}()

	return out, nil
}
