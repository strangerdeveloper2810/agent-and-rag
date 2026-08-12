// Package deepseek is an adapter for DeepSeek (OpenAI-compatible API), implementing
// provider.Provider. Uses the standard OpenAI chat completions endpoint. DeepSeek is
// ~10x cheaper than Gemini/Claude and serves as the immediate fallback for rate-limited
// primary providers with zero cooldown.
//
// Auto-routes: simple tasks → flash model, complex (tools, multi-turn) → pro model.
// Tool names with dots are sanitized to underscores (DeepSeek rejects dots in names).
package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Client implements provider.Provider for DeepSeek's OpenAI-compatible API.
type Client struct {
	apiKey     string
	flashModel string
	proModel   string
	baseURL    string
	hc         *http.Client
}

var _ provider.Provider = (*Client)(nil)

func New(apiKey, flashModel, proModel string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("deepseek: apiKey is required")
	}
	if flashModel == "" {
		flashModel = "deepseek-v4-flash"
	}
	if proModel == "" {
		proModel = "deepseek-v4-pro"
	}
	return &Client{
		apiKey:     apiKey,
		flashModel: flashModel,
		proModel:   proModel,
		baseURL:    "https://api.deepseek.com/v1",
		hc:         &http.Client{},
	}, nil
}

func (c *Client) Name() string { return "deepseek" }

// pickModel routes to flash or pro based on request complexity and reasoning config.
func (c *Client) pickModel(req provider.GenerateRequest) string {
	if req.Options.Model != "" {
		return req.Options.Model
	}
	if req.Options.ThinkingLevel != "" && req.Options.ThinkingLevel != provider.ThinkingOff {
		return c.proModel
	}
	if len(req.Messages) > 10 {
		return c.proModel
	}
	for _, m := range req.Messages {
		if m.Role == provider.RoleUser && len(m.Content) > 1000 {
			return c.proModel
		}
	}
	return c.flashModel
}

// --- OpenAI-compatible types ---

type dsMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content,omitempty"`
	ToolCalls  []dsToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type dsToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function dsFunctionCall `json:"function"`
}

type dsFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type dsToolDef struct {
	Type     string           `json:"type"`
	Function dsFunctionSchema `json:"function"`
}

type dsFunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type dsChatRequest struct {
	Model     string      `json:"model"`
	Messages  []dsMessage `json:"messages"`
	Tools     []dsToolDef `json:"tools,omitempty"`
	Stream    bool        `json:"stream"`
	MaxTokens int         `json:"max_tokens,omitempty"`
}

type dsChoice struct {
	Index        int        `json:"index"`
	Delta        *dsMessage `json:"delta,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

type dsStreamChunk struct {
	Choices []dsChoice `json:"choices"`
	Usage   *dsUsage   `json:"usage,omitempty"`
}

type dsUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Name sanitization (DeepSeek regex: ^[a-zA-Z0-9_-]+$ — no dots) ---

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func unsanitizeName(name string) string {
	return strings.ReplaceAll(name, "_", ".")
}

// --- Translation ---

func toDSMessages(msgs []provider.Message) []dsMessage {
	out := make([]dsMessage, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case provider.RoleAssistant:
			dm := dsMessage{Role: "assistant"}
			if len(m.ToolCalls) > 0 {
				// Assistant with tool calls — must NOT have content (DeepSeek requirement)
				dm.ToolCalls = make([]dsToolCall, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					dm.ToolCalls[i] = dsToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: dsFunctionCall{
							Name:      sanitizeName(tc.Name),
							Arguments: string(tc.Args),
						},
					}
				}
			} else if m.Content != "" {
				dm.Content = m.Content
			}
			out = append(out, dm)

		case provider.RoleTool:
			// DeepSeek supports native tool role (OpenAI format)
			out = append(out, dsMessage{
				Role:       "tool",
				ToolCallID: m.ToolCallID,
				Content:    m.Content,
			})

		case provider.RoleUser:
			out = append(out, dsMessage{Role: "user", Content: m.Content})

		case provider.RoleSystem:
			out = append(out, dsMessage{Role: "system", Content: m.Content})
		}
	}
	return out
}

func toDSTools(tools []provider.ToolDef) []dsToolDef {
	out := make([]dsToolDef, 0, len(tools))
	for _, t := range tools {
		schema := map[string]any{}
		if len(t.Schema) > 0 {
			_ = json.Unmarshal(t.Schema, &schema)
		}
		out = append(out, dsToolDef{
			Type: "function",
			Function: dsFunctionSchema{
				Name:        sanitizeName(t.Name),
				Description: t.Description,
				Parameters:  schema,
			},
		})
	}
	return out
}

// --- Generate ---

func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	model := c.pickModel(req)
	reasoningEnabled := (req.Options.ThinkingLevel != "" && req.Options.ThinkingLevel != provider.ThinkingOff) || model == c.proModel

	slog.Info("deepseek: calling API",
		"model", model,
		"reasoning_enabled", reasoningEnabled,
		"thinking_level", string(req.Options.ThinkingLevel),
		"tools_count", len(req.Tools),
	)

	messages := toDSMessages(req.Messages)
	if req.System != "" {
		messages = append([]dsMessage{{Role: "system", Content: req.System}}, messages...)
	}

	body := dsChatRequest{
		Model:     model,
		Messages:  messages,
		Tools:     toDSTools(req.Tools),
		Stream:    true,
		MaxTokens: req.Options.MaxTokens,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("deepseek: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("deepseek: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("deepseek: http request: %w", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("deepseek: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	out := make(chan provider.StreamChunk)
	go c.streamSSE(ctx, resp.Body, out)
	return out, nil
}

// pendingTool accumulates incremental tool call deltas across SSE chunks.
type pendingTool struct {
	id   string
	name string
	args strings.Builder
}

// streamSSE reads the SSE stream and emits normalized StreamChunks.
// Handles OpenAI-style incremental tool call deltas with name sanitization.
func (c *Client) streamSSE(ctx context.Context, body io.ReadCloser, out chan provider.StreamChunk) {
	defer close(out)
	defer body.Close()

	emit := func(chunk provider.StreamChunk) bool {
		select {
		case out <- chunk:
			return true
		case <-ctx.Done():
			return false
		}
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(nil, 64*1024)

var toolCalls []pendingTool

for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "data: [DONE]" {
			flushToolCalls(&toolCalls, emit)
			emit(provider.StreamChunk{Kind: provider.ChunkDone})
			return
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk dsStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Usage
		if chunk.Usage != nil {
			if !emit(provider.StreamChunk{
				Kind: provider.ChunkUsage,
				Usage: &provider.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				},
			}) {
				return
			}
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta
			if delta == nil {
				continue
			}

			// Text
			if delta.Content != "" {
				if !emit(provider.StreamChunk{Kind: provider.ChunkText, Text: delta.Content}) {
					return
				}
			}

			// Incremental tool call deltas
			for _, tc := range delta.ToolCalls {
				for len(toolCalls) <= tc.Index {
					toolCalls = append(toolCalls, pendingTool{})
				}
				pt := &toolCalls[tc.Index]
				if tc.ID != "" {
					pt.id = tc.ID
				}
				if tc.Function.Name != "" {
					pt.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					pt.args.WriteString(tc.Function.Arguments)
				}
			}

			// On finish, flush completed tool calls
			if choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" {
				flushToolCalls(&toolCalls, emit)
			}
		}
	}

	// End of stream (no DONE marker)
	flushToolCalls(&toolCalls, emit)
	emit(provider.StreamChunk{Kind: provider.ChunkDone})
}

func flushToolCalls(toolCalls *[]pendingTool, emit func(provider.StreamChunk) bool) {
	for _, tc := range *toolCalls {
		if tc.name != "" && tc.args.Len() > 0 {
			if !emit(provider.StreamChunk{
				Kind: provider.ChunkToolCall,
				ToolCall: &provider.ToolCall{
					ID:   tc.id,
					Name: unsanitizeName(tc.name),
					Args: json.RawMessage(tc.args.String()),
				},
			}) {
				return
			}
		}
	}
	*toolCalls = nil
}
