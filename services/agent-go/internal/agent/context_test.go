package agent

import (
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/skills"
)

// JARVIS đang được mở cho NHIỀU người dùng khác nhau, nên system prompt không
// được mặc định gọi ai là "sir", không được nhắc tên riêng "Tony" (theme Iron
// Man cũ), và không được giả định "chạy trên máy cá nhân của người dùng" —
// những điều đó vừa sai với thực tế multi-user, vừa gây khó chịu cho người dùng
// lạ.
func TestBuildSystemPrompt_NeutralPersona(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil)

	// "sir" chỉ được phép xuất hiện trong chính câu CẤM dùng nó.
	lines := strings.Split(prompt, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "sir") {
			continue
		}
		if !strings.Contains(lower, "không gọi 'sir'") {
			t.Errorf("prompt còn dùng 'sir' ngoài câu cấm: %q", line)
		}
	}

	for _, banned := range []string{"Tony", "máy cá nhân của người dùng"} {
		if strings.Contains(prompt, banned) {
			t.Errorf("prompt còn chứa %q — không phù hợp khi phục vụ nhiều người dùng", banned)
		}
	}

	// Phải nói rõ là trợ lý VẠN NĂNG, không phải agent coding.
	for _, want := range []string{"TRỢ LÝ VẠN NĂNG", "KHÔNG mặc định người dùng là lập trình viên"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt thiếu %q", want)
		}
	}
}

// rag.list phải được nhắc trong mục [CÔNG CỤ], nếu không model sẽ tiếp tục
// brute-force rag.search khi người dùng hỏi "tôi có tài liệu gì" (hành vi đã
// thấy trong log dev, bỏ sót một nửa knowledge base).
func TestBuildSystemPrompt_MentionsRAGList(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil)

	if !strings.Contains(prompt, "rag.list") {
		t.Error("prompt không nhắc rag.list — model sẽ không biết có tool liệt kê tài liệu")
	}
}

// Danh sách skill (nếu có) phải được liệt kê để model biết mình có kỹ năng gì —
// nhưng chỉ TÊN, không kèm description.
//
// Trước đây test này đòi cả description. Đã đổi có chủ ý: skill do
// skills.Loader.MatchSkill (code Go, khớp tên + trigger) chọn, model không tham
// gia, nên description gửi kèm mọi request chỉ tốn token. Xem buildSkillCatalogue.
func TestBuildSystemPrompt_ListsSkills(t *testing.T) {
	prompt := BuildSystemPrompt(nil, []skills.SkillSummary{
		{Name: "personal-finance", Description: "Quản lý chi tiêu"},
	})

	if !strings.Contains(prompt, "personal-finance") {
		t.Error("prompt thiếu tên skill được truyền vào")
	}
	if strings.Contains(prompt, "Quản lý chi tiêu") {
		t.Error("description của skill không được gửi trong mọi request")
	}
}
