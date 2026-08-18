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
	Content     string   // Thân SKILL.md (KHÔNG gồm frontmatter — xem parseSkill)

	// Triggers là các cụm từ kích hoạt TƯỜNG MINH (frontmatter `triggers:`),
	// hỗ trợ cả tiếng Việt. Đây là tín hiệu MẠNH, ngang với việc gọi thẳng tên
	// skill.
	//
	// Vì sao cần: description/when_to_use là văn xuôi tiếng Anh, trong khi người
	// dùng chat tiếng Việt. Đo trên skill thật cho thấy điểm khớp keyword của
	// skill ĐÚNG và skill SAI đều chỉ 3-4 điểm — tức nhiễu, không phân biệt được.
	// Ví dụ thật: câu "giải thích go, concurrency, ... database design" khớp
	// `api-designer` chỉ vì lọt chữ "design"; câu "check trong local doc..."
	// khớp `code-review` chỉ vì lọt chữ "check".
	Triggers []string
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

// Trọng số khi CHẤM ĐIỂM skill. Khớp đúng tên skill là tín hiệu mạnh nhất
// (người dùng gọi thẳng tên), rồi tới when_to_use (mô tả tình huống dùng), cuối
// cùng là description (mô tả năng lực, chung chung hơn).
const (
	skillScoreNameMatch      = 100
	skillScoreTriggerMatch   = 100
	skillScoreWhenToUseMatch = 3
	skillScoreDescMatch      = 1
)

// minSkillActivationScore là ngưỡng ĐIỂM TỐI THIỂU để kích hoạt một skill.
//
// Đo trên 23 skill thật với input thật từ log dev: điểm khớp keyword của skill
// ĐÚNG và skill SAI đều nằm trong khoảng 1-4 — tức không phân biệt được, chỉ là
// trùng hợp vài từ kỹ thuật tiếng Anh lọt vào câu tiếng Việt. Kích hoạt sai
// KHÔNG hề vô hại: nó nhồi toàn bộ SKILL.md (api-designer = 10.904 ký tự,
// ~3.000 token) vào system prompt và lái model sai hướng.
//
// Vì vậy: chỉ kích hoạt khi có tín hiệu MẠNH — người dùng gọi thẳng tên skill,
// khớp `triggers` tường minh, hoặc khớp nhiều từ khoá when_to_use cùng lúc.
// Thà KHÔNG kích hoạt skill nào còn hơn kích hoạt sai skill.
const minSkillActivationScore = 6

// MatchSkill tìm skill phù hợp nhất với input của người dùng bằng cách CHẤM
// ĐIỂM tất cả skill rồi lấy điểm cao nhất.
//
// Vì sao không dùng "khớp đầu tiên thắng" như trước: thứ tự duyệt là thứ tự nạp
// từ os.ReadDir, tức ALPHABET. Skill đầu bảng (`api-designer`) có when_to_use
// chứa toàn từ phổ thông ("design", "data", "review", "create") nên nó được
// quyền chọn trước 22 skill còn lại và thắng gần như mọi câu có các từ đó —
// log dev thật cho thấy câu hỏi về Go concurrency lại kích hoạt `api-designer`,
// và câu "check trong local doc..." lại kích hoạt `code-review`.
// Trước khi có l.order thì thứ tự map là random nên lỗi này ngẫu nhiên; làm
// deterministic xong thì nó thành thiên lệch hệ thống — nên phải chấm điểm theo
// SỐ từ khoá khớp thay vì dựa vào thứ tự.
//
// Trả về nil nếu không có skill nào khớp.
func (l *Loader) MatchSkill(userInput string) *Skill {
	lower := strings.ToLower(userInput)
	// Normalize input: replace punctuation with spaces, then pad with spaces
	// for word-boundary matching (avoids "debugging" matching "debug").
	normalized := normalizeForWordMatch(lower)

	var best *Skill
	bestScore := 0

	// Duyệt theo l.order (thứ tự nạp, ổn định) để khi ĐIỂM BẰNG NHAU kết quả
	// vẫn deterministic — xem comment ở field Loader.order.
	for _, name := range l.order {
		s := l.skills[name]
		score := 0

		nameLower := strings.ToLower(s.Name)
		nameSpaced := strings.ReplaceAll(nameLower, "-", " ")
		if strings.Contains(normalized, " "+nameLower+" ") ||
			strings.Contains(normalized, " "+nameSpaced+" ") {
			score += skillScoreNameMatch
		}
		// Trigger tường minh: khớp cụm từ (có thể tiếng Việt), không cần ranh
		// giới từ vì cụm nhiều từ đã đủ đặc trưng.
		for _, trg := range s.Triggers {
			trg = strings.ToLower(strings.TrimSpace(trg))
			if trg == "" {
				continue
			}
			if strings.Contains(lower, trg) {
				score += skillScoreTriggerMatch
				break
			}
		}
		score += skillScoreWhenToUseMatch * countKeywordMatches(normalized, s.WhenToUse)
		score += skillScoreDescMatch * countKeywordMatches(normalized, s.Description)

		if score > bestScore {
			best, bestScore = s, score
		}
	}

	// Điểm quá thấp = trùng hợp, không phải ý định → không kích hoạt skill nào.
	if bestScore < minSkillActivationScore {
		return nil
	}
	return best
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

	// Content = THÂN skill, KHÔNG gồm frontmatter.
	//
	// Trước đây gán cả `raw` nên mỗi lần skill được kích hoạt, toàn bộ YAML
	// frontmatter đi vào system prompt — trong đó có `triggers` (danh sách 20+
	// từ khoá chỉ dùng cho MatchSkill bên Go) và `tools`. Vừa tốn token mỗi
	// request có skill (learning-tutor: ~600 byte frontmatter), vừa dở về chất
	// lượng: đưa một danh sách từ khoá vào prompt dễ làm model nhắc lại chúng.
	skill.Content = strings.TrimSpace(parts[2])

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
		case "triggers":
			skill.Triggers = parseToolsList(value)
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

// countKeywordMatches đếm SỐ từ khoá đáng kể (không trùng lặp) của reference
// xuất hiện trong text. Từ khoá phải >= minSkillKeywordLen ký tự, không nằm
// trong skillStopWords, và khớp theo RANH GIỚI TỪ (không phải substring) để
// "use" không khớp "useMemo" và "port" không khớp "important".
//
// normalizedText PHẢI là kết quả của normalizeForWordMatch (đã bọc dấu cách).
func countKeywordMatches(normalizedText, reference string) int {
	refLower := strings.ToLower(reference)
	words := strings.FieldsFunc(refLower, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == ';' || r == ':'
	})

	seen := make(map[string]bool, len(words))
	count := 0
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) < minSkillKeywordLen || skillStopWords[word] || seen[word] {
			continue
		}
		seen[word] = true
		if strings.Contains(normalizedText, " "+word+" ") {
			count++
		}
	}
	return count
}

// containsAnyKeyword giữ lại cho tương thích: có khớp ít nhất 1 từ khoá hay không.
func containsAnyKeyword(text, reference string) bool {
	return countKeywordMatches(normalizeForWordMatch(text), reference) > 0
}
