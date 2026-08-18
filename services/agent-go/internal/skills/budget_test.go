package skills

import (
	"strings"
	"testing"
)

func TestPromptBody_Ngan_GiuNguyen(t *testing.T) {
	s := &Skill{Content: "# Skill\n\nNội dung ngắn."}
	if got := s.PromptBody(); got != s.Content {
		t.Errorf("skill ngắn phải giữ nguyên, got %q", got)
	}
	if strings.Contains(s.PromptBody(), "lược bỏ") {
		t.Error("không được thêm ghi chú lược bỏ khi không cắt gì")
	}
}

func TestTruncateToSections_CatOEanhGioiSection(t *testing.T) {
	content := "# Tiêu đề\n\nIntro ngắn.\n\n" +
		"## Section A\n" + strings.Repeat("dòng A dài dài dài\n", 40) +
		"## Section B\n" + strings.Repeat("dòng B dài dài dài\n", 40) +
		"## Section C\n" + strings.Repeat("dòng C dài dài dài\n", 40)

	// Ngân sách đủ cho intro + trọn Section A, không đủ cho Section B.
	got := truncateToSections(content, 1400)

	if !strings.Contains(got, "# Tiêu đề") || !strings.Contains(got, "Intro ngắn") {
		t.Error("phần mở đầu (tiêu đề + intro) phải luôn được giữ")
	}
	if !strings.Contains(got, "## Section A") {
		t.Error("section đầu phải được giữ")
	}
	if strings.Contains(got, "## Section C") {
		t.Error("section vượt ngân sách phải bị bỏ")
	}
	// Điểm quan trọng: không được kết thúc giữa một section — nếu đã bỏ Section C
	// thì cũng không được giữ nửa đầu của nó.
	if strings.Contains(got, "dòng C") {
		t.Error("giữ nội dung của section đã bị bỏ — cắt không đúng ranh giới")
	}
	if strings.Contains(got, "dòng B") {
		t.Error("giữ nửa đầu Section B — phải dừng ở ranh giới section")
	}
	if !strings.Contains(got, "lược bỏ") {
		t.Error("phải nói rõ với model là nội dung bị lược, không được im lặng cắt")
	}
}

// Ca biên quan trọng: khi CHÍNH section đầu tiên đã vượt ngân sách, cắt ở ranh
// giới section sẽ chỉ còn tiêu đề + intro — tức mất sạch hướng dẫn. Lúc đó phải
// giữ một phần section đầu (cắt theo dòng) thay vì bỏ trắng.
func TestTruncateToSections_SectionDauQuaLon_VanGiuMotPhan(t *testing.T) {
	content := "# Tiêu đề\n\nIntro.\n\n" +
		"## Section A\n" + strings.Repeat("dòng A rất dài\n", 100) +
		"## Section B\n" + strings.Repeat("dòng B rất dài\n", 100)

	got := truncateToSections(content, 600)

	if !strings.Contains(got, "## Section A") {
		t.Error("mất section đầu — chỉ còn tiêu đề thì skill vô dụng")
	}
	if !strings.Contains(got, "dòng A") {
		t.Error("phải giữ được một phần nội dung của section đầu")
	}
	if strings.Contains(got, "## Section B") {
		t.Error("không được vượt ngân sách sang section sau")
	}
}

func TestTruncateToSections_KhongCoSectionNao(t *testing.T) {
	// File phẳng, không có "## " nào → vẫn phải cắt (theo dòng), không trả nguyên.
	content := strings.Repeat("một dòng nội dung dài\n", 200)
	got := truncateToSections(content, 500)

	if len(got) > 500+len(truncationNote) {
		t.Errorf("không cắt: %d byte", len(got))
	}
	if !strings.HasSuffix(got, truncationNote) {
		t.Error("thiếu ghi chú lược bỏ")
	}
}

func TestTruncateToSections_DongDauQuaDai(t *testing.T) {
	// Một dòng duy nhất dài hơn ngân sách → phải cắt cứng, không được trả rỗng.
	content := strings.Repeat("x", 5000)
	got := truncateToSections(content, 400)

	body := strings.TrimSuffix(got, truncationNote)
	if body == "" {
		t.Fatal("trả về rỗng — mất hoàn toàn nội dung skill")
	}
	if len(body) > 400 {
		t.Errorf("body = %d byte, want <= 400", len(body))
	}
}

func TestTruncateToSections_TiengVietKhongBiHong(t *testing.T) {
	// Cắt cứng theo rune: không được sinh ký tự lỗi ở cuối.
	content := strings.Repeat("tiếng Việt có dấu ", 500)
	got := truncateToSections(content, 300)

	if strings.Contains(got, "�") {
		t.Errorf("cắt làm hỏng ký tự multi-byte: %q", got)
	}
}

// Skill thật trong repo: sau khi bỏ frontmatter + gọt ngân sách, phần chèn vào
// prompt phải nằm trong trần. Đây là test khoá mức tiết kiệm.
func TestPromptBody_SkillThat_TrongNganSach(t *testing.T) {
	loader, err := NewLoader("../../skills")
	if err != nil {
		t.Skipf("không load được skills dir: %v", err)
	}

	for _, sum := range loader.ListSkills() {
		sk := loader.LoadSkill(sum.Name)
		if sk == nil {
			t.Fatalf("LoadSkill(%q) = nil", sum.Name)
		}

		body := sk.PromptBody()
		if len(body) > MaxPromptBytes+len(truncationNote) {
			t.Errorf("skill %q: phần chèn prompt %d byte, vượt trần %d",
				sum.Name, len(body), MaxPromptBytes)
		}

		// Frontmatter không được lọt vào prompt: `triggers:` là danh sách từ khoá
		// chỉ dành cho MatchSkill bên Go.
		if strings.Contains(body, "triggers:") {
			t.Errorf("skill %q: frontmatter vẫn còn trong nội dung chèn vào prompt", sum.Name)
		}
	}
}
