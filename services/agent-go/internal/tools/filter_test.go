package tools

import (
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// filterTestRegistry là registry đại diện cho research/general agent (đủ >6 tool).
func filterTestRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewEchoTool())
	r.Register(NewWebSearchTool(nil))
	r.Register(NewWebFetchTool(nil))
	r.Register(NewFileSearchTool(nil))
	r.Register(NewFileReadTool(nil))
	r.Register(NewShellTool(nil))
	r.Register(NewCalculatorTool())
	r.Register(NewSaveMemoryTool())
	r.Register(NewRecallMemoryTool())
	return r
}

func defNames(defs []provider.ToolDef) map[string]bool {
	m := make(map[string]bool, len(defs))
	for _, d := range defs {
		m[d.Name] = true
	}
	return m
}

func TestFilterToolDefs_SearchIntent(t *testing.T) {
	r := filterTestRegistry()
	defs := r.FilterToolDefs("tìm kiếm tin tức mới nhất về deepseek", 0)
	names := defNames(defs)

	if !names["web.search"] || !names["web.fetch"] {
		t.Errorf("search intent should keep web.search/web.fetch, got %v", names)
	}
	if names["shell.exec"] || names["file.write"] || names["calculator"] {
		t.Errorf("search intent should drop code/utility tools, got %v", names)
	}
}

func TestFilterToolDefs_WordBoundary(t *testing.T) {
	// "good" chứa "go" nhưng không phải từ "go" riêng → KHÔNG trigger code intent.
	r := filterTestRegistry()
	defs := r.FilterToolDefs("điều này có tốt good không?", 0)
	names := defNames(defs)

	if names["shell.exec"] || names["file.search"] {
		t.Errorf("'good' must not trigger code intent via 'go' substring, got %v", names)
	}
}

func TestFilterToolDefs_CodeIntent(t *testing.T) {
	r := filterTestRegistry()
	defs := r.FilterToolDefs("fix bug trong file main.go", 0)
	names := defNames(defs)

	if !names["file.search"] || !names["file.read"] || !names["shell.exec"] {
		t.Errorf("code intent should keep file/shell tools, got %v", names)
	}
	if names["translate"] {
		t.Errorf("code intent should drop translate, got %v", names)
	}
}

func TestFilterToolDefs_NoIntent(t *testing.T) {
	r := filterTestRegistry()
	defs := r.FilterToolDefs("xin chào", 0)
	names := defNames(defs)

	// Không intent → chỉ giữ bộ tối thiểu (rag.search không có trong registry
	// test nên vắng mặt là đúng).
	for _, keep := range []string{"echo", "web.search", "memory.recall"} {
		if !names[keep] {
			t.Errorf("no-intent should keep %q, got %v", keep, names)
		}
	}
	for _, drop := range []string{"shell.exec", "calculator", "web.fetch", "memory.save"} {
		if names[drop] {
			t.Errorf("no-intent should drop %q, got %v", drop, names)
		}
	}
}

// ragFilterTestRegistry giống filterTestRegistry nhưng CÓ thêm 2 tool RAG, để
// kiểm tra rag.search chỉ được cấp khi câu hỏi nhắm vào tài liệu đã upload.
func ragFilterTestRegistry() *Registry {
	r := filterTestRegistry()
	r.Register(NewRAGSearchTool(nil, "test", "", nil, RAGSearchConfig{}))
	r.Register(NewRAGReadTool(nil, "test"))
	return r
}

// TestFilterToolDefs_RAGGatedByDocumentIntent khoá hành vi người dùng yêu cầu:
// "không phải lúc nào cũng dùng rag". Trước fix, rag.search nằm trong CẢ 3
// nhánh (no-intent, code, search) nên nó được cấp cho mọi câu hỏi — với nhánh
// no-intent thì rag.search/rag.read chiếm 2/5 tool, khiến model coi RAG là tool
// mặc định phải gọi.
func TestFilterToolDefs_RAGGatedByDocumentIntent(t *testing.T) {
	r := ragFilterTestRegistry()

	tests := []struct {
		name    string
		query   string
		wantRAG bool
	}{
		{
			// Chính câu hỏi trong log dev thật đã kích hoạt rag.search sai.
			name:    "câu hỏi lập trình chung chung → KHÔNG cấp rag",
			query:   "Viết custom hook useMemo kết hợp với useSelector của react-redux để memoized state",
			wantRAG: false,
		},
		{
			name:    "câu hỏi giải thích khái niệm → KHÔNG cấp rag",
			query:   "Hãy giải thích chi tiết khái niệm sau một cách trực quan dễ hiểu: golang, concurrency, goroutine",
			wantRAG: false,
		},
		{
			name:    "chào hỏi → KHÔNG cấp rag",
			query:   "xin chào",
			wantRAG: false,
		},
		{
			name:    "hỏi về tài liệu đã upload → CÓ cấp rag",
			query:   "trong tài liệu tôi upload có nói gì về cách cấu hình không?",
			wantRAG: true,
		},
		{
			name:    "hỏi về quy chuẩn nội bộ → CÓ cấp rag",
			query:   "convention nội bộ của dự án quy định thế nào?",
			wantRAG: true,
		},
		{
			name:    "nhắc tên file tài liệu → CÓ cấp rag",
			query:   "đọc giúp tôi nestjs.md",
			wantRAG: true,
		},
		{
			// Case hợp lệ mà fix KHÔNG được phá: vừa là câu hỏi code, vừa nhắc tài liệu.
			name:    "đối chiếu code với tài liệu đã upload → CÓ cấp rag VÀ tool code",
			query:   "review code này theo convention trong tài liệu tôi đã upload",
			wantRAG: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := defNames(r.FilterToolDefs(tc.query, 0))
			if got := names["rag.search"]; got != tc.wantRAG {
				t.Errorf("rag.search present = %v, want %v (query=%q, tools=%v)", got, tc.wantRAG, tc.query, names)
			}
			if got := names["rag.read"]; got != tc.wantRAG {
				t.Errorf("rag.read present = %v, want %v (query=%q)", got, tc.wantRAG, tc.query)
			}
		})
	}
}

// TestFilterToolDefs_RAGAvailableFromStepOne xác nhận RAG vẫn dùng được như
// phương án dự phòng: người dùng muốn "khi web search không tìm thấy kết quả
// thì mới dùng đến rag", nên việc chặn chỉ áp ở step 0.
func TestFilterToolDefs_RAGAvailableFromStepOne(t *testing.T) {
	r := ragFilterTestRegistry()
	names := defNames(r.FilterToolDefs("Viết custom hook useMemo với useSelector", 1))
	if !names["rag.search"] {
		t.Errorf("step>0 phải vẫn cấp rag.search làm fallback, got %v", names)
	}
}

// TestFilterToolDefs_DocumentKeywordWordBoundary: "rag" không được khớp trong
// "storage"/"fragment" — cùng lớp lỗi word-boundary đã fix cho codeKeywords.
func TestFilterToolDefs_DocumentKeywordWordBoundary(t *testing.T) {
	r := ragFilterTestRegistry()
	for _, q := range []string{
		"cấu hình storage cho ứng dụng thế nào?",
		"fragment này dùng để làm gì?",
	} {
		if names := defNames(r.FilterToolDefs(q, 0)); names["rag.search"] {
			t.Errorf("query %q không được trigger document intent qua substring 'rag', got %v", q, names)
		}
	}
}

func TestFilterToolDefs_StepGreaterThanZeroKeepsAll(t *testing.T) {
	r := filterTestRegistry()
	all := r.ToolDefs()
	defs := r.FilterToolDefs("bất kỳ", 1)
	if len(defs) != len(all) {
		t.Fatalf("step>0 should keep all tools: got %d, want %d", len(defs), len(all))
	}
}

func TestFilterToolDefs_SmallRegistryKeepsAll(t *testing.T) {
	r := NewRegistry()
	r.Register(NewEchoTool())
	r.Register(NewCalculatorTool())
	r.Register(NewSaveMemoryTool())

	defs := r.FilterToolDefs("tìm kiếm gì đó", 0)
	if len(defs) != 3 {
		t.Fatalf("small registry (<=6) should keep all tools: got %d, want 3", len(defs))
	}
}
