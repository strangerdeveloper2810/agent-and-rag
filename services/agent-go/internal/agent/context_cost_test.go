package agent

import (
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/skills"
)

func sampleSummaries(n int) []skills.SkillSummary {
	out := make([]skills.SkillSummary, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, skills.SkillSummary{
			Name: "skill-" + string(rune('a'+i%26)),
			// Description dài như thật (SKILL.md hiện tại ~100-200 ký tự).
			Description: strings.Repeat("mô tả dài dòng lặp lại nhiều lần ", 6),
		})
	}
	return out
}

// Danh sách skill nằm trong MỌI request nên độ dài của nó là chi phí lặp mỗi
// lượt chat. Description bị loại vì skill do code Go chọn (skills.MatchSkill),
// model không tham gia — xem comment ở buildSkillCatalogue.
func TestBuildSystemPrompt_KhongGuiDescriptionCuaSkill(t *testing.T) {
	summaries := sampleSummaries(30)
	prompt := BuildSystemPrompt(nil, summaries)

	if strings.Contains(prompt, "mô tả dài dòng") {
		t.Error("description của skill vẫn được gửi — tốn token mỗi request mà model không dùng để chọn skill")
	}

	// Tên skill PHẢI còn: model cần biết catalogue để trả lời "bạn làm được gì".
	for _, s := range summaries[:5] {
		if !strings.Contains(prompt, s.Name) {
			t.Errorf("system prompt thiếu tên skill %q", s.Name)
		}
	}
}

func TestBuildSkillCatalogue_GomNhieuTenTrenMotDong(t *testing.T) {
	got := buildSkillCatalogue(sampleSummaries(13))

	lines := strings.Split(strings.TrimSpace(got), "\n")
	wantLines := 3 // 13 skill / 6 mỗi dòng → 3 dòng
	if len(lines) != wantLines {
		t.Errorf("số dòng = %d, want %d — gom tên lại để bớt token phụ trợ", len(lines), wantLines)
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "- ") {
			t.Errorf("dòng %q phải bắt đầu bằng '- ' cho model dễ đọc", l)
		}
	}
}

func TestBuildSkillCatalogue_RongThiKhongCoMuc(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil)
	if strings.Contains(prompt, "[KỸ NĂNG]") {
		t.Error("không có skill nào thì không được in mục [KỸ NĂNG] rỗng")
	}
}

// Khoá lại mức tiết kiệm: 30 skill không được vượt quá ~200 rune trong prompt.
// Trước khi sửa, cùng bộ 30 skill này chiếm hơn 3.000 rune (~1.100 token).
func TestBuildSystemPrompt_ChiPhiDanhSachSkillBiChanTran(t *testing.T) {
	summaries := sampleSummaries(30)
	withSkills := len([]rune(BuildSystemPrompt(nil, summaries)))
	without := len([]rune(BuildSystemPrompt(nil, nil)))

	cost := withSkills - without
	const maxCost = 450 // gồm cả header của mục
	if cost > maxCost {
		t.Errorf("danh sách 30 skill chiếm %d rune trong prompt, ngưỡng %d", cost, maxCost)
	}
}
