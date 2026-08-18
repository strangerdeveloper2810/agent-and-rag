package skills

import "strings"

// MaxPromptBytes là ngân sách cho phần thân skill được chèn vào system prompt.
//
// Vì sao cần trần: SKILL.md dao động 2.000–11.600 byte (devops ~3.300 token,
// learning-tutor ~2.000 token) và toàn bộ nội dung được chèn lại MỖI lượt chat
// có skill khớp — State.activatedSkills chỉ chặn chèn lặp trong CÙNG một lượt
// chạy, lượt chat sau là State mới nên chèn lại từ đầu.
//
// 4.500 byte ≈ 1.285 token: đủ cho phần triết lý + phương pháp chính của một
// skill (những section đầu, nơi tác giả đặt hướng dẫn cốt lõi), cắt phần đuôi
// thường là ví dụ mở rộng và bảng tra cứu.
const MaxPromptBytes = 4500

// truncationNote nói THẲNG với model là nội dung bị lược, thay vì để nó tưởng
// skill kết thúc ở đó (im lặng cắt dễ làm model kết luận sai về phạm vi skill).
const truncationNote = "\n\n[Phần sau của kỹ năng đã được lược bỏ để tiết kiệm ngữ cảnh — dùng phần trên là đủ.]"

// PromptBody trả phần thân skill đã gọt vừa MaxPromptBytes để chèn vào prompt.
//
// Cắt theo RANH GIỚI SECTION (dòng bắt đầu bằng "## ") chứ không cắt theo số
// byte: một hướng dẫn bị chặt giữa câu còn tệ hơn là không có hướng dẫn đó.
// Phần mở đầu (tiêu đề "# ..." + đoạn intro trước section đầu tiên) luôn được
// giữ. Nếu ngay section đầu đã vượt ngân sách thì cắt ở ranh giới DÒNG.
func (s *Skill) PromptBody() string {
	return truncateToSections(s.Content, MaxPromptBytes)
}

func truncateToSections(content string, maxBytes int) string {
	if len(content) <= maxBytes {
		return content
	}

	lines := strings.Split(content, "\n")

	var (
		kept []string
		size int
		// bestCut là số dòng cần giữ để dừng ở ranh giới section VÀ đã gồm ít
		// nhất một section trọn vẹn. -1 = chưa có.
		bestCut      = -1
		seenSections int
	)

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			seenSections++
			// Ranh giới của section thứ 2 trở đi mới đáng cắt: cắt ở ranh giới
			// section ĐẦU TIÊN sẽ chỉ còn lại tiêu đề + intro, tức mất hết
			// hướng dẫn — thà giữ một phần section đầu còn hơn.
			if seenSections >= 2 {
				bestCut = len(kept)
			}
		}

		next := size + len(line) + 1 // +1 cho newline
		if next > maxBytes {
			break
		}
		kept = append(kept, line)
		size = next
	}

	if bestCut > 0 {
		kept = kept[:bestCut]
	}

	out := strings.TrimRight(strings.Join(kept, "\n"), "\n ")
	if out == "" {
		// Không có cả một dòng nào vừa (dòng đầu quá dài) → cắt cứng theo byte,
		// nhưng theo rune để không làm hỏng ký tự tiếng Việt.
		runes := []rune(content)
		for len(string(runes)) > maxBytes && len(runes) > 0 {
			runes = runes[:len(runes)-1]
		}
		out = string(runes)
	}
	return out + truncationNote
}
