package http

// Test cho tính năng MỚI (user MCP servers + skills management): BFF
// (apps/api) forward mcpServers/disabledSkills/customSkills trong body JSON
// POST /chat — ChatHandler.ServeHTTP phải map ĐÚNG các field này sang
// agent.RunInput để Engine.Run dùng (discovery MCP tool, lọc skill bị tắt,
// inject custom skill vào system prompt). Thiếu test này, một lần đổi tên
// field JSON hoặc quên gán trong ServeHTTP sẽ khiến toàn bộ tính năng "âm
// thầm" không hoạt động — không lỗi, không panic, chỉ đơn giản cấu hình của
// user bị bỏ qua.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatHandler_MapsMcpServersDisabledSkillsCustomSkills(t *testing.T) {
	runner := &fakeRunner{}
	body := `{
		"userMessage": "hi",
		"mcpServers": [{"name": "weather", "url": "https://mcp.example.com/sse", "apiKey": "secret-key", "transport": "http"}],
		"disabledSkills": ["code-review", "debug"],
		"customSkills": [{"name": "invoice", "description": "mô tả", "whenToUse": "khi nào", "content": "nội dung", "triggers": ["hoá đơn"]}]
	}`

	rec := httptest.NewRecorder()
	NewChatHandler(runner).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	in := runner.gotIn

	if len(in.McpServers) != 1 {
		t.Fatalf("McpServers = %+v, want 1 phần tử", in.McpServers)
	}
	if in.McpServers[0].Name != "weather" ||
		in.McpServers[0].URL != "https://mcp.example.com/sse" ||
		in.McpServers[0].APIKey != "secret-key" ||
		in.McpServers[0].Transport != "http" {
		t.Errorf("McpServers[0] = %+v", in.McpServers[0])
	}

	if len(in.DisabledSkills) != 2 || in.DisabledSkills[0] != "code-review" || in.DisabledSkills[1] != "debug" {
		t.Errorf("DisabledSkills = %v, want [code-review debug]", in.DisabledSkills)
	}

	if len(in.CustomSkills) != 1 {
		t.Fatalf("CustomSkills = %+v, want 1 phần tử", in.CustomSkills)
	}
	cs := in.CustomSkills[0]
	if cs.Name != "invoice" || cs.Description != "mô tả" || cs.WhenToUse != "khi nào" ||
		cs.Content != "nội dung" || len(cs.Triggers) != 1 || cs.Triggers[0] != "hoá đơn" {
		t.Errorf("CustomSkills[0] = %+v", cs)
	}
}

// Khi client KHÔNG gửi mcpServers/disabledSkills/customSkills (field vắng mặt
// trong JSON — trường hợp phổ biến nhất, user chưa cấu hình gì), ChatHandler
// KHÔNG được để giá trị gây panic khi Engine.Run/nodeModel lặp qua các slice
// này (len() trên nil slice là an toàn trong Go, nhưng test này khoá lại
// tường minh — tránh một thay đổi tương lai đổi kiểu dữ liệu gây panic).
func TestChatHandler_EmptyMcpSkillsFieldsDoNotBreakRequest(t *testing.T) {
	runner := &fakeRunner{}
	rec := httptest.NewRecorder()
	NewChatHandler(runner).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"userMessage":"hi"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if len(runner.gotIn.McpServers) != 0 {
		t.Errorf("McpServers = %+v, want rỗng", runner.gotIn.McpServers)
	}
	if len(runner.gotIn.DisabledSkills) != 0 {
		t.Errorf("DisabledSkills = %v, want rỗng", runner.gotIn.DisabledSkills)
	}
	if len(runner.gotIn.CustomSkills) != 0 {
		t.Errorf("CustomSkills = %+v, want rỗng", runner.gotIn.CustomSkills)
	}
}
