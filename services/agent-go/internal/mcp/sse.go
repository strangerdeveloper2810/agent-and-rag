package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// ServerConfig mô tả một MCP server từ xa do user tự cấu hình (SSE/Streamable HTTP).
// Khác MCPToolConfig (subprocess stdio, admin-only), cấu hình này chỉ gồm URL + API key,
// KHÔNG chạy lệnh local — an toàn cho MCP server của người dùng cuối.
type ServerConfig struct {
	Name   string
	URL    string
	APIKey string
}

const (
	mcpProtocolVersion = "2024-11-05"
	mcpClientName      = "jarvis-go"
	mcpClientVersion   = "0.1.0"
	mcpHTTPTimeout     = 30 * time.Second
)

// SSEClient nói chuyện với MCP server từ xa qua Streamable HTTP: mỗi JSON-RPC 2.0
// request là một HTTP POST; response có thể là JSON thuần (application/json) hoặc
// SSE (text/event-stream). Đây là transport mặc định cho MCP server remote.
type SSEClient struct {
	url    string
	apiKey string

	httpClient *http.Client
	mu         sync.Mutex
	nextID     int64
	sessionID  string // Mcp-Session-Id do server cấp (tuỳ chọn)
}

// NewSSEClient tạo client kết nối tới một MCP server SSE remote.
func NewSSEClient(url, apiKey string) *SSEClient {
	return &SSEClient{
		url:        url,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: mcpHTTPTimeout},
	}
}

// Connect gửi handshake "initialize" rồi "notifications/initialized".
func (c *SSEClient) Connect(ctx context.Context) error {
	if _, err := c.sendRequest(ctx, "initialize", map[string]interface{}{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    mcpClientName,
			"version": mcpClientVersion,
		},
	}); err != nil {
		return fmt.Errorf("mcp: initialize: %w", err)
	}

	// Notification initialized không có response — id < 0 để bỏ qua phần đọc body.
	c.mu.Lock()
	c.nextID++
	notif := jsonRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"}
	c.mu.Unlock()
	_, _ = c.post(ctx, notif, -1)
	return nil
}

// ListTools gọi "tools/list" và trả về danh sách tool definitions.
func (c *SSEClient) ListTools(ctx context.Context) ([]provider.ToolDef, error) {
	raw, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: list tools: %w", err)
	}

	var result listToolsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp: parse tools/list: %w", err)
	}

	defs := make([]provider.ToolDef, 0, len(result.Tools))
	for _, t := range result.Tools {
		defs = append(defs, provider.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Schema:      t.InputSchema,
		})
	}
	return defs, nil
}

// CallTool gọi "tools/call" và trả về kết quả dạng text.
// Không nhận ctx vì được gọi giữa lượt chạy qua adapter — dùng timeout của httpClient.
func (c *SSEClient) CallTool(name string, args json.RawMessage) (string, error) {
	raw, err := c.sendRequest(context.Background(), "tools/call", callToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("mcp: call tool %q: %w", name, err)
	}

	var result callToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("mcp: parse tools/call: %w", err)
	}

	var text string
	for _, c := range result.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	if result.IsError {
		return text, fmt.Errorf("mcp: tool %q returned error: %s", name, text)
	}
	return text, nil
}

// Close đóng client. Streamable HTTP không giữ kết nối lâu dài nên không cần dọn dẹp.
func (c *SSEClient) Close() error { return nil }

// sendRequest cấp id mới rồi gửi request và đọc response khớp theo id.
func (c *SSEClient) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	req := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	c.mu.Unlock()

	return c.post(ctx, req, id)
}

// post gửi JSON-RPC request qua HTTP POST và parse response (JSON hoặc SSE).
// expectedID < 0 → notification, không cần đọc kết quả.
func (c *SSEClient) post(ctx context.Context, req jsonRPCRequest, expectedID int64) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcp: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp: http post: %w", err)
	}
	defer resp.Body.Close()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("mcp: server trả %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	if expectedID < 0 {
		return nil, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcp: read response: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return parseSSEResponse(data, expectedID)
	}
	return parseJSONResponse(data, expectedID)
}

func parseJSONResponse(data []byte, expectedID int64) (json.RawMessage, error) {
	var resp jsonRPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("mcp: parse response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcp: rpc error code=%d: %s", resp.Error.Code, resp.Error.Message)
	}
	if resp.ID != expectedID {
		return nil, fmt.Errorf("mcp: response id mismatch: got %d, want %d", resp.ID, expectedID)
	}
	return resp.Result, nil
}

func parseSSEResponse(data []byte, expectedID int64) (json.RawMessage, error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(payload), &resp); err != nil {
			continue
		}
		if resp.ID != expectedID {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp: rpc error code=%d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
	return nil, fmt.Errorf("mcp: không tìm thấy response id=%d trong SSE stream", expectedID)
}

// DiscoverSSE kết nối tới từng MCP server remote, discovery tools và đăng ký vào
// một registry RIÊNG cho lượt chạy hiện tại (tránh ghi vào registry dùng chung gây
// data race và rò rỉ tool giữa các user). Trả về registry + danh sách client để
// caller đóng khi kết thúc lượt chạy.
func DiscoverSSE(ctx context.Context, servers []ServerConfig) (*tools.Registry, []*SSEClient, error) {
	reg := tools.NewRegistry()
	clients := make([]*SSEClient, 0, len(servers))

	for _, srv := range servers {
		if strings.TrimSpace(srv.URL) == "" {
			continue
		}
		client := NewSSEClient(srv.URL, srv.APIKey)
		if err := client.Connect(ctx); err != nil {
			_ = client.Close()
			return reg, clients, fmt.Errorf("mcp: connect %q: %w", srv.Name, err)
		}

		defs, err := client.ListTools(ctx)
		if err != nil {
			_ = client.Close()
			return reg, clients, fmt.Errorf("mcp: list tools %q: %w", srv.Name, err)
		}

		for _, def := range defs {
			reg.Register(&mcpAdapter{
				name:        qualifiedToolName(srv.Name, def.Name),
				rawName:     def.Name,
				description: def.Description,
				schema:      def.Schema,
				client:      client,
			})
		}
		clients = append(clients, client)
	}

	return reg, clients, nil
}
