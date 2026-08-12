// Package gemini là adapter cho Google Gemini (google.golang.org/genai), hiện thực
// provider.Provider. Adapter dịch Message/ToolDef chuẩn hoá ↔ kiểu riêng của genai và
// gom stream của genai về provider.StreamChunk.
package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"google.golang.org/genai"
)

// Client bọc *genai.Client, giữ model + mức thinking mặc định cho provider này.
type Client struct {
	client    *genai.Client
	model     string
	thinking  provider.ThinkingLevel
	cacheName string // cached content resource name (empty = no cache)
}

var _ provider.Provider = (*Client)(nil)

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

func (c *Client) Name() string { return "gemini" }

// ---------------------------------------------------------------------------
// Hàm dịch THUẦN (không I/O, test được)
// ---------------------------------------------------------------------------

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
					ThoughtSignature: tc.ThoughtSignature,
				})
			}
			out = append(out, &genai.Content{Role: genai.RoleModel, Parts: parts})

		case provider.RoleTool:
			out = append(out, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						ID:       m.ToolCallID,
						Name:     findToolName(msgs, m.ToolCallID),
						Response: map[string]any{"output": m.Content},
					},
					ThoughtSignature: findThoughtSignature(msgs, m.ToolCallID),
				}},
			})

		default:
			parts := make([]*genai.Part, 0, 1+len(m.Attachments))
			if m.Content != "" {
				parts = append(parts, &genai.Part{Text: m.Content})
			}
			for _, att := range m.Attachments {
				if att.Type == "image" {
					raw, err := base64.StdEncoding.DecodeString(att.Data)
					if err != nil {
						// Skip malformed images; text fallback is already in Content.
						continue
					}
					parts = append(parts, &genai.Part{
						InlineData: &genai.Blob{
							MIMEType: att.MimeType,
							Data:     raw,
						},
					})
				}
			}
			out = append(out, &genai.Content{Role: genai.RoleUser, Parts: parts})
		}
	}
	return out
}

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

func mapThinkingLevel(l provider.ThinkingLevel) *genai.ThinkingConfig {
	switch l {
	case provider.ThinkingLow:
		return &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow}
	case provider.ThinkingMedium:
		return &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelMedium}
	case provider.ThinkingHigh:
		return &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelHigh}
	default:
		return nil
	}
}

func findToolName(msgs []provider.Message, callID string) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == provider.RoleAssistant {
			for _, tc := range msgs[i].ToolCalls {
				if tc.ID == callID {
					return tc.Name
				}
			}
		}
	}
	return ""
}

func findThoughtSignature(msgs []provider.Message, callID string) []byte {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == provider.RoleAssistant {
			for _, tc := range msgs[i].ToolCalls {
				if tc.ID == callID && len(tc.ThoughtSignature) > 0 {
					return tc.ThoughtSignature
				}
			}
		}
	}
	return nil
}

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
// Generate
// ---------------------------------------------------------------------------

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
	// Enable Google Search as built-in Gemini tool (no API key needed).
	config.Tools = append(config.Tools, &genai.Tool{GoogleSearch: &genai.GoogleSearch{}})

	// Context cache: if a cached content name was set, use it for reduced costs.
	if c.cacheName != "" {
		config.CachedContent = c.cacheName
	}

	level := c.thinking
	if req.Options.ThinkingLevel != "" {
		level = req.Options.ThinkingLevel
	}
	if tc := mapThinkingLevel(level); tc != nil {
		config.ThinkingConfig = tc
	}
	reasoningEnabled := level != "" && level != provider.ThinkingOff

	slog.Info("gemini: calling API",
		"model", model,
		"reasoning_enabled", reasoningEnabled,
		"thinking_level", string(level),
		"tools_count", len(req.Tools),
	)

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
								ID:               part.FunctionCall.ID,
								Name:             part.FunctionCall.Name,
								Args:             args,
								ThoughtSignature: part.ThoughtSignature,
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
