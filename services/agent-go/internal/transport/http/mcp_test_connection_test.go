package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newFakeMCPServer dựng 1 HTTP server giả nói JSON-RPC 2.0 tối thiểu (giống
// pattern trong internal/mcp/sse_test.go) — trả về đúng toolCount tool khi
// "tools/list" được gọi.
func newFakeMCPServer(toolCount int) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     int64  `json:"id"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{}}`, req.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusOK)
		case "tools/list":
			tools := make([]string, toolCount)
			for i := range tools {
				tools[i] = fmt.Sprintf(`{"name":"tool_%d","description":"","inputSchema":{}}`, i)
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[%s]}}`, req.ID, joinJSON(tools))
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	return httptest.NewServer(mux)
}

func joinJSON(items []string) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ","
		}
		out += it
	}
	return out
}

func TestMcpTestConnection_MissingURL(t *testing.T) {
	body, _ := json.Marshal(McpTestConnectionRequest{Name: "x"})
	req := httptest.NewRequest(http.MethodPost, "/mcp/test-connection", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	NewMcpTestConnectionHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMcpTestConnection_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp/test-connection", bytes.NewReader([]byte("{not json")))
	rec := httptest.NewRecorder()

	NewMcpTestConnectionHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

// Server thật (giả lập) trả lời đúng -- ok=true kèm toolCount, HTTP 200.
func TestMcpTestConnection_Success(t *testing.T) {
	srv := newFakeMCPServer(3)
	defer srv.Close()

	body, _ := json.Marshal(McpTestConnectionRequest{Name: "test", URL: srv.URL})
	req := httptest.NewRequest(http.MethodPost, "/mcp/test-connection", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	NewMcpTestConnectionHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp McpTestConnectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Ok || resp.ToolCount != 3 || resp.Error != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// Server không tồn tại/không phản hồi -- KẾT QUẢ TEST là ok=false (không phải
// lỗi HTTP của chính endpoint) — vẫn trả 200, để caller (BFF/FE) hiển thị lý
// do thất bại cho user thay vì 1 lỗi 5xx chung chung.
func TestMcpTestConnection_UnreachableServer_ReturnsOkFalse(t *testing.T) {
	body, _ := json.Marshal(McpTestConnectionRequest{
		Name: "dead",
		URL:  "http://127.0.0.1:1", // port 1 -- chắc chắn không ai lắng nghe
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/test-connection", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	NewMcpTestConnectionHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 (test result, not HTTP error), got %d", rec.Code)
	}
	var resp McpTestConnectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ok || resp.Error == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
