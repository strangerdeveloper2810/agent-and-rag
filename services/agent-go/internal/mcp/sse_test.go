package mcp

// Test cho tính năng MỚI (user tự thêm MCP server có token xác thực): kiểm
// chứng ServerConfig.APIKey THỰC SỰ được gửi thành header
// `Authorization: Bearer <token>` khi user đã cấu hình token, và KHÔNG được
// gửi header đó khi server không có token (nhiều MCP server public/nội bộ
// không cần xác thực — gửi header rác có thể khiến chúng từ chối request).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newAuthCapturingMCPServer dựng 1 HTTP server giả nói JSON-RPC 2.0 (giống
// newFakeMCPServer ở engine_mcp_test.go) nhưng GHI LẠI header Authorization
// nhận được ở mỗi request, để test khẳng định đúng/không có header đó.
func newAuthCapturingMCPServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var gotAuthHeaders []string

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeaders = append(gotAuthHeaders, r.Header.Get("Authorization"))

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
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[]}}`, req.ID)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	srv := httptest.NewServer(mux)
	return srv, &gotAuthHeaders
}

// TestSSEClient_SendsAuthorizationHeader_WhenAPIKeySet: server có cấu hình
// token (ServerConfig.APIKey khác rỗng) → MỌI request JSON-RPC (initialize,
// notifications/initialized, tools/list) phải mang header
// `Authorization: Bearer <token>`. Đây là cách sửa lỗi cốt lõi: hầu hết MCP
// server remote thật (Notion, GitHub, Linear, Sentry...) đòi header này.
func TestSSEClient_SendsAuthorizationHeader_WhenAPIKeySet(t *testing.T) {
	srv, gotAuthHeaders := newAuthCapturingMCPServer(t)
	defer srv.Close()

	client := NewSSEClient(srv.URL, "secret-token-xyz")
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(*gotAuthHeaders) == 0 {
		t.Fatal("server không nhận được request nào")
	}
	for i, h := range *gotAuthHeaders {
		if h != "Bearer secret-token-xyz" {
			t.Errorf("request #%d: Authorization = %q, want %q", i, h, "Bearer secret-token-xyz")
		}
	}
}

// TestSSEClient_NoAuthorizationHeader_WhenAPIKeyEmpty: server KHÔNG cấu hình
// token (APIKey rỗng — trường hợp phổ biến cho MCP server public/nội bộ
// không cần xác thực) → KHÔNG được gửi header Authorization nào cả (kể cả
// rỗng) — một số server strict coi header Authorization hiện diện dù rỗng là
// lỗi định dạng.
func TestSSEClient_NoAuthorizationHeader_WhenAPIKeyEmpty(t *testing.T) {
	srv, gotAuthHeaders := newAuthCapturingMCPServer(t)
	defer srv.Close()

	client := NewSSEClient(srv.URL, "")
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(*gotAuthHeaders) == 0 {
		t.Fatal("server không nhận được request nào")
	}
	for i, h := range *gotAuthHeaders {
		if h != "" {
			t.Errorf("request #%d: Authorization = %q, want rỗng (không gửi header)", i, h)
		}
	}
}

// TestDiscoverSSE_PropagatesAPIKeyFromServerConfig: kiểm chứng đường đi ĐẦY
// ĐỦ user cấu hình → ServerConfig.APIKey → SSEClient → header Authorization,
// đi qua đúng entrypoint DiscoverSSE mà engine.go gọi (không chỉ NewSSEClient
// trực tiếp) — đây là "biên giới" thực sự giữa state của lượt chat và MCP
// client, sai ở đây thì token cấu hình đúng ở DB vẫn không tới được server.
func TestDiscoverSSE_PropagatesAPIKeyFromServerConfig(t *testing.T) {
	srv, gotAuthHeaders := newAuthCapturingMCPServer(t)
	defer srv.Close()

	reg, clients, err := DiscoverSSE(context.Background(), []ServerConfig{
		{Name: "notion", URL: srv.URL, APIKey: "token-from-db"},
	})
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()
	if err != nil {
		t.Fatalf("DiscoverSSE: %v", err)
	}
	if reg == nil {
		t.Fatal("registry = nil")
	}

	if len(*gotAuthHeaders) == 0 {
		t.Fatal("server không nhận được request nào")
	}
	for i, h := range *gotAuthHeaders {
		if h != "Bearer token-from-db" {
			t.Errorf("request #%d: Authorization = %q, want %q", i, h, "Bearer token-from-db")
		}
	}
}
