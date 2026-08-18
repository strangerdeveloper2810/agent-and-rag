package agent

// Test cho tính năng MỚI (user MCP servers): RunInput.McpServers rỗng/nil
// KHÔNG được làm vỡ luồng Engine.Run; MCP server cấu hình đúng phải discovery
// tool và cấp cho LLM; discovery lỗi (server không phản hồi) KHÔNG được làm
// hỏng cả lượt chạy — chỉ bỏ qua MCP tool cho lượt đó (xem engine.go: lỗi
// discovery chỉ log warning, không return error).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// newFakeMCPServer dựng 1 HTTP server giả nói JSON-RPC 2.0 qua Streamable
// HTTP (transport SSE remote mà mcp.SSEClient dùng): trả lời initialize,
// notifications/initialized, và tools/list với đúng 1 tool cho trước.
func newFakeMCPServer(t *testing.T, toolName, toolDesc string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
			ID     int64  `json:"id"`
		}
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{}}`, req.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusOK)
		case "tools/list":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":%q,"description":%q,"inputSchema":{"type":"object"}}]}}`,
				req.ID, toolName, toolDesc)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	return httptest.NewServer(mux)
}

// TestEngineRun_NilMcpServers_DoesNotBreakFlow: McpServers = nil (mặc định
// khi user chưa cấu hình MCP server nào — trường hợp phổ biến nhất) không
// được gây lỗi/panic, lượt chạy phải hoàn tất bình thường với event done.
func TestEngineRun_NilMcpServers_DoesNotBreakFlow(t *testing.T) {
	e := NewEngine(provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "chào"},
		provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishStop},
	), tools.NewRegistry())

	events, _, err := collectEvents(t, e, RunInput{UserMessage: "hi", McpServers: nil})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hasEvent(events, "done") == nil {
		t.Fatal("thiếu event done — McpServers=nil không được làm vỡ luồng")
	}
	if hasEvent(events, "error") != nil {
		t.Error("không được phát event error khi McpServers=nil")
	}
}

// TestEngineRun_EmptyMcpServers_DoesNotBreakFlow: slice RỖNG (không phải nil,
// vd JSON []) cũng phải xử lý an toàn giống hệt nil.
func TestEngineRun_EmptyMcpServers_DoesNotBreakFlow(t *testing.T) {
	e := NewEngine(provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "chào"},
		provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishStop},
	), tools.NewRegistry())

	events, _, err := collectEvents(t, e, RunInput{UserMessage: "hi", McpServers: []McpServer{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hasEvent(events, "done") == nil {
		t.Fatal("thiếu event done — McpServers=[] không được làm vỡ luồng")
	}
}

// TestEngineRun_McpServersDiscoverySuccess_ToolAvailableToModel là test giá
// trị nhất cho tính năng MCP server: discovery THÀNH CÔNG phải thực sự cấp
// tool đó cho LLM (qua toolDefs gửi trong GenerateRequest), namespace đúng
// theo server (mcp__<server>__<tool>) — đây là bằng chứng "MCP server không
// chỉ được lưu DB mà THỰC SỰ hoạt động" xuyên suốt Engine.Run → nodeModel.
func TestEngineRun_McpServersDiscoverySuccess_ToolAvailableToModel(t *testing.T) {
	srv := newFakeMCPServer(t, "get_weather", "Lấy thời tiết hiện tại")
	defer srv.Close()

	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	e := NewEngine(fake, tools.NewRegistry())

	events, _, err := collectEvents(t, e, RunInput{
		UserMessage: "thời tiết hôm nay thế nào",
		McpServers:  []McpServer{{Name: "weather", URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hasEvent(events, "done") == nil {
		t.Fatal("thiếu event done")
	}

	var found bool
	for _, d := range fake.LastRequest.Tools {
		if d.Name == "mcp__weather__get_weather" {
			found = true
		}
	}
	if !found {
		names := make([]string, len(fake.LastRequest.Tools))
		for i, d := range fake.LastRequest.Tools {
			names[i] = d.Name
		}
		t.Errorf("tool gửi cho LLM = %v, thiếu mcp__weather__get_weather (MCP tool phải luôn có mặt)", names)
	}
}

// TestEngineRun_McpServersDiscoveryFailure_DoesNotBreakRun: server không phản
// hồi được (URL sai/connection refused) → discovery lỗi, nhưng Engine.Run vẫn
// PHẢI hoàn tất lượt chat bình thường (chỉ là không có MCP tool nào) — đúng
// hành vi engine.go: lỗi discovery chỉ slog.Warn, không return error.
func TestEngineRun_McpServersDiscoveryFailure_DoesNotBreakRun(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	e := NewEngine(fake, tools.NewRegistry())

	events, _, err := collectEvents(t, e, RunInput{
		UserMessage: "hi",
		// Cổng không server nào lắng nghe → connect thất bại ngay.
		McpServers: []McpServer{{Name: "broken", URL: "http://127.0.0.1:1"}},
	})
	if err != nil {
		t.Fatalf("Run không được lỗi khi MCP discovery thất bại: %v", err)
	}
	if hasEvent(events, "done") == nil {
		t.Fatal("thiếu event done — discovery lỗi không được chặn cả lượt chạy")
	}
	if hasEvent(events, "error") != nil {
		t.Error("discovery lỗi không được phát event error tới client (chỉ log nội bộ)")
	}

	for _, d := range fake.LastRequest.Tools {
		if strings.HasPrefix(d.Name, "mcp__") {
			t.Errorf("không được có MCP tool nào khi discovery thất bại, got %q", d.Name)
		}
	}
}

// TestEngineRun_McpServers_BlankURLSkipped: entry có URL rỗng (chuỗi trắng)
// phải được BỎ QUA êm thấm bởi mcp.DiscoverSSE (xem strings.TrimSpace check),
// không được cố connect tới URL rỗng rồi lỗi làm vỡ discovery các server khác
// trong cùng danh sách.
func TestEngineRun_McpServers_BlankURLSkipped(t *testing.T) {
	srv := newFakeMCPServer(t, "search", "Tìm kiếm")
	defer srv.Close()

	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	e := NewEngine(fake, tools.NewRegistry())

	events, _, err := collectEvents(t, e, RunInput{
		UserMessage: "hi",
		McpServers: []McpServer{
			{Name: "blank", URL: "   "},
			{Name: "ok-server", URL: srv.URL},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hasEvent(events, "done") == nil {
		t.Fatal("thiếu event done")
	}

	var found bool
	for _, d := range fake.LastRequest.Tools {
		if d.Name == "mcp__ok-server__search" {
			found = true
		}
	}
	if !found {
		t.Error("server hợp lệ đứng SAU server URL rỗng vẫn phải discovery thành công")
	}
}
