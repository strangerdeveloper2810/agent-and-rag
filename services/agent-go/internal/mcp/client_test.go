package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// recordingStdin ghi lại mọi byte client gửi cho MCP server.
type recordingStdin struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *recordingStdin) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *recordingStdin) Close() error { return nil }

func (w *recordingStdin) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// errReader luôn trả lỗi khi đọc — mô phỏng stdout hỏng.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("stdout broken") }

// failingStdin trả lỗi khi ghi — mô phỏng server đóng stdin sớm.
type failingStdin struct{}

func (failingStdin) Write([]byte) (int, error) { return 0, errors.New("pipe closed") }
func (failingStdin) Close() error              { return nil }

// newFakeClient dựng MCPClient với stdin giả và stdout chứa sẵn các dòng response.
func newFakeClient(responses string) (*MCPClient, *recordingStdin) {
	stdin := &recordingStdin{}
	return &MCPClient{
		stdin:  stdin,
		stdout: bufio.NewScanner(strings.NewReader(responses)),
	}, stdin
}

func TestCallTool_Success(t *testing.T) {
	c, stdin := newFakeClient(
		`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"hello"},{"type":"image","text":"ignored"},{"type":"text","text":" world"}]}}` + "\n",
	)

	text, err := c.CallTool("echo", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q, want hello world (gộp mọi text, bỏ qua type khác)", text)
	}

	raw := stdin.String()
	if !strings.Contains(raw, `"method":"tools/call"`) {
		t.Fatalf("request = %q, want chứa tools/call", raw)
	}
	if !strings.Contains(raw, `"name":"echo"`) {
		t.Fatalf("request = %q, want chứa tên tool", raw)
	}
	if !strings.Contains(raw, `"arguments":{"x":1}`) {
		t.Fatalf("request = %q, want chứa arguments", raw)
	}
}

func TestCallTool_IsError(t *testing.T) {
	c, _ := newFakeClient(
		`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"file not found"}],"isError":true}}` + "\n",
	)

	text, err := c.CallTool("fs.read", nil)
	if err == nil {
		t.Fatal("CallTool = nil, want lỗi khi isError=true")
	}
	if text != "file not found" {
		t.Fatalf("text = %q, want file not found", text)
	}
	if !strings.Contains(err.Error(), `"fs.read"`) {
		t.Fatalf("err = %v, want chứa tên tool", err)
	}
}

func TestCallTool_EmptyResult(t *testing.T) {
	c, _ := newFakeClient(`{"jsonrpc":"2.0","id":1,"result":{}}` + "\n")

	text, err := c.CallTool("t", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want rỗng", text)
	}
}

func TestCallTool_RequestError(t *testing.T) {
	c, _ := newFakeClient("") // stdout đóng → sendRequest lỗi
	_, err := c.CallTool("t", nil)
	if err == nil || !strings.Contains(err.Error(), "call tool") {
		t.Fatalf("err = %v, want chứa call tool", err)
	}
}

func TestCallTool_ParseError(t *testing.T) {
	c, _ := newFakeClient(`{"jsonrpc":"2.0","id":1,"result":"not-an-object"}` + "\n")
	_, err := c.CallTool("t", nil)
	if err == nil || !strings.Contains(err.Error(), "parse tools/call") {
		t.Fatalf("err = %v, want chứa parse tools/call", err)
	}
}

func TestListTools_RequestError(t *testing.T) {
	c, _ := newFakeClient("")
	_, err := c.ListTools()
	if err == nil || !strings.Contains(err.Error(), "list tools") {
		t.Fatalf("err = %v, want chứa list tools", err)
	}
}

func TestListTools_ParseError(t *testing.T) {
	c, _ := newFakeClient(`{"jsonrpc":"2.0","id":1,"result":"not-an-object"}` + "\n")
	_, err := c.ListTools()
	if err == nil || !strings.Contains(err.Error(), "parse tools/list") {
		t.Fatalf("err = %v, want chứa parse tools/list", err)
	}
}

func TestListTools_Mapping(t *testing.T) {
	c, _ := newFakeClient(
		`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"fs.read","description":"Read a file","inputSchema":{"type":"object","properties":{"path":{"type":"string"}}}}]}}` + "\n",
	)

	defs, err := c.ListTools()
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("len = %d, want 1", len(defs))
	}
	if defs[0].Name != "fs.read" || defs[0].Description != "Read a file" {
		t.Fatalf("def = %+v, want fs.read", defs[0])
	}
	if !strings.Contains(string(defs[0].Schema), `"path"`) {
		t.Fatalf("Schema = %s, want inputSchema truyền nguyên vẹn", defs[0].Schema)
	}
}

func TestReadResponse_ParseError(t *testing.T) {
	c, _ := newFakeClient("not-json\n")
	if _, err := c.readResponse(1); err == nil || !strings.Contains(err.Error(), "parse response") {
		t.Fatalf("err = %v, want chứa parse response", err)
	}
}

func TestReadResponse_RPCError(t *testing.T) {
	c, _ := newFakeClient(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"unknown method"}}` + "\n")
	_, err := c.readResponse(1)
	if err == nil || !strings.Contains(err.Error(), "rpc error code=-32601") {
		t.Fatalf("err = %v, want chứa rpc error code=-32601", err)
	}
}

func TestReadResponse_IDMismatch(t *testing.T) {
	c, _ := newFakeClient(`{"jsonrpc":"2.0","id":99,"result":{}}` + "\n")
	_, err := c.readResponse(1)
	if err == nil || !strings.Contains(err.Error(), "id mismatch") {
		t.Fatalf("err = %v, want chứa id mismatch", err)
	}
}

func TestReadResponse_ClosedStdout(t *testing.T) {
	c, _ := newFakeClient("")
	_, err := c.readResponse(1)
	if err == nil || !strings.Contains(err.Error(), "closed stdout") {
		t.Fatalf("err = %v, want chứa closed stdout", err)
	}
}

func TestReadResponse_ScannerError(t *testing.T) {
	c := &MCPClient{stdout: bufio.NewScanner(errReader{})}
	_, err := c.readResponse(1)
	if err == nil || !strings.Contains(err.Error(), "stdout broken") {
		t.Fatalf("err = %v, want chứa stdout broken", err)
	}
}

func TestSendRequest_MarshalError(t *testing.T) {
	c, _ := newFakeClient("")
	// chan int không marshal được → lỗi trước khi ghi stdin.
	_, err := c.sendRequest("tools/list", make(chan int))
	if err == nil || !strings.Contains(err.Error(), "write request") {
		t.Fatalf("err = %v, want chứa write request", err)
	}
}

func TestSendRequest_WriteError(t *testing.T) {
	c := &MCPClient{stdin: failingStdin{}}
	_, err := c.sendRequest("tools/list", nil)
	if err == nil || !strings.Contains(err.Error(), "pipe closed") {
		t.Fatalf("err = %v, want chứa pipe closed", err)
	}
}

func TestWriteJSON_MarshalError(t *testing.T) {
	c := &MCPClient{stdin: failingStdin{}}
	if err := c.writeJSON(map[string]interface{}{"bad": make(chan int)}); err == nil {
		t.Fatal("writeJSON = nil, want lỗi marshal")
	}
}

func TestConnect_StartError(t *testing.T) {
	c := &MCPClient{}
	err := c.Connect(filepath.Join(t.TempDir(), "no-such-binary"))
	if err == nil || !strings.Contains(err.Error(), "start server") {
		t.Fatalf("err = %v, want chứa start server", err)
	}
}

func TestClose_NoProcess(t *testing.T) {
	if err := (&MCPClient{}).Close(); err != nil {
		t.Fatalf("Close client rỗng = %v, want nil", err)
	}
}

func TestClose_UnstartedCmd(t *testing.T) {
	// cmd chưa Start → Process nil → Close không Wait.
	c := &MCPClient{cmd: &exec.Cmd{}}
	if err := c.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
}

func TestAdapter_Execute(t *testing.T) {
	c, _ := newFakeClient(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"ok"}]}}` + "\n")
	a := &mcpAdapter{name: "echo", description: "d", schema: json.RawMessage(`{"type":"object"}`), client: c}

	res, err := a.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Content != "ok" {
		t.Fatalf("res.Content = %q, want ok", res.Content)
	}

	// isError → Execute trả lỗi.
	c2, _ := newFakeClient(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"boom"}],"isError":true}}` + "\n")
	a2 := &mcpAdapter{name: "bad", client: c2}
	if _, err := a2.Execute(context.Background(), nil); err == nil {
		t.Fatal("Execute = nil, want lỗi khi tool trả isError")
	}
}
