package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/mcp"
)

// mcpTestTimeout giới hạn thời gian chờ khi test 1 MCP server remote — ngắn
// hơn hẳn timeout gọi tool thật (mcpHTTPTimeout=30s trong sse.go) vì đây chỉ
// là "còn sống không", không cần đợi lâu như 1 lượt chat thật.
const mcpTestTimeout = 10 * time.Second

// McpTestConnectionRequest là body JSON BFF gửi lên — url/apiKey lấy từ
// user_mcp_servers đã lưu (BFF tự tra theo userId, KHÔNG bao giờ để FE gửi
// thẳng token của người khác).
type McpTestConnectionRequest struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	APIKey string `json:"apiKey,omitempty"`
}

// McpTestConnectionResponse báo kết quả test — Ok=false kèm Error là kết quả
// HỢP LỆ (server cấu hình sai/không phản hồi), KHÔNG phải lỗi HTTP của chính
// endpoint này (vẫn trả 200).
type McpTestConnectionResponse struct {
	Ok        bool   `json:"ok"`
	ToolCount int    `json:"toolCount,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewMcpTestConnectionHandler tạo handler cho POST /mcp/test-connection —
// thử handshake + list tools với 1 MCP server remote, dùng lại đúng
// mcp.DiscoverSSE đang dùng cho lượt chat thật (không viết lại logic MCP
// client lần 2).
func NewMcpTestConnectionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req McpTestConnectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid json: %v"}`, err), http.StatusBadRequest)
			return
		}
		if req.URL == "" {
			http.Error(w, `{"error":"url is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), mcpTestTimeout)
		defer cancel()

		reg, clients, err := mcp.DiscoverSSE(ctx, []mcp.ServerConfig{
			{Name: req.Name, URL: req.URL, APIKey: req.APIKey},
		})
		// Chỉ TEST kết nối — không giữ client sống, đóng ngay bất kể thành công
		// hay lỗi (DiscoverSSE có thể trả về client đã connect dù ListTools lỗi).
		for _, c := range clients {
			_ = c.Close()
		}

		resp := McpTestConnectionResponse{Ok: err == nil}
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.ToolCount = len(reg.ToolDefs())
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
