// Package skills cung cấp progressive disclosure cho JARVIS.
// Skill là các chỉ thị chuyên biệt chỉ được nạp đầy đủ khi cần,
// giúp system prompt luôn gọn nhẹ.
//
// Cơ chế:
//   - ListSkills() -> danh sách tên + mô tả (cho system prompt)
//   - LoadSkill(name) -> nạp nội dung đầy đủ khi skill được trigger
//   - MatchSkill(userInput) -> so khớp từ khóa để tự động chọn skill
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Skill đại diện cho một kỹ năng của JARVIS.
// Name, Description, WhenToUse được parse từ frontmatter.
// Content là toàn bộ nội dung thân của SKILL.md (sau frontmatter).
// Tools là danh sách tên tool liên quan (để hiển thị).
type Skill struct {
	Name        string   // "code-review", "standup-prep"
	Description string   // Mô tả ngắn
	WhenToUse   string   // Khi nào kích hoạt skill này
	Tools       []string // Danh sách tool liên quan
	Content     string   // Toàn bộ nội dung SKILL.md, bao gồm cả frontmatter
}

// SkillSummary là bản rút gọn của Skill dùng cho system prompt.
// Chỉ chứa Name + Description để giữ prompt ngắn.
type SkillSummary struct {
	Name        string
	Description string
}

// Loader quản lý danh sách skill đã nạp từ thư mục skills/.
type Loader struct {
	skills map[string]*Skill // name -> skill
}

// NewLoader quét skillsDir và nạp tất cả file SKILL.md vào bộ nhớ.
// Mỗi skill nằm trong thư mục con: skills/<name>/SKILL.md.
// Trả về lỗi nếu thư mục không tồn tại hoặc không đọc được.
func NewLoader(skillsDir string) (*Loader, error) {
	l := &Loader{
		skills: make(map[string]*Skill),
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("skills: read dir %q: %w", skillsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			// Bỏ qua thư mục không có SKILL.md
			continue
		}

		skill, err := parseSkill(string(data))
		if err != nil {
			return nil, fmt.Errorf("skills: parse %q: %w", skillFile, err)
		}

		l.skills[skill.Name] = skill
	}

	return l, nil
}

// ListSkills trả về danh sách skill dạng rút gọn (tên + mô tả).
// Dùng để chèn vào system prompt — chỉ tốn vài dòng.
func (l *Loader) ListSkills() []SkillSummary {
	result := make([]SkillSummary, 0, len(l.skills))
	for _, s := range l.skills {
		result = append(result, SkillSummary{
			Name:        s.Name,
			Description: s.Description,
		})
	}
	return result
}

// LoadSkill nạp nội dung đầy đủ của một skill theo tên.
// Trả về nil nếu skill không tồn tại.
func (l *Loader) LoadSkill(name string) *Skill {
	return l.skills[name]
}

// MatchSkill tìm skill phù hợp nhất với input của người dùng.
// Dùng so khớp từ khóa đơn giản:
//   - Kiểm tra tên skill có xuất hiện trong input không
//   - Kiểm tra từ khóa trong WhenToUse
//   - Kiểm tra từ khóa trong Description
//
// Trả về nil nếu không có skill nào khớp.
func (l *Loader) MatchSkill(userInput string) *Skill {
	lower := strings.ToLower(userInput)

	// Match by name first (e.g., "code review" matches "code-review")
	// Normalize input: replace punctuation with spaces, then pad with spaces
	// for word-boundary matching (avoids "debugging" matching "debug").
	normalized := normalizeForWordMatch(lower)
	for _, s := range l.skills {
		nameLower := strings.ToLower(s.Name)
		nameSpaced := strings.ReplaceAll(nameLower, "-", " ")
		if strings.Contains(normalized, " "+nameLower+" ") ||
			strings.Contains(normalized, " "+nameSpaced+" ") {
			return s
		}
	}

	// Match by WhenToUse keywords
	for _, s := range l.skills {
		if containsAnyKeyword(lower, s.WhenToUse) {
			return s
		}
	}

	// Match by Description keywords
	for _, s := range l.skills {
		if containsAnyKeyword(lower, s.Description) {
			return s
		}
	}

	return nil
}

// Len trả về số lượng skill đã nạp.
func (l *Loader) Len() int {
	return len(l.skills)
}

// --- frontmatter parser ---

// parseSkill parse nội dung SKILL.md: frontmatter (giữa hai dấu ---) + body.
//
// Format frontmatter:
//
//	---
//	name: code-review
//	description: Review code for bugs, security, and best practices
//	when_to_use: When user asks for code review, bug analysis, or code quality check
//	tools: [file.read, shell.exec, git.diff, git.log]
//	---
func parseSkill(raw string) (*Skill, error) {
	skill := &Skill{}

	// Tìm frontmatter (giữa --- mở đầu và --- kết thúc)
	parts := strings.SplitN(raw, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("skills: missing frontmatter delimiters (---)")
	}

	fm := strings.TrimSpace(parts[1])
	skill.Content = strings.TrimSpace(raw)

	lines := strings.Split(fm, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = value
		case "when_to_use":
			skill.WhenToUse = value
		case "tools":
			skill.Tools = parseToolsList(value)
		}
	}

	if skill.Name == "" {
		return nil, fmt.Errorf("skills: frontmatter missing required field 'name'")
	}

	return skill, nil
}

// parseToolsList parse danh sách tool từ format [a, b, c] hoặc a, b, c.
func parseToolsList(raw string) []string {
	raw = strings.TrimSpace(raw)

	// Bỏ dấu [ ] nếu có
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		raw = raw[1 : len(raw)-1]
	}

	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// normalizeForWordMatch thay thế dấu câu bằng dấu cách và bọc text
// với dấu cách 2 đầu để so khớp từ chính xác (word-boundary matching).
func normalizeForWordMatch(s string) string {
	// Replace common punctuation with spaces
	replacer := strings.NewReplacer(
		",", " ", ".", " ", "!", " ", "?", " ", ";", " ", ":", " ",
		"-", " ", "_", " ", "/", " ",
	)
	return " " + replacer.Replace(s) + " "
}

// containsAnyKeyword kiểm tra xem text có chứa ít nhất 1 từ khóa
// đáng kể (>= 3 ký tự) từ reference không.
func containsAnyKeyword(text, reference string) bool {
	refLower := strings.ToLower(reference)
	words := strings.FieldsFunc(refLower, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == ';' || r == ':'
	})
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) >= 3 && strings.Contains(text, word) {
			return true
		}
	}
	return false
}
