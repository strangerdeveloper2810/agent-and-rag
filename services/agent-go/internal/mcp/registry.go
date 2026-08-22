package mcp

import "github.com/ai-agent-tut/agent-go/internal/tools"

// NewDefaultToolRegistry dựng 1 registry TỐI GIẢN, ĐỘC LẬP dành riêng cho MCP
// server (cmd/server/main.go route "POST /mcp") — CỐ TÌNH không tái dùng
// codeRegistry/researchRegistry/generalRegistry build trong
// cmd/server/main.go's buildRegistries(), vì hai lý do:
//
//  1. Tránh phải đổi chữ ký buildRegistries/newHTTPHandler (2 agent khác đang
//     sửa cmd/server/main.go song song ở vùng khác — SetInterruptStore/cost
//     ledger, router — đổi chữ ký hàm dùng chung sẽ lan ra nhiều chỗ ngoài
//     phạm vi được giao).
//  2. Whitelist TƯỜNG MINH: ai đó thêm tool mới vào code/research/general
//     registry sau này KHÔNG tự động lộ ra qua MCP — phải sửa chính hàm này
//     mới expose thêm được, tránh rò rỉ tool mới quên chặn.
//
// Chỉ chọn tool KHÔNG cần cfg/store bên ngoài (constructor không đối số hoặc
// nhận nil an toàn) và ĐÃ được xác nhận không đặc quyền qua
// tools.IsPrivilegedTool (Server.allowed vẫn tự kiểm tra lại — xem
// server.go — nên danh sách dưới đây KHÔNG phải là lớp phòng vệ duy nhất).
func NewDefaultToolRegistry() *tools.Registry {
	reg := tools.NewRegistry()
	reg.Register(tools.NewCalculatorTool())
	reg.Register(tools.NewDateTimeTool())
	reg.Register(tools.NewEchoTool())
	reg.Register(tools.NewVersionTool())
	reg.Register(tools.NewWebSearchTool(nil))
	reg.Register(tools.NewWebFetchTool(nil))
	reg.Register(tools.NewNotesSearchTool("."))
	reg.Register(tools.NewNotesCreateTool("."))
	return reg
}
