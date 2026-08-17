package tools

import "github.com/ai-agent-tut/agent-go/internal/provider"

// privilegedTools là các tool CHỈ chủ hệ thống được dùng, vì chúng tác động lên
// MÁY CHẠY AGENT (không phải máy của người dùng) và KHÔNG hề scope theo tenant:
//
//   - file.read/file.search: đọc filesystem của server. AllowedPaths mặc định là
//     [".", $HOME] → bất kỳ người dùng nào cũng có thể đọc .env chứa toàn bộ API
//     key, SSH key, hoặc dữ liệu người dùng khác.
//   - file.write: ghi file lên server.
//   - shell.exec: chạy lệnh tuỳ ý trên server.
//   - git: đọc/thao tác repo trên server.
//
// Khi JARVIS chỉ chạy 1 người trên máy cá nhân thì vô hại. Khi mở cho nhiều
// người dùng thì đây là lỗ hổng nghiêm trọng — cả registry `general` (mọi user
// vào mặc định) lẫn `code` đều đang có nhóm tool này.
//
// version KHÔNG nằm trong danh sách: nó chỉ tra phiên bản package npm/GitHub qua
// web, không đụng tới server. notes.* cũng không: chúng đã được scope theo tenant.
var privilegedTools = map[string]bool{
	"file.read":   true,
	"file.search": true,
	"file.write":  true,
	"shell.exec":  true,
	"git":         true,
}

// IsPrivilegedTool cho biết tool có thuộc nhóm chỉ-chủ-hệ-thống hay không.
func IsPrivilegedTool(name string) bool { return privilegedTools[name] }

// PrivilegedToolNames trả về danh sách tên tool đặc quyền (để log/tài liệu).
func PrivilegedToolNames() []string {
	out := make([]string, 0, len(privilegedTools))
	for name := range privilegedTools {
		out = append(out, name)
	}
	return out
}

// defaultTenantID khớp middleware.GetTenantID khi request không có X-Tenant-ID
// (chế độ single-user, chạy local không qua auth).
const defaultTenantID = "default"

// IsOwnerTenant quyết định tenant có được dùng nhóm tool đặc quyền hay không.
//
// FAIL CLOSED có chủ đích: khi owners rỗng (chưa cấu hình OWNER_TENANT_IDS), CHỈ
// tenant "default" — tức chạy local không có auth — được coi là chủ. Mọi tenant
// thật đều KHÔNG có đặc quyền. Chọn hướng này vì hậu quả của việc quên cấu hình
// là cho người lạ quyền chạy shell trên server, còn hậu quả của fail-closed chỉ
// là chủ máy phải thêm 1 dòng .env.
func IsOwnerTenant(tenantID string, owners []string) bool {
	if len(owners) == 0 {
		return tenantID == "" || tenantID == defaultTenantID
	}
	for _, o := range owners {
		if o != "" && o == tenantID {
			return true
		}
	}
	return false
}

// StripPrivilegedTools loại các tool đặc quyền khỏi danh sách gửi cho LLM.
// Dùng cho tenant không phải chủ — model không nên BIẾT là có những tool này,
// nếu không nó sẽ cố gọi rồi nhận lỗi và làm rối câu trả lời.
func StripPrivilegedTools(defs []provider.ToolDef) []provider.ToolDef {
	out := make([]provider.ToolDef, 0, len(defs))
	for _, d := range defs {
		if privilegedTools[d.Name] {
			continue
		}
		out = append(out, d)
	}
	return out
}
