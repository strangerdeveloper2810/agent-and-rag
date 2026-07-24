package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Client implements provider.Provider for Ollama (local LLM).
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

var _ provider.Provider = (*Client)(nil)

// New creates an Ollama client. baseURL e.g. "http://localhost:11434".
func New(baseURL, model string) (*Client, error) {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: &http.Client{},
	}, nil
}

func (c *Client) Name() string { return "ollama" }

// Generate calls Ollama /api/chat with streaming, maps to StreamChunk channel.
func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	body := ollamaChatRequest{
		Model:    c.model,
		Messages: toOllamaMessages(req.Messages),
		Tools:    toOllamaTools(req.Tools),
		Stream:   true,
	}
	if req.Options.MaxTokens > 0 {
		body.Options = &ollamaOptions{NumPredict: req.Options.MaxTokens}
	}
	if req.System != "" {
		body.Messages = append([]ollamaMessage{{Role: "system", Content: req.System}}, body.Messages...)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	ch := make(chan provider.StreamChunk, 16)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		emit := func(chunk provider.StreamChunk) bool {
			select {
			case ch <- chunk:
				return true
			case <-ctx.Done():
				return false
			}
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			chunk, err := fromOllamaChunk(scanner.Bytes())
			if err != nil {
				if !emit(provider.StreamChunk{Kind: provider.ChunkError, Err: err}) {
					return
				}
				continue
			}
			if !emit(chunk) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			emit(provider.StreamChunk{Kind: provider.ChunkError, Err: fmt.Errorf("ollama: read stream: %w", err)})
		}
	}()

	return ch, nil
}

// --- Ollama API types ---

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaTool struct {
	Type     string          `json:"type"`
	Function ollamaFunction  `json:"function"`
}

type ollamaFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ollamaToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaChunk struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done bool `json:"done"`
}

// --- Translation functions (PURE, testable) ---

func toOllamaMessages(msgs []provider.Message) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(msgs))
	for _, m := range msgs {
		om := ollamaMessage{}
		switch m.Role {
		case provider.RoleUser:
			om.Role = "user"
		case provider.RoleAssistant:
			om.Role = "assistant"
		case provider.RoleSystem:
			om.Role = "system"
		case provider.RoleTool:
			om.Role = "user"
			om.Content = fmt.Sprintf("Tool result (call_id=%s): %s", m.ToolCallID, m.Content)
		default:
			om.Role = "user"
		}

		if m.Role == provider.RoleAssistant && len(m.ToolCalls) > 0 {
			om.ToolCalls = make([]ollamaToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				om.ToolCalls[i] = ollamaToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: ollamaFunctionCall{
						Name:      tc.Name,
						Arguments: tc.Args,
					},
				}
			}
		}
		if m.Content != "" || m.Role != provider.RoleAssistant {
			om.Content = m.Content
		}
		out = append(out, om)
	}
	return out
}

func toOllamaTools(tools []provider.ToolDef) []ollamaTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ollamaTool, len(tools))
	for i, t := range tools {
		out[i] = ollamaTool{
			Type: "function",
			Function: ollamaFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			},
		}
	}
	return out
}

func fromOllamaChunk(line []byte) (provider.StreamChunk, error) {
	var c ollamaChunk
	if err := json.Unmarshal(line, &c); err != nil {
		return provider.StreamChunk{}, fmt.Errorf("ollama: parse chunk: %w", err)
	}

	if c.Done {
		return provider.StreamChunk{Kind: provider.ChunkDone}, nil
	}

	if len(c.Message.ToolCalls) > 0 {
		tc := c.Message.ToolCalls[0]
		toolCall := &provider.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		}
		return provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: toolCall}, nil
	}

	if c.Message.Content != "" {
		return provider.StreamChunk{Kind: provider.ChunkText, Text: c.Message.Content}, nil
	}

	return provider.StreamChunk{}, nil
}
