package memory

import (
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Ngưỡng cho "lượt tán gẫu": câu user ngắn VÀ câu trả lời ngắn thì gần như chắc
// chắn không chứa fact về người dùng hay bài học kỹ thuật nào.
//
// Cố tình đặt thấp và yêu cầu CẢ HAI điều kiện: bỏ sót một lượt đáng học chỉ làm
// mất một fact (lượt sau nhắc lại là học được), nhưng học mọi lượt tán gẫu thì
// nhân đôi hoá đơn LLM của toàn hệ thống.
const (
	trivialUserRunes      = 25
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
//   - Câu trả lời dài (> trivialAssistantRunes) → học, vì câu trả lời dài
//     thường là giải pháp kỹ thuật đáng lưu thành knowledge item.
//   - Còn lại (user ngắn + trả lời ngắn + không từ khoá) → bỏ qua.
func worthLearning(messages []provider.Message) bool {
	lastUser, lastAssistant := lastByRole(messages)

	if lastUser == "" {
		return false
	}

	lower := strings.ToLower(lastUser)
	for keyword := range keywordToKeys {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	if len([]rune(lastUser)) > trivialUserRunes {
		return true
	}
	return len([]rune(lastAssistant)) > trivialAssistantRunes
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
