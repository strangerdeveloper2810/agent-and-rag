package skills

import "testing"

// Test cho bộ skill ĐỜI THƯỜNG — chạy trên thư mục skills THẬT của repo.
//
// Vì sao cần chạy trên skill thật thay vì fixture: vấn đề khó nhất của việc chọn
// skill không phải logic chấm điểm mà là TRIGGER KHỚP CHÉO giữa các skill, và
// điều đó chỉ lộ ra khi có đủ 30 skill cùng cạnh tranh. Đặc biệt với tiếng Việt
// KHÔNG DẤU, nhiều cụm trở thành substring của nhau: "dau tu" (đầu tư) nằm trong
// "bắt đầu từ", "mon an" (món ăn) nằm trong "môn anh" (văn), "an gi" (ăn gì) nằm
// trong "an toàn gì", "dang mua" (đáng mua) nằm trong "đang mưa".
//
// Trigger được so khớp bằng substring nên MỘT trigger quá chung là đủ để kích
// hoạt sai skill và nhồi vài nghìn token prompt lệch hướng vào system prompt.

// everydaySkills là các skill dành cho người dùng phổ thông (không phải dev).
var everydaySkills = []string{
	"personal-finance", "health-wellness", "travel-planner",
	"cooking-assistant", "language-learning", "shopping-advisor", "document-qa",
}

// privilegedToolNames khớp tools.privilegedTools — không import package tools để
// tránh phụ thuộc vòng; danh sách này cố tình lặp lại để test độc lập.
var privilegedToolNames = []string{"file.read", "file.write", "file.search", "shell.exec", "git"}

func TestEverydaySkills_LoadedAndSafe(t *testing.T) {
	l, err := NewLoader("../../skills")
	if err != nil {
		t.Skipf("bỏ qua: không đọc được thư mục skills thật: %v", err)
	}

	for _, name := range everydaySkills {
		s := l.LoadSkill(name)
		if s == nil {
			t.Errorf("không nạp được skill %q", name)
			continue
		}
		if len(s.Triggers) == 0 {
			t.Errorf("%s: thiếu triggers → câu hỏi tiếng Việt sẽ không kích hoạt được", name)
		}
		if len(s.Tools) == 0 {
			t.Errorf("%s: thiếu tools", name)
		}
		// Skill cho người dùng phổ thông KHÔNG được khai tool đặc quyền: chúng
		// tác động lên máy chạy agent và chỉ chủ hệ thống mới có quyền, nên khai
		// vào đây chỉ khiến model cố gọi rồi nhận lỗi.
		for _, tool := range s.Tools {
			for _, banned := range privilegedToolNames {
				if tool == banned {
					t.Errorf("%s khai tool đặc quyền %q — người dùng phổ thông không có quyền", name, tool)
				}
			}
		}
	}
}

func TestEverydaySkills_MatchesVietnameseQuestions(t *testing.T) {
	l, err := NewLoader("../../skills")
	if err != nil {
		t.Skipf("bỏ qua: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"quản lý chi tiêu tháng này thế nào", "personal-finance"},
		{"minh muon tiet kiem tien de mua nha", "personal-finance"},
		{"gợi ý thực đơn cho tuần", "cooking-assistant"},
		{"tủ lạnh còn trứng và cà chua thì nấu gì được", "cooking-assistant"},
		{"lên lịch trình đi Đà Nẵng 3 ngày", "travel-planner"},
		{"can mang gi trong hanh ly khi di du lich", "travel-planner"},
		{"học tiếng Anh giao tiếp", "language-learning"},
		{"sửa giúp mình ngữ pháp câu này", "language-learning"},
		{"nên mua laptop nào tầm 20 triệu", "shopping-advisor"},
		{"trong tài liệu tôi upload có gì", "document-qa"},
		{"theo tai lieu thi quy trinh onboarding the nao", "document-qa"},
		{"mình bị mất ngủ mấy tuần nay", "health-wellness"},
		{"chế độ ăn để giảm cân an toàn", "health-wellness"},
		{"lịch tập gym cho người mới", "health-wellness"},
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

// TestEverydaySkills_NoFalsePositives là phần QUAN TRỌNG NHẤT của file này: các
// câu dưới đây từng (hoặc rất dễ) kích hoạt sai skill vì trigger là substring
// của cụm khác — nhất là khi người dùng gõ tiếng Việt không dấu.
func TestEverydaySkills_NoFalsePositives(t *testing.T) {
	l, err := NewLoader("../../skills")
	if err != nil {
		t.Skipf("bỏ qua: %v", err)
	}

	tests := []struct {
		input string
		why   string
	}{
		{"bắt đầu từ đâu bây giờ", `"đầu tư" nằm trong "bắt đầu từ"`},
		{"bat dau tu dau bay gio", `"dau tu" nằm trong "bat dau tu" (không dấu)`},
		{"học môn anh văn khó lắm", `"món ăn" gần với "môn anh"`},
		{"hoc mon anh van kho lam", `"mon an" nằm trong "mon anh" (không dấu)`},
		{"an toan gi dau ma lo", `"an gi" nằm trong "toan gi" (không dấu)`},
		{"trời đang mưa to", `"đáng mua" gần với "đang mưa"`},
		{"troi dang mua to", `"dang mua" trùng hoàn toàn khi không dấu`},
		{"upload file len server nhu the nao", `"rag" nằm trong "storage"; "upload" quá chung`},
		{"hôm nay trời đẹp quá", "câu tán gẫu, không có ý định nào"},
		{"chào bạn, bạn tên gì", "câu chào"},
		{"bây giờ là mấy giờ", "câu hỏi vụn, không cần skill"},
		{"cảm ơn bạn nhiều nha", "câu cảm ơn"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := l.MatchSkill(tc.input); got != nil {
				t.Errorf("kích hoạt sai skill %q cho câu %q (bẫy: %s)", got.Name, tc.input, tc.why)
			}
		})
	}
}

// Skill kỹ thuật (dành cho chủ hệ thống) không được bị skill đời thường cướp.
func TestEverydaySkills_DoNotHijackTechnicalSkills(t *testing.T) {
	l, err := NewLoader("../../skills")
	if err != nil {
		t.Skipf("bỏ qua: %v", err)
	}

	tests := map[string]string{
		"review code này giúp tôi":            "code-review",
		"debug lỗi này":                       "debug",
		"thiết kế REST API cho user service":  "api-designer",
		"tối ưu query này đang chậm":          "performance-optimizer",
		"deploy lên kubernetes thế nào":       "devops",
		"kiểm tra bảo mật cho endpoint login": "security-audit",
	}

	for input, want := range tests {
		got := l.MatchSkill(input)
		name := ""
		if got != nil {
			name = got.Name
		}
		if name != want {
			t.Errorf("MatchSkill(%q) = %q, want %q (skill kỹ thuật bị cướp)", input, name, want)
		}
	}
}
