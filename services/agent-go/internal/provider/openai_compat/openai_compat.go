// Package openai_compat is an adapter for ANY server exposing an OpenAI-compatible
// chat/completions API (vLLM, llama.cpp server, LM Studio, text-generation-webui...),
// implementing provider.Provider. Unlike internal/provider/deepseek (which is hardcoded
// to DeepSeek's hosted API + its thinking/reasoning_effort quirks), baseURL/model are
// caller-supplied so this adapter can point at any local OpenAI-compatible server.
//
// apiKey is OPTIONAL — many local servers don't require auth. When empty, no
// Authorization header is sent at all (sending an empty "Bearer " header can make some
// servers reject the request outright instead of treating it as anonymous).
//
// Cloned from deepseek.go's streaming/SSE machinery (see comments there for why
// scanner.Err() must be checked before treating stream end as success), with
// DeepSeek-specific fields removed: no "thinking"/"reasoning_effort" (DeepSeek v4-only,
// unknown fields on other servers are usually just ignored per JSON round-trip
// semantics, but there is no guarantee of that across every OpenAI-compatible
// implementation — safest to simply not send them), no ReasoningContent parsing, and no
// dot-to-underscore tool name sanitization (that's specifically because DeepSeek's tool
// name validator rejects dots — no evidence other OpenAI-compatible servers share that
// restriction, and OpenAI's own spec allows dots in function names).
package openai_compat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Client implements provider.Provider for an OpenAI-compatible chat/completions API.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	hc      *http.Client
}

var _ provider.Provider = (*Client)(nil)

// New creates a client pointed at an OpenAI-compatible server.
// baseURL e.g. "http://localhost:8000/v1" (vLLM) or "http://localhost:8080/v1"
// (llama.cpp server) — the client appends "/chat/completions" itself.
// apiKey may be empty for servers that don't require auth.
func New(baseURL, apiKey, model string) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("openai_compat: baseURL is required")
	}
	if model == "" {
		return nil, errors.New("openai_compat: model is required")
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		hc:      &http.Client{},
	}, nil
}

func (c *Client) Name() string { return "openai_compat" }

// Model trả về model được cấu hình — để tầng fallback log đúng model nào lỗi.
func (c *Client) Model() string { return c.model }

// --- OpenAI-compatible types ---

type ocMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content,omitempty"`
	ToolCalls  []ocToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type ocToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function ocFunctionCall `json:"function"`
}

type ocFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ocToolDef struct {
	Type     string           `json:"type"`
	Function ocFunctionSchema `json:"function"`
}

type ocFunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ocChatRequest struct {
	Model     string      `json:"model"`
	Messages  []ocMessage `json:"messages"`
	Tools     []ocToolDef `json:"tools,omitempty"`
	Stream    bool        `json:"stream"`
	MaxTokens int         `json:"max_tokens,omitempty"`

	// StreamOptions bật usage trong stream — chuẩn OpenAI (không phải quirk
	// riêng của DeepSeek), giữ lại vì hầu hết server OpenAI-compatible hiện đại
	// (vLLM, llama.cpp server) đều hỗ trợ; server không hỗ trợ sẽ đơn giản bỏ
	// qua field lạ này.
	StreamOptions *ocStreamOptions `json:"stream_options,omitempty"`
}

type ocStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ocChoice struct {
	Index        int        `json:"index"`
	Delta        *ocMessage `json:"delta,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

type ocStreamChunk struct {
	Choices []ocChoice `json:"choices"`
	Usage   *ocUsage   `json:"usage,omitempty"`
}

type ocUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Translation ---

func toMessages(msgs []provider.Message) []ocMessage {
	out := make([]ocMessage, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case provider.RoleAssistant:
			om := ocMessage{Role: "assistant"}
			if len(m.ToolCalls) > 0 {
				// Assistant with tool calls — must NOT have content (standard
				// OpenAI chat completions requirement).
				om.ToolCalls = make([]ocToolCall, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					om.ToolCalls[i] = ocToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: ocFunctionCall{
							Name:      tc.Name,
							Arguments: string(tc.Args),
						},
					}
				}
			} else if m.Content != "" {
				om.Content = m.Content
			}
			out = append(out, om)

		case provider.RoleTool:
			out = append(out, ocMessage{
				Role:       "tool",
				ToolCallID: m.ToolCallID,
				Content:    m.Content,
			})

		case provider.RoleUser:
			out = append(out, ocMessage{Role: "user", Content: m.Content})

		case provider.RoleSystem:
			out = append(out, ocMessage{Role: "system", Content: m.Content})
		}
	}
	return out
}

func toTools(tools []provider.ToolDef) []ocToolDef {
	out := make([]ocToolDef, 0, len(tools))
	for _, t := range tools {
		schema := map[string]any{}
		if len(t.Schema) > 0 {
			_ = json.Unmarshal(t.Schema, &schema)
		}
		out = append(out, ocToolDef{
			Type: "function",
			Function: ocFunctionSchema{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schema,
			},
		})
	}
	return out
}

// --- Generate ---

func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	messages := toMessages(req.Messages)
	if req.System != "" {
		messages = append([]ocMessage{{Role: "system", Content: req.System}}, messages...)
	}

	body := ocChatRequest{
		Model:     c.model,
		Messages:  messages,
		Tools:     toTools(req.Tools),
		Stream:    true,
		MaxTokens: req.Options.MaxTokens,
		// Luôn xin usage trong stream — nếu không có flag này API tương thích
		// OpenAI thường không gửi usage khi stream.
		StreamOptions: &ocStreamOptions{IncludeUsage: true},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai_compat: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("openai_compat: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	// apiKey optional — nhiều server local (vLLM, llama.cpp server, LM Studio)
	// không yêu cầu auth. Không gửi header Authorization gì cả khi trống, thay
	// vì gửi "Bearer " rỗng — một số server có thể coi header rỗng là request
	// hỏng thay vì ẩn danh hợp lệ.
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai_compat: http request: %w", err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai_compat: http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
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
// Handles OpenAI-style incremental tool call deltas.
//
// IMPORTANT (cloned from deepseek.go, same reasoning applies here): scanner.Scan()
// returning false means EITHER a clean end of stream (already handled by the
// "data: [DONE]" branch below) OR a real read error (connection reset, timeout
// mid-stream...). We must check scanner.Err() before treating this as success,
// otherwise callers get a misleading "empty but successful" response instead of
// a clear I/O error.
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
	var finish provider.FinishReason

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "data: [DONE]" {
			flushToolCalls(&toolCalls, emit)
			emit(provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: finish})
			return
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk ocStreamChunk
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
			if r := mapFinishReason(choice.FinishReason); r != "" {
				finish = r
			}

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

	// scanner.Scan() trả false cho CẢ 2 trường hợp: hết stream bình thường (đã
	// return sớm ở nhánh "data: [DONE]" phía trên) LẪN lỗi đọc thật sự
	// (connection reset, timeout giữa chừng...). Phải check scanner.Err() TRƯỚC
	// khi coi đây là kết thúc bình thường — xem giải thích đầy đủ trong
	// deepseek.go streamSSE (bug thật đã gặp trong log dev).
	if err := scanner.Err(); err != nil {
		emit(provider.StreamChunk{Kind: provider.ChunkError, Err: fmt.Errorf("openai_compat: lỗi đọc SSE stream: %w", err)})
		return
	}

	// scanner.Err() CHỈ trả lỗi non-EOF — không bịt được trường hợp server/proxy
	// đóng stream "sạch" giữa chừng (Err() nil). Phân biệt bằng finish: stream
	// hoàn tất tử tế luôn kèm finish_reason ở chunk cuối. Nếu KHÔNG có [DONE] và
	// cũng KHÔNG có finish_reason nào thì đây là stream bị cắt ngang.
	if finish == "" {
		emit(provider.StreamChunk{Kind: provider.ChunkError, Err: errors.New("openai_compat: stream kết thúc giữa đường (không có [DONE] lẫn finish_reason)")})
		return
	}

	// End of stream (thiếu [DONE] nhưng đã có finish_reason → coi là hợp lệ)
	flushToolCalls(&toolCalls, emit)
	emit(provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: finish})
}

// mapFinishReason dịch finish_reason kiểu OpenAI sang chuẩn chung.
// "length" = bị cắt vì chạm max_tokens.
func mapFinishReason(raw string) provider.FinishReason {
	switch raw {
	case "length":
		return provider.FinishLength
	case "tool_calls":
		return provider.FinishToolCalls
	case "stop":
		return provider.FinishStop
	default:
		return ""
	}
}

func flushToolCalls(toolCalls *[]pendingTool, emit func(provider.StreamChunk) bool) {
	for _, tc := range *toolCalls {
		// Chỉ cần có TÊN là hợp lệ — tool không có tham số nào cũng có thể gửi
		// arguments: "" (xem deepseek.go flushToolCalls cho bug lịch sử chỗ này).
		if tc.name == "" {
			continue
		}
		args := tc.args.String()
		if args == "" {
			args = "{}"
		}
		if !emit(provider.StreamChunk{
			Kind: provider.ChunkToolCall,
			ToolCall: &provider.ToolCall{
				ID:   tc.id,
				Name: tc.name,
				Args: json.RawMessage(args),
			},
		}) {
			return
		}
	}
	*toolCalls = nil
}
