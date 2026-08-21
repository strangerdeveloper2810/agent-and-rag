package memory

import (
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Ngưỡng cho "lượt tán gẫu": câu user ngắn thì gần như chắc chắn không chứa
// fact về người dùng hay bài học kỹ thuật nào.
//
// Cố tình đặt thấp: bỏ sót một lượt đáng học chỉ làm mất một fact (lượt sau
// nhắc lại là học được), nhưng học mọi lượt tán gẫu thì nhân đôi hoá đơn LLM
// của toàn hệ thống.
const (
	trivialUserRunes = 25

	// trivialAssistantRunes không còn được worthLearning dùng — điều kiện
	// "assistant dài → học" đã bị xoá vì gần như luôn đúng với bất kỳ câu trả
	// lời có nội dung, khiến gate vô hiệu (production log cho thấy reflection
	// chạy hầu như mọi lượt). Hằng số này chỉ còn phục vụ test
	// (dựng longAnswer trong learner_gate_test.go).
	trivialAssistantRunes = 400
)

// worthLearning quyết định có đáng gọi LLM để reflection cho lượt hội thoại này.
//
// Reflection (ReflectAndExtract) là một lượt gọi LLM riêng chạy nền SAU MỖI câu
// trả lời, chỉ để trích xuất user_facts + knowledge_items. Với "xin chào" hay
// "cảm ơn nhé" thì nó luôn trả về rỗng — vẫn tốn token, vẫn ăn quota, vẫn đếm
// vào rate limit của provider.
//
// Quy tắc (bảo toàn phía học, chỉ bỏ khi CHẮC là không có gì):
//   - Câu user chứa từ khoá gợi ý có fact (dùng CHUNG keywordToKeys với
//     RecallNode — một nguồn sự thật, xem recall.go) → LUÔN học.
//   - Câu user dài (> trivialUserRunes) → học, vì câu dài thường mang yêu cầu
//     hoặc thông tin thật.
//   - Còn lại (user ngắn + không từ khoá, bất kể câu trả lời dài hay ngắn) →
//     bỏ qua.
func worthLearning(messages []provider.Message) bool {
	lastUser, _ := lastByRole(messages)

	if lastUser == "" {
		return false
	}

	lower := strings.ToLower(lastUser)
	for keyword := range keywordToKeys {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	return len([]rune(lastUser)) > trivialUserRunes
}

// lastByRole trả về nội dung tin nhắn user và assistant gần nhất.
func lastByRole(messages []provider.Message) (user, assistant string) {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if user == "" && m.Role == provider.RoleUser {
			user = m.Content
		}
		if assistant == "" && m.Role == provider.RoleAssistant {
			assistant = m.Content
		}
		if user != "" && assistant != "" {
			break
		}
	}
	return user, assistant
}
