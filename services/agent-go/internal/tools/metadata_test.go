package tools

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// allTools liệt kê mọi tool dựng được offline (không cần Mongo/API key).
func allTools(t *testing.T) []Tool {
	t.Helper()
	dir := t.TempDir()
	hc := &http.Client{}

	return []Tool{
		NewEchoTool(),
		NewCalculatorTool(),
		NewCalendarTool(dir + "/cal.ics"),
		NewDateTimeTool(),
		NewFileWriteTool([]string{dir}),
		NewFileSearchTool([]string{dir}),
		NewFileReadTool([]string{dir}),
		NewHTTPTool(hc),
		NewGitTool(dir),
		NewNotesSearchTool(dir),
		NewNotesCreateTool(dir),
		NewSaveMemoryTool(newFakeMemoryStore()),
		NewRecallMemoryTool(newFakeMemoryStore()),
		NewListMemoriesTool(newFakeMemoryStore()),
		NewJSONTool(),
		NewTimerTool(),
		NewShellTool([]string{"echo"}),
		NewWebSearchTool(hc),
		NewWebFetchTool(hc),
		NewVersionTool(),
		NewTranslateTool(hc),
		NewWeatherTool(hc),
	}
}

// Mọi tool phải khai báo metadata hợp lệ — LLM dựa vào đây để chọn tool.
func TestAllTools_Metadata(t *testing.T) {
	seen := map[string]bool{}

	for _, tool := range allTools(t) {
		name := tool.Name()
		if name == "" {
			t.Errorf("%T: Name() rỗng", tool)
			continue
		}
		if seen[name] {
			t.Errorf("tên tool trùng: %q", name)
		}
		seen[name] = true

		if tool.Description() == "" {
			t.Errorf("%s: Description() rỗng", name)
		}

		schema := tool.Schema()
		if len(schema) == 0 {
			t.Errorf("%s: Schema() rỗng", name)
			continue
		}
		var parsed map[string]any
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Errorf("%s: Schema() không phải JSON hợp lệ: %v", name, err)
			continue
		}
		if parsed["type"] != "object" {
			t.Errorf("%s: schema type = %v, want object", name, parsed["type"])
		}

		switch tool.Kind() {
		case KindRead, KindWrite, KindDestructive:
		default:
			t.Errorf("%s: Kind() = %d không hợp lệ", name, tool.Kind())
		}
	}
}

// Tool ghi/xoá phải khai đúng Kind để guardrails chặn được.
func TestToolKinds(t *testing.T) {
	dir := t.TempDir()

	writeTools := map[string]Tool{
		"file.write":   NewFileWriteTool([]string{dir}),
		"notes.create": NewNotesCreateTool(dir),
		"memory.save":  NewSaveMemoryTool(newFakeMemoryStore()),
	}
	for name, tool := range writeTools {
		if tool.Kind() == KindRead {
			t.Errorf("%s: Kind() = KindRead, want write/destructive", name)
		}
	}

	readTools := map[string]Tool{
		"echo":          NewEchoTool(),
		"calculator":    NewCalculatorTool(),
		"memory.recall": NewRecallMemoryTool(newFakeMemoryStore()),
	}
	for name, tool := range readTools {
		if tool.Kind() != KindRead {
			t.Errorf("%s: Kind() = %d, want KindRead", name, tool.Kind())
		}
	}
}

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{Name: "không-có"}
	if !strings.Contains(err.Error(), "không-có") {
		t.Errorf("Error() = %q, want chứa tên tool", err.Error())
	}
}

// --- Helper thuần của web.search ---

func TestCleanHTML(t *testing.T) {
	cases := map[string]string{
		"<b>đậm</b>":              "đậm",
		"a &amp; b":               "a & b",
		"&lt;tag&gt;":             "<tag>",
		`&quot;trích&quot;`:       `"trích"`,
		"it&#39;s":                "it's",
		"  <p>khoảng trắng</p>  ": "khoảng trắng",
	}
	for in, want := range cases {
		if got := cleanHTML(in); got != want {
			t.Errorf("cleanHTML(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanURL(t *testing.T) {
	cases := map[string]string{
		"//example.com":       "https://example.com",
		"example.com":         "https://example.com",
		"http://example.com":  "http://example.com",
		"https://example.com": "https://example.com",
		"":                    "",
	}
	for in, want := range cases {
		if got := cleanURL(in); got != want {
			t.Errorf("cleanURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("ngắn", 100); got != "ngắn" {
		t.Errorf("truncateStr = %q, want giữ nguyên", got)
	}
	long := strings.Repeat("a", 50)
	got := truncateStr(long, 10)
	if got != strings.Repeat("a", 10)+"..." {
		t.Errorf("truncateStr = %q", got)
	}
}

func TestParseGoogleResults(t *testing.T) {
	html := `
	<a href="/url?q=https://golang.org/doc"><h3>Tài liệu Go</h3></a>
	<div class="VwiC3b">Trang tài liệu chính thức của Go.</div>
	<a href="https://vi.wikipedia.org/wiki/Go"><h3>Go trên Wikipedia</h3></a>
	<div class="VwiC3b">Bách khoa toàn thư.</div>
	<a href="https://www.google.com/setting"><h3>Cài đặt Google</h3></a>
	<div class="VwiC3b">Nội bộ Google.</div>
	`

	got := parseGoogleResults(html, 5)
	if len(got) != 1 {
		t.Fatalf("kết quả = %d, want 1 (loại Wikipedia + link nội bộ Google): %+v", len(got), got)
	}
	if got[0]["title"] != "Tài liệu Go" {
		t.Errorf("title = %q", got[0]["title"])
	}
	if !strings.HasPrefix(got[0]["url"], "https://golang.org") {
		t.Errorf("url = %q", got[0]["url"])
	}
	if got[0]["snippet"] == "" {
		t.Error("snippet rỗng")
	}
}

func TestParseGoogleResults_NoMatches(t *testing.T) {
	if got := parseGoogleResults("<html>không có kết quả</html>", 5); len(got) != 0 {
		t.Errorf("kết quả = %+v, want rỗng", got)
	}
}

// Google chặn/timeout → trả nil, không panic (caller tự fallback).
func TestSearchGoogleWeb_TransportError(t *testing.T) {
	got := searchGoogleWeb(t.Context(), &http.Client{Transport: errTransport{}}, "go", 5, "basic")
	if got != nil {
		t.Errorf("kết quả = %+v, want nil khi lỗi mạng", got)
	}
}

type errTransport struct{}

func (errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrServerClosed
}
