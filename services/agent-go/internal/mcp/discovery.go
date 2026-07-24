// Package mcp cung cấp MCP client qua subprocess stdin/stdout JSON-RPC 2.0,
// và MCPRegistry tự động discovery tool từ file YAML cấu hình.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// mcpAdapter biến MCP tool thành tools.Tool để dùng chung registry.
type mcpAdapter struct {
	name        string
	description string
	schema      json.RawMessage
	client      *MCPClient
}

func (a *mcpAdapter) Name() string            { return a.name }
func (a *mcpAdapter) Description() string     { return a.description }
func (a *mcpAdapter) Schema() json.RawMessage { return a.schema }
func (a *mcpAdapter) Kind() tools.Kind        { return tools.KindRead }
func (a *mcpAdapter) Execute(_ context.Context, args json.RawMessage) (tools.Result, error) {
	text, err := a.client.CallTool(a.name, args)
	if err != nil {
		return tools.Result{}, err
	}
	return tools.Result{Content: text}, nil
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
		if ext := name[len(name)-4:]; ext != ".yml" && ext != "yaml" && name[len(name)-5:] != ".yaml" {
			continue
		}

		path := configDir + "/" + name
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
			name:        def.Name,
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
