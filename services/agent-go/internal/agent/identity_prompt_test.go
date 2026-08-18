package agent

import (
	"strings"
	"testing"
)

// Bug thật từ production: mọi lượt trả lời đều mở đầu bằng "Tôi là J.A.R.V.I.S.,
// trợ lý AI của bạn." khiến người dùng tưởng server mất session và mở hội thoại
// mới. Nguyên nhân: prompt viết "Khi được hỏi 'bạn là ai': LUÔN trả lời '...'"
// nằm dưới tiêu đề "[DANH TÍNH — TUYỆT ĐỐI TUÂN THỦ]" — model nhỏ (flash-lite)
// đọc "LUÔN" + "TUYỆT ĐỐI TUÂN THỦ" thành mệnh lệnh vô điều kiện.
//
// Nhóm test này khoá lại cách diễn đạt: điều kiện phải đứng TRƯỚC, và phải có
// câu cấm tự giới thiệu lại.
func TestSystemPrompt_ChiGioiThieuKhiDuocHoi(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil, "vi")

	if !strings.Contains(prompt, "CHỈ giới thiệu bản thân khi người dùng hỏi trực tiếp") {
		t.Error("prompt thiếu điều kiện 'CHỈ ... khi được hỏi' cho phần tự giới thiệu")
	}

	// Câu tự giới thiệu vẫn phải còn — người dùng hỏi "bạn là ai" thì cần trả lời
	// đúng danh tính, không phải bỏ hẳn.
	if !strings.Contains(prompt, "Tôi là J.A.R.V.I.S., trợ lý AI của bạn.") {
		t.Error("prompt mất luôn câu trả lời danh tính — hỏi 'bạn là ai' sẽ trả lời sai")
	}

	// Cách diễn đạt cũ (nguyên nhân bug) không được quay lại.
	if strings.Contains(prompt, "Khi được hỏi 'bạn là ai': luôn trả lời") {
		t.Error("prompt quay về cách viết cũ: 'luôn trả lời' đứng sau điều kiện → model coi là vô điều kiện")
	}
}

func TestSystemPrompt_CamTuGioiThieuLaiMoiLuot(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil, "vi")

	for _, want := range []string{
		"KHÔNG tự giới thiệu",
		"không chào lại",
		"đầu câu trả lời",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt thiếu phần cấm %q — model sẽ tự giới thiệu lại mỗi lượt", want)
		}
	}
}

// Prompt tiếng Anh cũng phải mang cùng ràng buộc (phần danh tính là chung, chỉ
// mục [QUY TẮC] mới đổi theo lang).
func TestSystemPrompt_RangBuocDanhTinhApDungCaKhiLangEn(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil, "en")

	if !strings.Contains(prompt, "CHỈ giới thiệu bản thân khi người dùng hỏi trực tiếp") {
		t.Error("lang=en mất ràng buộc chỉ-giới-thiệu-khi-được-hỏi")
	}
	if !strings.Contains(prompt, "ALWAYS respond in English") {
		t.Error("lang=en thiếu quy tắc trả lời bằng tiếng Anh")
	}
}
