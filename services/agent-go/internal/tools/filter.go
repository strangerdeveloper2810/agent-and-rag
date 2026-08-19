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
	"url", "online", "báo", "thông tin", "wikipedia",
	"best practice", "best practices", "claude", "gpt", "llm", "ai", "cách", "hướng dẫn",
	"mới nhất", "kinh nghiệm", "nào", "gì", "là gì", "thế nào",
}

// documentKeywords nhận diện câu hỏi thực sự nhắm vào TÀI LIỆU RIÊNG người
// dùng đã upload (cơ sở tri thức RAG), tách khỏi searchKeywords vốn quá phổ
// thông ("cách", "nào", "gì", "thế nào"... khớp gần như mọi câu tiếng Việt).
//
// Vì sao phải tách: trước đây "rag"/"document"/"tài liệu" nằm CHUNG trong
// searchKeywords, nên chỉ cần câu hỏi chứa "cách" hay "gì" là rag.search được
// cấp — biến nó thành tool mặc định cho mọi câu hỏi. Người dùng chỉ muốn dùng
// RAG khi hỏi về nghiệp vụ/tài liệu chuyên dụng của họ.
var documentKeywords = []string{
	"rag", "document", "tài liệu", "tài liệu của tôi", "upload", "đã upload",
	"knowledge base", "cơ sở tri thức", "tri thức", "nội bộ", "quy chuẩn",
	"quy trình", "convention", "nghiệp vụ", ".md", ".pdf", ".docx",
}

var utilityKeywords = []string{
	"tính", "tính toán", "math", "calculator", "cộng", "trừ", "nhân", "chia",
	"mấy giờ", "ngày", "hôm nay", "bây giờ", "date", "time", "dịch", "translate",
	"hẹn giờ", "timer", "báo thức", "lưu", "bộ nhớ", "memory", "note",
}

var planningKeywords = []string{
	"plan", "planning", "kế hoạch", "roadmap", "lộ trình", "tư vấn", "brainstorm",
	"xây dựng", "phát triển", "thiết kế", "kiến trúc", "triển khai", "hệ thống",
	"architecture", "tư vấn giải pháp", "hướng giải quyết", "app", "ứng dụng", "dự án",
}

// FilterToolDefs filters the registered tools dynamically based on user query intent and current execution step.
// For multi-step execution (step > 0), all tools in the registry are kept to allow complex tool chains.
// For initial step (step 0), tools are pruned down to 3-8 relevant tools to minimize token usage and latency.
//
// LƯU Ý về rag.search/rag.read: ở step 0 chúng CHỈ được cấp khi câu hỏi khớp
// documentKeywords (nhắm vào tài liệu người dùng đã upload). Câu hỏi lập trình
// hay tra cứu chung chung không được cấp — trước đây chúng nằm trong cả nhánh
// no-intent, code và search, nên rag.search là 2/5 tool được cấp cho MỌI câu
// hỏi, khiến model coi đó là tool mặc định phải dùng. Từ step 1 trở đi vẫn trả
// toàn bộ registry nên RAG vẫn dùng được như phương án dự phòng khi web search
// không cho kết quả.
//
// userQuery PHẢI là câu hỏi mới nhất của người dùng (agent.State.LastUserContent);
// truyền câu cũ vào đây từng là nguyên nhân lọc tool sai suốt cả cuộc hội thoại.
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
	hasPlanningIntent := containsAny(queryLower, planningKeywords)
	// hasDocIntent là CỬA DUY NHẤT cấp rag.search/rag.read ở step 0. Câu hỏi
	// lập trình hay tra cứu chung chung không còn được cấp RAG nữa — nếu web
	// search không đủ, model vẫn lấy được RAG từ step 1 trở đi (xem nhánh
	// step > 0 phía trên) đúng như ý "chỉ khi web search không ra mới dùng rag".
	hasDocIntent := containsAny(queryLower, documentKeywords)

	// If no specific intent detected (e.g. casual chat/greetings), return minimal core tools
	if !hasCodeIntent && !hasSearchIntent && !hasUtilityIntent && !hasDocIntent && !hasPlanningIntent {
		return filterByName(allDefs, []string{
			"ask_user", "web.search", "memory.recall", "echo",
		})
	}

	selectedNames := make(map[string]bool)

	// Always keep memory recall, save & ask_user
	selectedNames["memory.recall"] = true
	selectedNames["memory.save"] = true
	selectedNames["ask_user"] = true

	if hasCodeIntent {
		for _, name := range []string{"file.search", "file.read", "file.write", "shell.exec", "git", "version", "web.search", "web.fetch"} {
			selectedNames[name] = true
		}
	}

	if hasSearchIntent {
		for _, name := range []string{"web.search", "web.fetch", "notes.search", "notes.create"} {
			selectedNames[name] = true
		}
	}

	if hasDocIntent {
		for _, name := range []string{"rag.search", "rag.read", "rag.list", "web.search", "web.fetch"} {
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

// UnionToolDefs bổ sung các tool có tên trong extraNames vào defs (nếu tool đó
// tồn tại trong registry và chưa có mặt), giữ nguyên thứ tự defs rồi nối phần
// thêm vào cuối. Dùng để bảo đảm tool mà skill khai báo luôn khả dụng.
func (r *Registry) UnionToolDefs(defs []provider.ToolDef, extraNames []string) []provider.ToolDef {
	if len(extraNames) == 0 {
		return defs
	}
	present := make(map[string]bool, len(defs))
	for _, d := range defs {
		present[d.Name] = true
	}
	for _, name := range extraNames {
		if present[name] {
			continue
		}
		t, ok := r.Get(name)
		if !ok {
			// Skill khai tool không có trong registry của agent này — bỏ qua im
			// lặng là đúng: mỗi agent có registry riêng (vd research không có
			// file.*), skill dùng chung cho mọi agent.
			continue
		}
		present[name] = true
		defs = append(defs, provider.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	return defs
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
	all := make([]string, 0, len(codeKeywords)+len(searchKeywords)+len(utilityKeywords)+len(documentKeywords))
	all = append(all, codeKeywords...)
	all = append(all, searchKeywords...)
	all = append(all, utilityKeywords...)
	// documentKeywords cũng cần word boundary: "rag" không được khớp
	// "storage"/"fragment", "upload" không khớp "uploaded_at"...
	all = append(all, documentKeywords...)
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
