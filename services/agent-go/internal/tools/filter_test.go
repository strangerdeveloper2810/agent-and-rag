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
