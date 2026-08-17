package tools

import (
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Nhóm tool đặc quyền tác động lên MÁY CHẠY AGENT (AllowedPaths mặc định gồm
// $HOME của server) và không scope theo tenant. Khi mở JARVIS cho nhiều người
// dùng, để lọt nhóm này nghĩa là bất kỳ ai cũng đọc được .env chứa toàn bộ API
// key của server.
func TestIsPrivilegedTool(t *testing.T) {
	for _, name := range []string{"file.read", "file.write", "file.search", "shell.exec", "git"} {
		if !IsPrivilegedTool(name) {
			t.Errorf("%q PHẢI là tool đặc quyền", name)
		}
	}
	// Những tool này an toàn cho mọi người dùng: chỉ gọi web hoặc đã scope tenant.
	for _, name := range []string{
		"web.search", "web.fetch", "rag.search", "rag.read", "rag.list",
		"memory.save", "memory.recall", "notes.search", "notes.create",
		"calculator", "datetime", "translate", "echo", "version",
	} {
		if IsPrivilegedTool(name) {
			t.Errorf("%q KHÔNG nên bị coi là đặc quyền (chặn oan người dùng phổ thông)", name)
		}
	}
}

// FAIL CLOSED: chưa cấu hình OWNER_TENANT_IDS thì CHỈ chế độ local (tenant
// "default", tức request không có header X-Tenant-ID) được coi là chủ. Nếu làm
// ngược lại (rỗng = ai cũng là chủ), quên cấu hình một lần là mở quyền chạy
// shell trên server cho người lạ.
func TestIsOwnerTenant_FailsClosedWhenUnconfigured(t *testing.T) {
	if !IsOwnerTenant("default", nil) {
		t.Error("tenant \"default\" (local, không auth) phải là chủ khi chưa cấu hình")
	}
	if !IsOwnerTenant("", nil) {
		t.Error("tenant rỗng phải được coi như \"default\"")
	}
	// Tenant thật (đã đăng nhập) KHÔNG được mặc định là chủ.
	if IsOwnerTenant("327533c0-16ec-49de-b1d8-40c55be7f81e", nil) {
		t.Error("tenant thật KHÔNG được là chủ khi chưa cấu hình OWNER_TENANT_IDS (phải fail closed)")
	}
}

func TestIsOwnerTenant_ExplicitList(t *testing.T) {
	owners := []string{"owner-1", "owner-2"}

	for _, id := range owners {
		if !IsOwnerTenant(id, owners) {
			t.Errorf("%q có trong danh sách chủ nhưng bị từ chối", id)
		}
	}
	for _, id := range []string{"nguoi-la", "default", ""} {
		if IsOwnerTenant(id, owners) {
			t.Errorf("%q KHÔNG có trong danh sách chủ nhưng được cấp đặc quyền", id)
		}
	}
	// Chuỗi rỗng trong cấu hình không được biến tenant rỗng thành chủ.
	if IsOwnerTenant("", []string{""}) {
		t.Error("phần tử rỗng trong OWNER_TENANT_IDS không được cấp đặc quyền cho tenant rỗng")
	}
}

func TestStripPrivilegedTools(t *testing.T) {
	defs := []provider.ToolDef{
		{Name: "web.search"}, {Name: "file.read"}, {Name: "rag.list"},
		{Name: "shell.exec"}, {Name: "memory.save"}, {Name: "git"},
		{Name: "file.write"}, {Name: "file.search"},
	}

	got := StripPrivilegedTools(defs)

	names := make(map[string]bool, len(got))
	for _, d := range got {
		names[d.Name] = true
	}
	for _, banned := range []string{"file.read", "file.write", "file.search", "shell.exec", "git"} {
		if names[banned] {
			t.Errorf("%q vẫn còn trong tool list sau khi strip", banned)
		}
	}
	for _, kept := range []string{"web.search", "rag.list", "memory.save"} {
		if !names[kept] {
			t.Errorf("%q bị strip oan", kept)
		}
	}
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
}

func TestStripPrivilegedTools_EmptyAndAllPrivileged(t *testing.T) {
	if got := StripPrivilegedTools(nil); len(got) != 0 {
		t.Errorf("nil input → len %d, want 0", len(got))
	}
	allPriv := []provider.ToolDef{{Name: "shell.exec"}, {Name: "git"}}
	if got := StripPrivilegedTools(allPriv); len(got) != 0 {
		t.Errorf("toàn tool đặc quyền → len %d, want 0", len(got))
	}
}
