package tools

import (
	"regexp"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Intent keywords mapping to tool names
var codeKeywords = []string{
	"code", "coding", "program", "function", "func", "golang", "go", "python",
	"typescript", "javascript", "rust", "file", "read", "write", "search", "repo",
	"directory", "folder", "shell", "bash", "exec", "git", "commit", "branch",
	"refactor", "bug", "debug", "test", "build", "script", "doc", "md", "json",
}

var searchKeywords = []string{
	"search", "research", "tìm kiếm", "tra cứu", "tin tức", "news", "web", "fetch",
	"url", "online", "báo", "thông tin", "rag", "document", "tài liệu", "wikipedia",
	"best practice", "best practices", "claude", "gpt", "llm", "ai", "cách", "hướng dẫn",
	"mới nhất", "kinh nghiệm", "nào", "gì", "là gì", "thế nào",
}

var utilityKeywords = []string{
	"tính", "tính toán", "math", "calculator", "cộng", "trừ", "nhân", "chia",
	"mấy giờ", "ngày", "hôm nay", "bây giờ", "date", "time", "dịch", "translate",
	"hẹn giờ", "timer", "báo thức", "lưu", "bộ nhớ", "memory", "note",
}

// FilterToolDefs filters the registered tools dynamically based on user query intent and current execution step.
// For multi-step execution (step > 0), all tools in the registry are kept to allow complex tool chains.
// For initial step (step 0), tools are pruned down to 3-8 relevant tools to minimize token usage and latency.
func (r *Registry) FilterToolDefs(userQuery string, step int) []provider.ToolDef {
	allDefs := r.ToolDefs()
	if len(allDefs) <= 6 || step > 0 {
		return allDefs
	}

	queryLower := strings.ToLower(userQuery)

	// Check matching intents
	hasCodeIntent := containsAny(queryLower, codeKeywords)
	hasSearchIntent := containsAny(queryLower, searchKeywords)
	hasUtilityIntent := containsAny(queryLower, utilityKeywords)

	// If no specific intent detected (e.g. casual chat/greetings), return minimal core tools
	if !hasCodeIntent && !hasSearchIntent && !hasUtilityIntent {
		return filterByName(allDefs, []string{
			"rag.search", "web.search", "memory.recall", "echo",
		})
	}

	selectedNames := make(map[string]bool)

	// Always keep memory recall & save
	selectedNames["memory.recall"] = true
	selectedNames["memory.save"] = true

	if hasCodeIntent {
		for _, name := range []string{"file.search", "file.read", "file.write", "shell.exec", "git", "version", "rag.search", "web.search", "web.fetch"} {
			selectedNames[name] = true
		}
	}

	if hasSearchIntent {
		for _, name := range []string{"rag.search", "web.search", "web.fetch", "notes.search", "notes.create"} {
			selectedNames[name] = true
		}
	}

	if hasUtilityIntent {
		for _, name := range []string{"calculator", "datetime", "translate", "timer", "notes.create", "echo"} {
			selectedNames[name] = true
		}
	}

	filtered := make([]provider.ToolDef, 0, len(selectedNames))
	for _, def := range allDefs {
		if selectedNames[def.Name] {
			filtered = append(filtered, def)
		}
	}

	// Fallback if filter result is empty
	if len(filtered) == 0 {
		return allDefs
	}

	return filtered
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if matchKeyword(s, kw) {
			return true
		}
	}
	return false
}

// asciiWordRe nhận diện keyword ASCII đơn từ (chữ cái/số, không khoảng trắng).
var asciiWordRe = regexp.MustCompile(`^[a-z0-9]+$`)

// asciiWordRegex cache regex word-boundary cho từng keyword ASCII đơn từ.
// Word boundary tránh false positive: "go" chỉ khớp từ "go" riêng, không
// khớp "good"/"category"; "ai" không khớp "hai"/"email".
var asciiWordRegex = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp)
	all := make([]string, 0, len(codeKeywords)+len(searchKeywords)+len(utilityKeywords))
	all = append(all, codeKeywords...)
	all = append(all, searchKeywords...)
	all = append(all, utilityKeywords...)
	for _, kw := range all {
		if asciiWordRe.MatchString(kw) {
			m[kw] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
		}
	}
	return m
}()

// matchKeyword khớp keyword: từ ASCII đơn → word boundary; còn lại (tiếng
// Việt, cụm nhiều từ) → substring như cũ.
func matchKeyword(s, kw string) bool {
	if re, ok := asciiWordRegex[kw]; ok {
		return re.MatchString(s)
	}
	return strings.Contains(s, kw)
}

func filterByName(allDefs []provider.ToolDef, names []string) []provider.ToolDef {
	nameMap := make(map[string]bool, len(names))
	for _, n := range names {
		nameMap[n] = true
	}
	out := make([]provider.ToolDef, 0, len(names))
	for _, def := range allDefs {
		if nameMap[def.Name] {
			out = append(out, def)
		}
	}
	return out
}
