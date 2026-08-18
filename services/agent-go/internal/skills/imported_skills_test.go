package skills

import (
	"strings"
	"testing"
)

// Hai skill dưới đây được adapt từ obra/superpowers (MIT). Nhóm test này bảo đảm
// việc thêm chúng KHÔNG làm hỏng hai thứ:
//   - chúng kích hoạt đúng khi cần;
//   - chúng không giành chỗ của skill sẵn có (debug, code-review, planning...),
//     vì kích hoạt sai skill vừa tốn ~1.285 token vừa cho câu trả lời lệch.

func loaderOrSkip(t *testing.T) *Loader {
	t.Helper()
	l, err := NewLoader("../../skills")
	if err != nil {
		t.Skipf("bỏ qua: %v", err)
	}
	return l
}

func TestImportedSkills_DuocNapVaHopLe(t *testing.T) {
	l := loaderOrSkip(t)

	for _, name := range []string{"test-driven-development", "verification-before-completion"} {
		sk := l.LoadSkill(name)
		if sk == nil {
			t.Fatalf("skill %q không được nạp", name)
		}
		if len(sk.Triggers) == 0 {
			t.Errorf("skill %q thiếu triggers tiếng Việt — MatchSkill sẽ gần như không bắt được", name)
		}
		if len(sk.Tools) == 0 {
			t.Errorf("skill %q thiếu tools — node_model sẽ không cấp tool cần thiết", name)
		}
		// Trần token: nội dung chèn vào prompt phải vừa ngân sách, nếu không nó bị
		// gọt và mất phần cuối.
		if body := sk.PromptBody(); len(body) > MaxPromptBytes {
			t.Errorf("skill %q: %d byte, vượt trần %d — sẽ bị gọt", name, len(body), MaxPromptBytes)
		}
		// Giữ attribution: nội dung adapt từ repo MIT.
		if !strings.Contains(sk.Content, "superpowers") {
			t.Errorf("skill %q thiếu ghi nguồn (MIT attribution)", name)
		}
	}
}

func TestImportedSkills_KichHoatDung(t *testing.T) {
	l := loaderOrSkip(t)

	tests := []struct {
		input string
		want  string
	}{
		{"viết unit test cho hàm này", "test-driven-development"},
		{"lam theo tdd giup minh", "test-driven-development"},
		{"thêm test case cho phần thanh toán", "test-driven-development"},
		{"kiểm tra lại xem đã chạy được chưa", "verification-before-completion"},
		{"chay thu lai giup minh", "verification-before-completion"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := l.MatchSkill(tc.input)
			name := ""
			if got != nil {
				name = got.Name
			}
			if name != tc.want {
				t.Errorf("MatchSkill(%q) = %q, want %q", tc.input, name, tc.want)
			}
		})
	}
}

// Phần quan trọng nhất: skill mới KHÔNG được chiếm chỗ skill sẵn có.
func TestImportedSkills_KhongChiemChoSkillCoSan(t *testing.T) {
	l := loaderOrSkip(t)

	tests := []struct {
		input string
		want  string
		why   string
	}{
		{"app bị lỗi khi bấm nút đăng nhập", "debug", "câu báo bug phải vào debug, không vào TDD"},
		{"sửa lỗi crash khi mở file", "debug", "sửa lỗi là debug"},
		{"review code giúp mình file này", "code-review", "review vẫn phải vào code-review"},
		{"lên kế hoạch làm tính năng thanh toán", "planning", "kế hoạch vẫn phải vào planning"},
		{"giải thích cho mình khái niệm closure", "learning-tutor", "câu hỏi học tập không được rơi vào skill mới"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := l.MatchSkill(tc.input)
			name := ""
			if got != nil {
				name = got.Name
			}
			if name != tc.want {
				t.Errorf("MatchSkill(%q) = %q, want %q — %s", tc.input, name, tc.want, tc.why)
			}
		})
	}
}

// Câu tán gẫu/không liên quan không được kích hoạt skill nào: mỗi lần kích hoạt
// sai là ~1.285 token trả cho một hướng dẫn không dùng tới.
func TestImportedSkills_KhongKichHoatVoiCauVoThuong(t *testing.T) {
	l := loaderOrSkip(t)

	for _, input := range []string{
		"xin chào",
		"cảm ơn nhé",
		"hôm nay trời thế nào",
		"bạn là ai",
	} {
		t.Run(input, func(t *testing.T) {
			if got := l.MatchSkill(input); got != nil {
				t.Errorf("MatchSkill(%q) = %q, want không kích hoạt gì", input, got.Name)
			}
		})
	}
}
