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
	prompt := BuildSystemPrompt(nil, nil, "")

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
	prompt := BuildSystemPrompt(nil, nil, "")

	if !strings.Contains(prompt, "rag.list") {
		t.Error("prompt không nhắc rag.list — model sẽ không biết có tool liệt kê tài liệu")
	}
}

// Danh sách skill (nếu có) phải được liệt kê để model biết mình có kỹ năng gì.
func TestBuildSystemPrompt_ListsSkills(t *testing.T) {
	prompt := BuildSystemPrompt(nil, []skills.SkillSummary{
		{Name: "personal-finance", Description: "Quản lý chi tiêu"},
	}, "")

	if !strings.Contains(prompt, "personal-finance") {
		t.Error("prompt thiếu tên skill được truyền vào")
	}
	if !strings.Contains(prompt, "Quản lý chi tiêu") {
		t.Error("prompt thiếu description của skill")
	}
}

// TestBuildSystemPrompt_Lang khoá hành vi i18n: FE cho phép user chọn ngôn
// ngữ UI (vi/en) và JARVIS phải trả lời đúng ngôn ngữ đó. lang="" (chưa từng
// truyền trước đây) BẮT BUỘC giữ nguyên hành vi cũ — mặc định tiếng Việt —
// để không phá vỡ mọi caller hiện có chưa biết về tham số lang mới.
func TestBuildSystemPrompt_Lang(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "english", lang: "en", want: "ALWAYS respond in English"},
		{name: "vietnamese explicit", lang: "vi", want: "LUÔN trả lời bằng tiếng Việt"},
		{name: "empty defaults to vietnamese (backward compat)", lang: "", want: "LUÔN trả lời bằng tiếng Việt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := BuildSystemPrompt(nil, nil, tt.lang)
			if !strings.Contains(prompt, tt.want) {
				t.Errorf("lang=%q: prompt thiếu %q", tt.lang, tt.want)
			}
		})
	}

	// lang="en" KHÔNG được để sót câu tiếng Việt (tránh 2 chỉ dẫn ngôn ngữ
	// mâu thuẫn nhau trong cùng system prompt).
	enPrompt := BuildSystemPrompt(nil, nil, "en")
	if strings.Contains(enPrompt, "LUÔN trả lời bằng tiếng Việt") {
		t.Error("lang=en nhưng prompt vẫn còn chỉ dẫn 'LUÔN trả lời bằng tiếng Việt'")
	}
}
