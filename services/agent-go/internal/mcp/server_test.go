package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// fakePrivilegedTool giả lập 1 tool ĐĂNG KÝ TRÙNG TÊN với tool đặc quyền thật
// ("shell.exec") nhưng Execute vô hại — mục đích DUY NHẤT của nó là chứng
// minh Server lọc theo TÊN qua tools.IsPrivilegedTool, không phải theo type
// hay theo registry gốc nó tới từ đâu. Nếu Server lộ tool này ra (tools/list
// hoặc cho tools/call chạy), đó là dấu hiệu lớp lọc privileged bị vô hiệu.
type fakePrivilegedTool struct{ called bool }

func (f *fakePrivilegedTool) Name() string            { return "shell.exec" }
func (f *fakePrivilegedTool) Description() string     { return "fake shell.exec dùng để test lọc" }
func (f *fakePrivilegedTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (f *fakePrivilegedTool) Kind() tools.Kind        { return tools.KindDestructive }
func (f *fakePrivilegedTool) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	f.called = true
	return tools.Result{Content: "PWNED"}, nil
}

// testRegistry dựng 1 registry gồm: 2 tool an toàn thật (calculator, echo) +
// 1 fakePrivilegedTool tên "shell.exec". Trả về registry và con trỏ tới fake
// tool để test kiểm tra Execute có bị gọi hay không.
func testRegistry() (*tools.Registry, *fakePrivilegedTool) {
	reg := tools.NewRegistry()
	reg.Register(tools.NewCalculatorTool())
	reg.Register(tools.NewEchoTool())
	fake := &fakePrivilegedTool{}
	reg.Register(fake)
	return reg, fake
}

// doRPC gửi 1 JSON-RPC request qua Server.ServeHTTP và parse response.
func doRPC(t *testing.T, s *Server, method string, id string, params interface{}) (*httptest.ResponseRecorder, rpcServerResponse) {
	t.Helper()
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		paramsRaw = b
	}
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if id != "" {
		reqBody["id"] = json.RawMessage(id)
	}
	if paramsRaw != nil {
		reqBody["params"] = paramsRaw
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(body)))
	req.RemoteAddr = "127.0.0.1:1234" // authorized() mặc định chỉ cho loopback
	s.ServeHTTP(rec, req)

	var resp rpcServerResponse
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("parse response: %v (body: %s)", err, rec.Body.String())
		}
	}
	return rec, resp
}

func TestServer_Initialize_ReturnsCapabilities(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	rec, resp := doRPC(t, s, "initialize", "1", map[string]interface{}{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "test-client", "version": "0.0.1"},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if string(resp.ID) != "1" {
		t.Errorf("id = %s, want 1", resp.ID)
	}

	var result rpcInitializeResult
	if err := json.Unmarshal(marshalResult(t, resp.Result), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.ProtocolVersion != "2025-06-18" {
		t.Errorf("protocolVersion = %q, want echoed 2025-06-18", result.ProtocolVersion)
	}
	if result.Capabilities.Tools == nil {
		t.Fatal("capabilities.tools phải có mặt — server này expose tools")
	}
	if result.ServerInfo.Name == "" {
		t.Error("serverInfo.name rỗng")
	}
}

// TestServer_Initialize_DefaultsProtocolVersionWhenOmitted xác nhận server
// không panic/lỗi khi client không gửi protocolVersion, và rơi về default.
func TestServer_Initialize_DefaultsProtocolVersionWhenOmitted(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	_, resp := doRPC(t, s, "initialize", "1", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result rpcInitializeResult
	if err := json.Unmarshal(marshalResult(t, resp.Result), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.ProtocolVersion != mcpServerProtocolVersion {
		t.Errorf("protocolVersion = %q, want default %q", result.ProtocolVersion, mcpServerProtocolVersion)
	}
}

// TestServer_ToolsList_ExcludesPrivilegedTools là test QUAN TRỌNG NHẤT của
// file này: registry chứa 1 tool tên "shell.exec" (tools.IsPrivilegedTool ==
// true) đăng ký CẠNH 2 tool an toàn — tools/list KHÔNG được liệt kê nó, dù nó
// nằm trong registry truyền vào Server, dù filter == nil (không lọc thêm gì).
func TestServer_ToolsList_ExcludesPrivilegedTools(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	_, resp := doRPC(t, s, "tools/list", "2", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result listToolsResult
	if err := json.Unmarshal(marshalResult(t, resp.Result), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	names := make(map[string]bool, len(result.Tools))
	for _, tl := range result.Tools {
		names[tl.Name] = true
		if tools.IsPrivilegedTool(tl.Name) {
			t.Errorf("tools/list lộ tool đặc quyền %q — lớp hard-exclude BỊ VÔ HIỆU", tl.Name)
		}
	}
	if !names["calculator"] || !names["echo"] {
		t.Errorf("tools/list thiếu tool an toàn hợp lệ, có: %v", names)
	}
	if names["shell.exec"] {
		t.Fatal("shell.exec (đặc quyền) xuất hiện trong tools/list")
	}
}

// TestServer_ToolsCall_RejectsPrivilegedTool xác nhận KHÔNG chỉ tools/list mà
// cả tools/call cũng chặn — gọi thẳng "shell.exec" (không qua tools/list)
// phải bị từ chối bằng lỗi protocol, và Execute() của fake tool KHÔNG ĐƯỢC
// gọi (chứng minh bằng field fake.called).
func TestServer_ToolsCall_RejectsPrivilegedTool(t *testing.T) {
	reg, fake := testRegistry()
	s := NewServer(reg, nil)

	_, resp := doRPC(t, s, "tools/call", "3", map[string]interface{}{
		"name":      "shell.exec",
		"arguments": map[string]interface{}{"command": "rm -rf /"},
	})

	if resp.Error == nil {
		t.Fatal("tools/call shell.exec phải trả JSON-RPC error, không phải result")
	}
	if resp.Error.Code != jsonRPCInvalidParams {
		t.Errorf("error code = %d, want %d (Invalid params / Unknown tool)", resp.Error.Code, jsonRPCInvalidParams)
	}
	if fake.called {
		t.Fatal("fake shell.exec.Execute() ĐÃ bị gọi — lớp chặn privileged không ngăn được tools/call")
	}
}

// TestServer_ToolsCall_CallsRealTool xác nhận đường đi thành công: gọi
// calculator qua tools/call, nhận đúng kết quả tính toán.
func TestServer_ToolsCall_CallsRealTool(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	_, resp := doRPC(t, s, "tools/call", "4", map[string]interface{}{
		"name":      "calculator",
		"arguments": map[string]interface{}{"expression": "2 + 2"},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result callToolResult
	if err := json.Unmarshal(marshalResult(t, resp.Result), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.IsError {
		t.Fatalf("isError = true, content: %+v", result.Content)
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "4") {
		t.Errorf("content = %+v, muốn thấy kết quả 4", result.Content)
	}
}

// TestServer_ToolsCall_UnknownTool: tool không tồn tại trong registry (không
// phải privileged, chỉ đơn giản chưa từng đăng ký) cũng phải trả lỗi rõ ràng
// thay vì panic hay 500.
func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	_, resp := doRPC(t, s, "tools/call", "5", map[string]interface{}{
		"name":      "khong-ton-tai",
		"arguments": map[string]interface{}{},
	})

	if resp.Error == nil {
		t.Fatal("tool không tồn tại phải trả JSON-RPC error")
	}
	if resp.Error.Code != jsonRPCInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, jsonRPCInvalidParams)
	}
}

// TestServer_ToolsCall_ToolExecutionError: tool tồn tại và được phép gọi
// nhưng Execute lỗi (vd biểu thức toán không hợp lệ) → PHẢI trả về như
// "Tool Execution Error" (result.isError=true), KHÔNG PHẢI JSON-RPC protocol
// error — đúng phân biệt spec MCP yêu cầu.
func TestServer_ToolsCall_ToolExecutionError(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	_, resp := doRPC(t, s, "tools/call", "6", map[string]interface{}{
		"name":      "calculator",
		"arguments": map[string]interface{}{"expression": "1 / 0"},
	})

	if resp.Error != nil {
		t.Fatalf("lỗi thực thi tool KHÔNG được là JSON-RPC protocol error, got: %+v", resp.Error)
	}

	var result callToolResult
	if err := json.Unmarshal(marshalResult(t, resp.Result), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if !result.IsError {
		t.Error("isError phải true khi tool Execute lỗi")
	}
}

// TestServer_UnknownMethod_ReturnsMethodNotFound xác nhận method JSON-RPC lạ
// trả đúng mã chuẩn -32601 "Method not found".
func TestServer_UnknownMethod_ReturnsMethodNotFound(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	_, resp := doRPC(t, s, "resources/list", "7", nil)

	if resp.Error == nil {
		t.Fatal("method lạ phải trả JSON-RPC error")
	}
	if resp.Error.Code != jsonRPCMethodNotFound {
		t.Errorf("error code = %d, want %d (Method not found)", resp.Error.Code, jsonRPCMethodNotFound)
	}
}

// TestServer_Notification_Returns202NoBody: notification (không có "id",
// vd "notifications/initialized") phải nhận 202 Accepted, KHÔNG có body JSON.
func TestServer_Notification_Returns202NoBody(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234" // authorized() mặc định chỉ cho loopback
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want rỗng", rec.Body.String())
	}
}

// TestServer_MalformedJSON_ReturnsParseError: body không phải JSON hợp lệ →
// lỗi -32700 Parse error, HTTP 400 (transport-level, không xác định được id).
func TestServer_MalformedJSON_ReturnsParseError(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{ khong phai json"))
	req.RemoteAddr = "127.0.0.1:1234" // authorized() mặc định chỉ cho loopback
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var resp rpcServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != jsonRPCParseError {
		t.Errorf("error = %+v, want code %d", resp.Error, jsonRPCParseError)
	}
}

// TestServer_GetMethod_NotAllowed: server này không mở SSE stream nên GET
// phải trả 405 (đúng như spec Streamable HTTP cho phép trong trường hợp đó).
func TestServer_GetMethod_NotAllowed(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.RemoteAddr = "127.0.0.1:1234" // authorized() mặc định chỉ cho loopback
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// TestServer_CallerFilter_NarrowsFurtherButNeverWidens: filter caller truyền
// vào có thể THU HẸP thêm (ẩn "echo") nhưng KHÔNG BAO GIỜ có thể MỞ RỘNG để lộ
// lại tool đặc quyền — dù filter cố tình cho phép "shell.exec" đi qua, hard
// -exclude trong allowed() vẫn chặn nó (defense-in-depth: không tin caller).
func TestServer_CallerFilter_NarrowsFurtherButNeverWidens(t *testing.T) {
	reg, fake := testRegistry()
	// filter "rộng lượng" cố tình cho qua MỌI tool, kể cả shell.exec — mô
	// phỏng 1 caller vô tình/cố ý viết filter sai.
	permissive := func(name string) bool { return true }
	s := NewServer(reg, permissive)

	_, resp := doRPC(t, s, "tools/list", "8", nil)
	var result listToolsResult
	if err := json.Unmarshal(marshalResult(t, resp.Result), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	for _, tl := range result.Tools {
		if tl.Name == "shell.exec" {
			t.Fatal("filter permissive KHÔNG được làm lộ shell.exec — hard-exclude phải thắng caller filter")
		}
	}

	_, callResp := doRPC(t, s, "tools/call", "9", map[string]interface{}{
		"name": "shell.exec", "arguments": map[string]interface{}{},
	})
	if callResp.Error == nil {
		t.Fatal("tools/call shell.exec phải bị chặn dù filter permissive")
	}
	if fake.called {
		t.Fatal("fake shell.exec.Execute() không được gọi dù filter permissive")
	}

	// Filter thu hẹp thêm: chỉ cho "calculator" đi qua dù registry còn "echo".
	narrow := func(name string) bool { return name == "calculator" }
	s2 := NewServer(reg, narrow)
	_, resp2 := doRPC(t, s2, "tools/list", "10", nil)
	var result2 listToolsResult
	if err := json.Unmarshal(marshalResult(t, resp2.Result), &result2); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if len(result2.Tools) != 1 || result2.Tools[0].Name != "calculator" {
		t.Errorf("filter thu hẹp không hoạt động đúng, tools = %+v", result2.Tools)
	}
}

// marshalResult tiện ích: rpcServerResponse.Result giải mã JSON thành
// interface{} (map[string]interface{}/[]interface{}) qua vòng
// marshal→unmarshal đầu tiên trong doRPC; test cần marshal LẠI để unmarshal
// vào struct cụ thể (rpcInitializeResult/listToolsResult/callToolResult).
func marshalResult(t *testing.T, result interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	return b
}

// --- Auth (SetAPIKey / authorized) ---
//
// Thêm SAU khi phát hiện endpoint /mcp mặc định KHÔNG có auth (ai gọi HTTP
// tới server đều dùng được tool non-privileged) — không phải "risky/breaking
// behavior" theo nghĩa hẹp (không đổi response cho /chat), nhưng vẫn là 1 mặt
// tấn công mới tự động bật ngay khi server khởi động, nên cần secure-by-
// default: không cấu hình API key → chỉ chấp nhận loopback.

func TestServer_NoAPIKey_RejectsNonLoopback(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil) // apiKey rỗng (mặc định)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"1","method":"tools/list"}`))
	req.RemoteAddr = "203.0.113.7:54321" // TEST-NET-3, chắc chắn không phải loopback

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (non-loopback, no API key configured)", rec.Code)
	}
}

func TestServer_NoAPIKey_AllowsLoopback(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil) // apiKey rỗng (mặc định)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"1","method":"tools/list"}`))
	req.RemoteAddr = "127.0.0.1:54321"

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (loopback, no API key configured)", rec.Code)
	}
}

func TestServer_APIKeySet_RejectsMissingOrWrongHeader(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)
	s.SetAPIKey("super-secret")

	for name, header := range map[string]string{
		"missing header":   "",
		"wrong key":        "Bearer nope",
		"no Bearer prefix": "super-secret",
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"1","method":"tools/list"}`))
			req.RemoteAddr = "127.0.0.1:54321" // ngay cả loopback cũng phải bị chặn khi key sai
			if header != "" {
				req.Header.Set("Authorization", header)
			}

			s.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestServer_APIKeySet_AllowsCorrectHeaderFromAnywhere(t *testing.T) {
	reg, _ := testRegistry()
	s := NewServer(reg, nil)
	s.SetAPIKey("super-secret")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"1","method":"tools/list"}`))
	req.RemoteAddr = "203.0.113.7:54321" // KHÔNG loopback — key đúng vẫn phải qua được
	req.Header.Set("Authorization", "Bearer super-secret")

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (correct API key from non-loopback)", rec.Code)
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:1234", true},
		{"[::1]:1234", true},
		{"203.0.113.7:1234", false},
		{"not-an-ip:1234", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isLoopbackAddr(c.addr); got != c.want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", c.addr, got, c.want)
		}
	}
}
