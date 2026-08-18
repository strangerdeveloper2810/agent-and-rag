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

// Model trả về model mặc định được cấu hình.
func (c *Client) Model() string { return c.flashModel }

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
	Role      string       `json:"role"`
	Content   string       `json:"content,omitempty"`
	ToolCalls []dsToolCall `json:"tool_calls,omitempty"`
	// ReasoningContent là chuỗi suy luận (CoT) model sinh ra, đến dưới dạng
	// delta RIÊNG chứ không nằm trong content. Không emit ra ChunkText (không
	// phải câu trả lời), nhưng PHẢI đọc để log được: khi nó ăn hết ngân sách
	// token thì content rỗng, và nếu không thấy field này thì triệu chứng
	// trông như "provider trả rỗng không rõ lý do".
	ReasoningContent string `json:"reasoning_content,omitempty"`
	ToolCallID       string `json:"tool_call_id,omitempty"`
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

	// Thinking tắt/bật chế độ suy luận. Với model deepseek-v4-* thinking BẬT
	// MẶC ĐỊNH và token suy luận TÍNH VÀO max_tokens — đã verify bằng API thật:
	// cùng prompt với max_tokens=16, nếu KHÔNG gửi field này thì toàn bộ 16
	// token vào reasoning, content trả về RỖNG và finish_reason="length"; gửi
	// {"type":"disabled"} thì content="OK" chỉ tốn 1 token.
	// Đây là lý do các task phụ trợ ngân sách nhỏ (HyDE/LLM rerank max_tokens
	// =200, reflection 4096) âm thầm trả rỗng hoặc bị cắt.
	Thinking *dsThinking `json:"thinking,omitempty"`

	// ReasoningEffort điều chỉnh ĐỘ SÂU suy luận khi thinking bật. Lưu ý đã
	// verify: reasoning_effort="low" KHÔNG tắt suy luận (vẫn ăn hết ngân sách
	// nhỏ) — muốn tắt phải dùng Thinking ở trên.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// StreamOptions bật usage trong stream. API tương thích OpenAI không gửi
	// usage ở chế độ stream nếu thiếu flag này, nên ChunkUsage trước đây
	// thường không có số thật.
	StreamOptions *dsStreamOptions `json:"stream_options,omitempty"`
}

type dsThinking struct {
	Type string `json:"type"` // "disabled" | "enabled"
}

type dsStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
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

// mapReasoningEffort ánh xạ ThinkingLevel sang reasoning_effort của DeepSeek.
// Trả rỗng khi không cần gửi (OFF xử lý riêng bằng thinking.disabled, và ""
// nghĩa là để model dùng mặc định của nó).
func mapReasoningEffort(level provider.ThinkingLevel) string {
	switch level {
	case provider.ThinkingLow:
		return "low"
	case provider.ThinkingMedium, provider.ThinkingHigh:
		return "high"
	default:
		return ""
	}
}

// --- Generate ---

func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	model := c.pickModel(req)

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
		// Luôn xin usage trong stream — nếu không có flag này API tương thích
		// OpenAI không gửi usage khi stream.
		StreamOptions: &dsStreamOptions{IncludeUsage: true},
	}

	// ThinkingLevel trước đây chỉ được dùng để LOG, không hề gửi lên API, nên
	// provider.ThinkingOff là no-op hoàn toàn với DeepSeek.
	if req.Options.ThinkingLevel == provider.ThinkingOff {
		body.Thinking = &dsThinking{Type: "disabled"}
	} else if effort := mapReasoningEffort(req.Options.ThinkingLevel); effort != "" {
		body.ReasoningEffort = effort
	}

	// Log ĐÚNG những gì thực sự gửi lên API. Trước đây log một biến
	// reasoningEnabled tự suy ra và không liên quan tới request, nên khi
	// ThinkingLevel=OFF nó vẫn in reasoning_enabled=true — sai lệch đúng chỗ
	// cần chẩn đoán nhất.
	slog.Info("deepseek: calling API",
		"model", model,
		"thinking_disabled", body.Thinking != nil,
		"reasoning_effort", body.ReasoningEffort,
		"thinking_level", string(req.Options.ThinkingLevel),
		"max_tokens", body.MaxTokens,
		"tools_count", len(req.Tools),
	)

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
	var finish provider.FinishReason
	// textLen/reasoningLen chỉ để log chẩn đoán: phân biệt "model không trả gì"
	// với "model suy luận hết ngân sách token nên không còn chỗ cho câu trả lời".
	var textLen, reasoningLen int

	// warnIfReasoningAteBudget log khi câu trả lời rỗng/bị cắt mà phần suy luận
	// lại dài — dấu hiệu ngân sách max_tokens bị CoT ăn hết. Đã verify bằng API
	// thật: max_tokens=16 + thinking bật mặc định → 16/16 token vào reasoning,
	// content rỗng, finish_reason="length".
	warnIfReasoningAteBudget := func() {
		if reasoningLen > 0 && (textLen == 0 || finish == provider.FinishLength) {
			slog.Warn("deepseek: phần suy luận (reasoning) chiếm ngân sách token, câu trả lời bị rỗng hoặc bị cắt",
				"reasoning_chars", reasoningLen,
				"text_chars", textLen,
				"finish_reason", string(finish),
				"hint", "đặt ThinkingLevel=OFF cho task ngân sách nhỏ, hoặc tăng MaxTokens",
			)
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "data: [DONE]" {
			flushToolCalls(&toolCalls, emit)
			warnIfReasoningAteBudget()
			emit(provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: finish})
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
			if r := mapFinishReason(choice.FinishReason); r != "" {
				finish = r
			}

			delta := choice.Delta
			if delta == nil {
				continue
			}

			// Text
			if delta.Content != "" {
				textLen += len(delta.Content)
				if !emit(provider.StreamChunk{Kind: provider.ChunkText, Text: delta.Content}) {
					return
				}
			}

			// Chuỗi suy luận: KHÔNG emit ra ChunkText (không phải câu trả lời),
			// chỉ đếm để log. Cần thiết vì token suy luận tính vào max_tokens —
			// khi nó ăn hết ngân sách thì content rỗng và nếu không đo được
			// phần này thì triệu chứng trông như "provider trả rỗng vô cớ".
			if delta.ReasoningContent != "" {
				reasoningLen += len(delta.ReasoningContent)
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

	// scanner.Scan() trả false cho CẢ 2 trường hợp: hết stream bình thường
	// (đã return sớm ở nhánh "data: [DONE]" phía trên) LẪN lỗi đọc thật sự
	// (connection reset, timeout giữa chừng...). Trước fix, code không phân
	// biệt — luôn rơi xuống đây và emit ChunkDone như thể thành công, khiến
	// caller (rerankLLM, generateHypotheticalAnswer, memory.ReflectAndExtract)
	// nhận 1 response "rỗng nhưng thành công" thay vì biết rõ có lỗi I/O,
	// hiện ra dưới dạng lỗi mơ hồ "unexpected end of JSON input (raw=\"\")"
	// ở tầng gọi thay vì lỗi provider thật. Phải check scanner.Err() TRƯỚC
	// khi coi đây là kết thúc bình thường.
	if err := scanner.Err(); err != nil {
		emit(provider.StreamChunk{Kind: provider.ChunkError, Err: fmt.Errorf("deepseek: lỗi đọc SSE stream: %w", err)})
		return
	}

	// scanner.Err() CHỈ trả lỗi non-EOF (theo tài liệu bufio) — nên nó KHÔNG
	// bịt được trường hợp server/proxy đóng stream "sạch" giữa chừng: khi đó
	// Err() là nil và ta rơi xuống đây. Phân biệt bằng finish: stream hoàn tất
	// tử tế luôn kèm finish_reason ở chunk cuối. Nếu KHÔNG có [DONE] và cũng
	// KHÔNG có finish_reason nào thì đây là stream bị cắt ngang, không phải
	// thành công — emit ChunkError để caller biết thay vì nhận một response
	// "rỗng nhưng thành công" rồi báo lỗi mơ hồ ở tầng parse JSON.
	if finish == "" {
		warnIfReasoningAteBudget()
		emit(provider.StreamChunk{Kind: provider.ChunkError, Err: errors.New("deepseek: stream kết thúc giữa đường (không có [DONE] lẫn finish_reason)")})
		return
	}

	// End of stream (thiếu [DONE] nhưng đã có finish_reason → coi là hợp lệ)
	flushToolCalls(&toolCalls, emit)
	warnIfReasoningAteBudget()
	emit(provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: finish})
}

// mapFinishReason dịch finish_reason kiểu OpenAI của DeepSeek sang chuẩn chung.
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
		// Chỉ cần có TÊN là hợp lệ. Trước đây còn đòi args.Len() > 0 nên tool
		// call KHÔNG có argument bị bỏ ÂM THẦM — API tương thích OpenAI gửi
		// arguments: "" cho tool mà mọi tham số đều optional. Hệ quả: tool
		// không chạy, và nếu lượt đó cũng không có text thì caller báo lỗi mơ
		// hồ "empty response". Adapter Anthropic đã xử lý đúng case này bằng
		// cách mặc định "{}" — nay đồng bộ.
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
				Name: unsanitizeName(tc.name),
				Args: json.RawMessage(args),
			},
		}) {
			return
		}
	}
	*toolCalls = nil
}
