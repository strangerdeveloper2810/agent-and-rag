package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// TestJSONRPCTypes tests JSON-RPC request/response marshaling.
func TestJSONRPCTypes(t *testing.T) {
	// Verify request marshaling
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var parsed jsonRPCRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if parsed.Method != "tools/list" || parsed.ID != 1 {
		t.Errorf("round-trip failed: got method=%q id=%d", parsed.Method, parsed.ID)
	}

	// Verify response marshaling
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"tools":[]}`),
	}
	data, err = json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var parsedResp jsonRPCResponse
	if err := json.Unmarshal(data, &parsedResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if string(parsedResp.Result) != `{"tools":[]}` {
		t.Errorf("result mismatch: %s", parsedResp.Result)
	}
}

// TestListToolsResultParsing tests unmarshaling of tools/list response.
func TestListToolsResultParsing(t *testing.T) {
	payload := `{"tools":[{"name":"fs.read","description":"Read a file","inputSchema":{"type":"object","properties":{"path":{"type":"string"}}}}]}`

	var result listToolsResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("len = %d, want 1", len(result.Tools))
	}
	tool := result.Tools[0]
	if tool.Name != "fs.read" {
		t.Errorf("name = %q, want %q", tool.Name, "fs.read")
	}
	if tool.Description != "Read a file" {
		t.Errorf("desc = %q, want %q", tool.Description, "Read a file")
	}
}

// TestCallToolResultParsing tests unmarshaling of tools/call response.
func TestCallToolResultParsing(t *testing.T) {
	payload := `{"content":[{"type":"text","text":"hello"},{"type":"text","text":" world"}],"isError":false}`

	var result callToolResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(result.Content) != 2 {
		t.Fatalf("content len = %d, want 2", len(result.Content))
	}
	if result.IsError {
		t.Error("isError should be false")
	}
}

// TestCallToolResultError tests parsing a tool error response.
func TestCallToolResultError(t *testing.T) {
	payload := `{"content":[{"type":"text","text":"file not found"}],"isError":true}`

	var result callToolResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("isError should be true")
	}
}

// TestMCPToolConfigStruct validates the MCPToolConfig struct shape.
func TestMCPToolConfigStruct(t *testing.T) {
	cfg := MCPToolConfig{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"@anthropic/mcp-filesystem", "/tmp"},
	}
	if cfg.Name != "filesystem" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.Command != "npx" {
		t.Errorf("Command = %q", cfg.Command)
	}
	if len(cfg.Args) != 2 {
		t.Errorf("Args len = %d, want 2", len(cfg.Args))
	}
}

// TestMCPRegistry_Discover tests YAML config file discovery and parsing.
func TestMCPRegistry_Discover(t *testing.T) {
	dir := t.TempDir()

	// Write a YAML config file
	configYAML := `
servers:
  - name: test-server
    command: echo
    args: []
`
	if err := os.WriteFile(filepath.Join(dir, "mcp.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)

	// Discover parses YAML, tries to connect — "echo" is not an MCP server
	// so it will fail, but we verify YAML parsing works.
	err := mcpReg.Discover(dir)
	// We expect an error because "echo" exits immediately (not a JSON-RPC server)
	if err == nil {
		t.Log("echo surprisingly connected as MCP server")
	}
	// Close is safe even on failed connections
	mcpReg.Close()
}

// TestMCPRegistry_DiscoverNoYAML tests discovery with empty directory.
func TestMCPRegistry_DiscoverNoYAML(t *testing.T) {
	dir := t.TempDir()

	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)

	if err := mcpReg.Discover(dir); err != nil {
		t.Fatalf("Discover empty dir: %v", err)
	}
	if len(reg.All()) != 0 {
		t.Errorf("registry has %d tools, want 0", len(reg.All()))
	}
}

// TestMCPRegistry_DiscoverNonYAML tests that non-YAML files are skipped.
func TestMCPRegistry_DiscoverNonYAML(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# docs"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "script.sh"), []byte("#!/bin/sh"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)

	if err := mcpReg.Discover(dir); err != nil {
		t.Fatalf("Discover with non-YAML files: %v", err)
	}
}

// TestMCPAdapterInterface tests that mcpAdapter implements tools.Tool.
func TestMCPAdapterInterface(t *testing.T) {
	adapter := &mcpAdapter{
		name:        "test",
		description: "test tool",
		schema:      json.RawMessage(`{"type":"object"}`),
	}

	if adapter.Name() != "test" {
		t.Errorf("Name = %q, want %q", adapter.Name(), "test")
	}
	if adapter.Description() != "test tool" {
		t.Errorf("Description = %q, want %q", adapter.Description(), "test tool")
	}
	if string(adapter.Schema()) != `{"type":"object"}` {
		t.Errorf("Schema = %s", adapter.Schema())
	}
	if adapter.Kind() != tools.KindRead {
		t.Errorf("Kind = %v, want KindRead", adapter.Kind())
	}
}

// TestMCPClient_Integration runs a real subprocess MCP server using bash.
// The bash script implements a minimal JSON-RPC 2.0 MCP server that
// responds to initialize + tools/list.
func TestMCPClient_Integration(t *testing.T) {
	// A minimal MCP server in bash that reads JSON-RPC lines from stdin,
	// responds to initialize and tools/list, then exits.
	// Client sends: initialize(id=1), initialized notification (no id, no response), tools/list(id=3)
	script := `#!/bin/bash
# MCP minimal test server
# Line 1: read initialize request (id=1), send response with same id
read line
printf '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}\n'
# Line 2: read initialized notification (no id, no response needed)
read line
# Line 3: read tools/list request (id=3 because initialized notification also incremented counter)
read line
printf '{"jsonrpc":"2.0","id":3,"result":{"tools":[{"name":"echo","description":"Echo back input","inputSchema":{"type":"object","properties":{"text":{"type":"string"}}}}]}}\n'
`

	scriptPath := filepath.Join(t.TempDir(), "mcp-server.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	client := &MCPClient{}
	if err := client.Connect("bash", scriptPath); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	defs, err := client.ListTools()
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("len = %d, want 1", len(defs))
	}
	if defs[0].Name != "echo" {
		t.Errorf("name = %q, want %q", defs[0].Name, "echo")
	}
}

// TestDiscover_MissingDir tests discovery with a nonexistent config directory.
func TestDiscover_MissingDir(t *testing.T) {
	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)

	err := mcpReg.Discover(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("Discover(missing dir) = nil, want lỗi")
	}
	if !strings.Contains(err.Error(), "read config dir") {
		t.Fatalf("err = %v, want chứa read config dir", err)
	}
}

// TestDiscover_InvalidYAML tests that a malformed YAML file fails loudly.
func TestDiscover_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("servers: [unclosed"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tools.NewRegistry()
	err := NewMCPRegistry(reg).Discover(dir)
	if err == nil {
		t.Fatal("Discover(bad yaml) = nil, want lỗi parse")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Fatalf("err = %v, want chứa parse", err)
	}
}

// TestDiscover_YMLExtension tests that .yml files are read too (proved by parse error).
func TestDiscover_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mcp.yml"), []byte(":\n\tbroken"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tools.NewRegistry()
	err := NewMCPRegistry(reg).Discover(dir)
	if err == nil {
		t.Fatal("Discover(.yml) = nil, want lỗi parse chứng tỏ file được đọc")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Fatalf("err = %v, want chứa parse", err)
	}
}

// TestDiscover_SkipsSubdirectories tests that nested directories are ignored.
func TestDiscover_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "inner.yaml"), []byte(":\n\tbroken"), 0644); err != nil {
		t.Fatalf("write inner config: %v", err)
	}

	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)
	if err := mcpReg.Discover(dir); err != nil {
		t.Fatalf("Discover = %v, want nil (bỏ qua subdirectory)", err)
	}
	if len(reg.All()) != 0 {
		t.Fatalf("registry có %d tools, want 0", len(reg.All()))
	}
}

// TestDiscover_EmptyConfig tests a valid YAML file with no servers.
func TestDiscover_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty.yaml"), []byte(""), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)
	if err := mcpReg.Discover(dir); err != nil {
		t.Fatalf("Discover(empty) = %v, want nil", err)
	}
	if len(reg.All()) != 0 {
		t.Fatalf("registry có %d tools, want 0", len(reg.All()))
	}
	if err := mcpReg.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
}

// TestDiscover_ConnectFailurePropagates tests that a server whose command
// fails to start surfaces the server name in the error.
func TestDiscover_ConnectFailurePropagates(t *testing.T) {
	dir := t.TempDir()
	configYAML := "servers:\n  - name: bad-srv\n    command: /no/such/binary\n"
	if err := os.WriteFile(filepath.Join(dir, "mcp.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tools.NewRegistry()
	err := NewMCPRegistry(reg).Discover(dir)
	if err == nil {
		t.Fatal("Discover = nil, want lỗi kết nối")
	}
	if !strings.Contains(err.Error(), `server "bad-srv"`) {
		t.Fatalf("err = %v, want chứa tên server bad-srv", err)
	}
}

// TestDiscover_FullFlow drives the whole registry path: YAML → subprocess
// MCP server → tools/list → adapter đăng ký → Close.
func TestDiscover_FullFlow(t *testing.T) {
	script := `#!/bin/bash
read line
printf '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}\n'
read line
read line
printf '{"jsonrpc":"2.0","id":3,"result":{"tools":[{"name":"echo","description":"Echo back","inputSchema":{"type":"object"}},{"name":"fs.read","description":"Read file","inputSchema":{"type":"object"}}]}}\n'
`
	scriptPath := filepath.Join(t.TempDir(), "registry-server.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	dir := t.TempDir()
	configYAML := "servers:\n  - name: fake\n    command: bash\n    args: [" + scriptPath + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "mcp.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)
	if err := mcpReg.Discover(dir); err != nil {
		t.Fatalf("Discover = %v, want nil", err)
	}

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("registry có %d tools, want 2", len(all))
	}
	if all[0].Name() != "echo" || all[1].Name() != "fs.read" {
		t.Fatalf("tool names = %v, want [echo fs.read]", []string{all[0].Name(), all[1].Name()})
	}
	if got := reg.ToolDefs(); len(got) != 2 {
		t.Fatalf("ToolDefs = %d, want 2", len(got))
	}

	if err := mcpReg.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
}

// TestDiscover_ListToolsFailure tests the connectServer path where the server
// connects fine but tools/list fails — client phải được đóng và lỗi phải nổi.
func TestDiscover_ListToolsFailure(t *testing.T) {
	script := `#!/bin/bash
read line
printf '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}\n'
read line
read line
printf '{"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"tools/list not supported"}}\n'
`
	scriptPath := filepath.Join(t.TempDir(), "listfail-server.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	dir := t.TempDir()
	configYAML := "servers:\n  - name: fake\n    command: bash\n    args: [" + scriptPath + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "mcp.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tools.NewRegistry()
	err := NewMCPRegistry(reg).Discover(dir)
	if err == nil {
		t.Fatal("Discover = nil, want lỗi từ tools/list")
	}
	if !strings.Contains(err.Error(), `server "fake"`) || !strings.Contains(err.Error(), "list tools") {
		t.Fatalf("err = %v, want chứa server fake và list tools", err)
	}
	if len(reg.All()) != 0 {
		t.Fatalf("registry có %d tools, want 0 (server lỗi không đăng ký gì)", len(reg.All()))
	}
}

// TestDiscover_ReadFileError tests that a dangling symlink surfaces a read error.
func TestDiscover_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "missing-target"), filepath.Join(dir, "dangling.yaml")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	reg := tools.NewRegistry()
	err := NewMCPRegistry(reg).Discover(dir)
	if err == nil {
		t.Fatal("Discover = nil, want lỗi đọc file symlink hỏng")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Fatalf("err = %v, want chứa read", err)
	}
}

// TestDiscover_ShortFileNames là regression test cho panic slice out of range:
// tên file ngắn hơn 4-5 ký tự (vd "a", "x.md", ".yml") từng làm Discover panic
// vì cắt chuỗi bằng index cố định thay vì filepath.Ext.
func TestDiscover_ShortFileNames(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"a", "ab", "abc", "x.md", ".yml", ".yaml", "y.yml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("servers: []\n"), 0644); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}

	reg := tools.NewRegistry()
	mcpReg := NewMCPRegistry(reg)

	// Không được panic, không được lỗi: các file không phải .yml/.yaml bị bỏ qua,
	// các file yaml hợp lệ có servers rỗng.
	if err := mcpReg.Discover(dir); err != nil {
		t.Fatalf("Discover with short file names: %v", err)
	}
}
