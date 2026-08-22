// Package mcp cung cấp 2 cơ chế MCP client, KHÔNG chỉ 1:
//
//   - Subprocess stdin/stdout JSON-RPC 2.0 (MCPClient.Connect + MCPRegistry.
//     Discover đọc file YAML *.yaml/*.yml trong configDir) — cơ chế GỐC của
//     package này. Hiện KHÔNG có caller nào trong cmd/server hay cmd/jarvis
//     gọi MCPRegistry.Discover — dead code ở tầng wiring (vẫn còn test riêng),
//     giữ lại phòng khi cần MCP server chạy local dạng subprocess.
//   - SSE/Streamable HTTP remote (sse.go: DiscoverSSE + ServerConfig) — cơ chế
//     THẬT ĐANG CHẠY production: Engine.Run gọi mcp.DiscoverSSE ở MỖI lượt
//     chat cho các MCP server user tự cấu hình qua RunInput.McpServers (không
//     qua file YAML, không phải subprocess).
package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// --- JSON-RPC 2.0 structures ---

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type listToolsResult struct {
	Tools []mcpToolDef `json:"tools"`
}

type mcpToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type callToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPClient nói chuyện với MCP server qua subprocess stdin/stdout dùng JSON-RPC 2.0.
type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	nextID int64
}

// Connect khởi động MCP server subprocess và khởi tạo kết nối.
// command là đường dẫn executable, args là tham số dòng lệnh.
func (c *MCPClient) Connect(command string, args ...string) error {
	c.cmd = exec.Command(command, args...)
	c.cmd.Stderr = os.Stderr // server logs đi qua stderr của parent

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdout pipe: %w", err)
	}
	c.stdout = bufio.NewScanner(stdout)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("mcp: start server: %w", err)
	}

	// Gửi initialize request (JSON-RPC "initialize" method)
	if _, err := c.sendRequest("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "jarvis-go",
			"version": "0.1.0",
		},
	}); err != nil {
		return fmt.Errorf("mcp: initialize: %w", err)
	}

	// Gửi initialized notification (không cần response)
	c.mu.Lock()
	c.nextID++
	notif := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	c.mu.Unlock()
	if err := c.writeJSON(notif); err != nil {
		return fmt.Errorf("mcp: send initialized: %w", err)
	}

	return nil
}

// ListTools gọi "tools/list" và trả về danh sách tool definitions từ MCP server.
func (c *MCPClient) ListTools() ([]provider.ToolDef, error) {
	raw, err := c.sendRequest("tools/list", nil)
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

// CallTool gọi "tools/call" trên MCP server và trả về kết quả dạng text.
func (c *MCPClient) CallTool(name string, args json.RawMessage) (string, error) {
	raw, err := c.sendRequest("tools/call", callToolParams{
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

	// Gộp tất cả text content thành 1 string
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

// Close đóng stdin và đợi process kết thúc.
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Wait()
	}
	return nil
}

// sendRequest gửi JSON-RPC request và đọc response.
func (c *MCPClient) sendRequest(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	c.mu.Unlock()

	if err := c.writeJSON(req); err != nil {
		return nil, fmt.Errorf("mcp: write request: %w", err)
	}

	return c.readResponse(id)
}

func (c *MCPClient) writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	return err
}

func (c *MCPClient) readResponse(expectedID int64) (json.RawMessage, error) {
	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("mcp: read response: %w", err)
		}
		return nil, fmt.Errorf("mcp: server closed stdout unexpectedly")
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
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

// --- MCPRegistry ---

// MCPToolConfig mô tả một MCP server trong file YAML config.
type MCPToolConfig struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// MCPConfig là cấu trúc file config MCP (vd mcp.yaml).
type MCPConfig struct {
	Servers []MCPToolConfig `yaml:"servers"`
}

// toolClient là interface gọi tool trên một MCP server — giúp adapter dùng chung
// cho cả transport stdio (MCPClient) lẫn SSE remote (SSEClient).
type toolClient interface {
	CallTool(name string, args json.RawMessage) (string, error)
}

// mcpAdapter biến MCP tool thành tools.Tool để dùng chung registry.
// name là tên đã namespace (dùng cho LLM + registry key); rawName là tên gốc
// server trả về (dùng khi gọi "tools/call" qua JSON-RPC — server không biết
// namespace prefix). rawName rỗng → Execute rơi về dùng name (tương thích
// adapter dựng tay trong test, trước khi namespace tồn tại).
type mcpAdapter struct {
	name        string
	rawName     string
	description string
	schema      json.RawMessage
	client      toolClient
}

func (a *mcpAdapter) Name() string            { return a.name }
func (a *mcpAdapter) Description() string     { return a.description }
func (a *mcpAdapter) Schema() json.RawMessage { return a.schema }
func (a *mcpAdapter) Kind() tools.Kind        { return tools.KindRead }
func (a *mcpAdapter) Execute(_ context.Context, args json.RawMessage) (tools.Result, error) {
	name := a.rawName
	if name == "" {
		name = a.name
	}
	text, err := a.client.CallTool(name, args)
	if err != nil {
		return tools.Result{}, err
	}
	return tools.Result{Content: text}, nil
}

// maxToolNameLen giới hạn độ dài tên tool theo ràng buộc chung của các provider
// (OpenAI/DeepSeek function name tối đa 64 ký tự).
const maxToolNameLen = 64

// invalidToolNameChar khớp mọi ký tự ngoài [A-Za-z0-9_-] — ràng buộc tên hàm
// chung của hầu hết provider (DeepSeek/Anthropic chỉ tự sanitize dấu chấm).
var invalidToolNameChar = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// qualifiedToolName namespace 1 MCP tool thành "mcp__<server>__<raw>" (cùng quy
// ước Claude Code/Codex dùng) để tool trùng tên từ 2 server khác nhau không bao
// giờ đụng độ trong registry. Ký tự không hợp lệ được thay bằng "_".
//
// Nếu tên sau namespace vượt maxToolNameLen, cắt bớt và gắn hash 8-hex của
// (server, raw) — hash chỉ phụ thuộc 2 input này, không phụ thuộc thứ tự kết
// nối server hay số lần re-sync, nên tên ổn định qua các lần khởi động.
func qualifiedToolName(server, raw string) string {
	name := "mcp__" + invalidToolNameChar.ReplaceAllString(server, "_") +
		"__" + invalidToolNameChar.ReplaceAllString(raw, "_")
	if len(name) <= maxToolNameLen {
		return name
	}

	sum := sha256.Sum256([]byte(server + "\x00" + raw))
	suffix := "-" + hex.EncodeToString(sum[:])[:8]
	keep := max(maxToolNameLen-len(suffix), 0)
	return name[:keep] + suffix
}

// MCPRegistry quản lý nhiều MCP client, tự động discovery tool từ file YAML.
type MCPRegistry struct {
	clients []*MCPClient
	reg     *tools.Registry
}

// NewMCPRegistry tạo MCPRegistry rỗng.
func NewMCPRegistry(reg *tools.Registry) *MCPRegistry {
	return &MCPRegistry{reg: reg}
}

// Discover đọc file YAML trong configDir (*.yaml, *.yml), kết nối từng MCP server,
// list tools, và đăng ký vào registry.
func (r *MCPRegistry) Discover(configDir string) error {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return fmt.Errorf("mcp: read config dir %q: %w", configDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// filepath.Ext an toàn với tên file ngắn (vd "a", ".yml") — cắt chuỗi
		// bằng index cố định sẽ panic khi len(name) < 5.
		if ext := filepath.Ext(name); ext != ".yml" && ext != ".yaml" {
			continue
		}

		path := filepath.Join(configDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("mcp: read %q: %w", path, err)
		}

		var config MCPConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("mcp: parse %q: %w", path, err)
		}

		for _, srv := range config.Servers {
			if err := r.connectServer(srv); err != nil {
				return fmt.Errorf("mcp: server %q: %w", srv.Name, err)
			}
		}
	}

	return nil
}

// connectServer kết nối 1 MCP server, list tools, đăng ký vào registry.
func (r *MCPRegistry) connectServer(cfg MCPToolConfig) error {
	client := &MCPClient{}
	if err := client.Connect(cfg.Command, cfg.Args...); err != nil {
		return fmt.Errorf("connect %q: %w", cfg.Name, err)
	}

	defs, err := client.ListTools()
	if err != nil {
		client.Close()
		return fmt.Errorf("list tools %q: %w", cfg.Name, err)
	}

	for _, def := range defs {
		adapter := &mcpAdapter{
			name:        qualifiedToolName(cfg.Name, def.Name),
			rawName:     def.Name,
			description: def.Description,
			schema:      def.Schema,
			client:      client,
		}
		r.reg.Register(adapter)
	}

	r.clients = append(r.clients, client)
	return nil
}

// Close đóng tất cả MCP clients.
func (r *MCPRegistry) Close() error {
	for _, c := range r.clients {
		c.Close()
	}
	return nil
}
