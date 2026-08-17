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

	// order giữ tên skill theo thứ tự nạp (os.ReadDir trả về đã sort theo tên)
	// để MatchSkill và ListSkills duyệt ỔN ĐỊNH. Trước đây cả 2 duyệt trực tiếp
	// trên map: thứ tự map trong Go là random nên (1) cùng một câu hỏi khớp
	// nhiều skill thì skill nào thắng là random giữa các lần chạy, (2) block
	// [KỸ NĂNG] trong system prompt đổi thứ tự mỗi lần khởi động, phá vỡ chính
	// cái cacheable prefix mà BuildSystemPrompt đang tối ưu cho prompt caching.
	order []string
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

		if _, exists := l.skills[skill.Name]; !exists {
			l.order = append(l.order, skill.Name)
		}
		l.skills[skill.Name] = skill
	}

	return l, nil
}

// ListSkills trả về danh sách skill dạng rút gọn (tên + mô tả).
// Dùng để chèn vào system prompt — chỉ tốn vài dòng.
func (l *Loader) ListSkills() []SkillSummary {
	result := make([]SkillSummary, 0, len(l.order))
	for _, name := range l.order {
		s := l.skills[name]
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
	// Cả 3 vòng dưới duyệt theo l.order (thứ tự nạp, ổn định) chứ KHÔNG duyệt
	// map — xem comment ở field Loader.order.
	normalized := normalizeForWordMatch(lower)
	for _, name := range l.order {
		s := l.skills[name]
		nameLower := strings.ToLower(s.Name)
		nameSpaced := strings.ReplaceAll(nameLower, "-", " ")
		if strings.Contains(normalized, " "+nameLower+" ") ||
			strings.Contains(normalized, " "+nameSpaced+" ") {
			return s
		}
	}

	// Match by WhenToUse keywords
	for _, name := range l.order {
		if s := l.skills[name]; containsAnyKeyword(lower, s.WhenToUse) {
			return s
		}
	}

	// Match by Description keywords
	for _, name := range l.order {
		if s := l.skills[name]; containsAnyKeyword(lower, s.Description) {
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

// skillStopWords là các từ quá phổ thông trong description/when_to_use — chúng
// không mang ý định nào nên không được dùng làm căn cứ kích hoạt skill.
//
// Vì sao cần: containsAnyKeyword coi MỌI từ >= 3 ký tự là keyword và so khớp
// SUBSTRING. Ví dụ thật đã gặp: learning-tutor có description "...use
// analogies..." → từ "use" khớp substring trong "useMemo"/"useSelector", nên
// câu hỏi "Viết custom hook useMemo với useSelector" kích hoạt skill dạy học.
// Tương tự "the" khớp "theo", "new" khớp "renew", "and" khớp "android".
var skillStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "use": true,
	"using": true, "used": true, "uses": true, "when": true, "that": true,
	"this": true, "any": true, "all": true, "you": true,
	"your": true, "not": true, "but": true, "how": true, "why": true,
	"who": true, "was": true, "are": true, "has": true, "have": true,
	"had": true, "his": true, "her": true, "its": true, "our": true,
	"can": true, "may": true, "get": true, "got": true, "let": true,
	"new": true, "old": true, "one": true, "two": true, "out": true,
	"off": true, "via": true, "per": true, "than": true, "then": true,
	"them": true, "they": true, "there": true, "here": true, "what": true,
	"which": true, "into": true, "onto": true, "over": true, "some": true,
	"more": true, "most": true, "much": true, "many": true, "need": true,
	"needs": true, "want": true, "wants": true, "make": true, "makes": true,
	"does": true, "done": true, "will": true, "would": true, "should": true,
	"could": true, "about": true, "after": true, "before": true, "also": true,
	"just": true, "only": true, "very": true, "each": true, "both": true,
	"from": true, "been": true, "being": true, "were": true, "such": true,
}

// minSkillKeywordLen giữ ở 3 vì nhiều keyword kỹ thuật có nghĩa chỉ dài 3 ký
// tự ("bug", "api", "css", "sql", "git"). Việc chống khớp nhầm được xử lý bởi
// so khớp theo ranh giới từ + skillStopWords, không phải bằng cách nâng ngưỡng.
const minSkillKeywordLen = 3

// containsAnyKeyword kiểm tra xem text có chứa ít nhất 1 từ khóa đáng kể từ
// reference không. Từ khóa phải >= minSkillKeywordLen ký tự, không nằm trong
// skillStopWords, và phải khớp theo RANH GIỚI TỪ (không phải substring) để
// "use" không khớp "useMemo" và "port" không khớp "important".
func containsAnyKeyword(text, reference string) bool {
	refLower := strings.ToLower(reference)
	words := strings.FieldsFunc(refLower, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == ';' || r == ':'
	})
	// Bọc text bằng dấu cách + thay dấu câu để so khớp theo ranh giới từ.
	normalizedText := normalizeForWordMatch(text)
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) < minSkillKeywordLen || skillStopWords[word] {
			continue
		}
		if strings.Contains(normalizedText, " "+word+" ") {
			return true
		}
	}
	return false
}
