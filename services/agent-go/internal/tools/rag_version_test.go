package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// --- rag.search: metadata + degrade khi chưa cấu hình ---

func TestRAGSearchTool_Metadata(t *testing.T) {
	tool := NewRAGSearchTool(nil, "db", "", false, false)

	if tool.Name() != "rag.search" {
		t.Errorf("Name() = %q, want rag.search", tool.Name())
	}
	if tool.Kind() != KindRead {
		t.Errorf("Kind() = %d, want KindRead", tool.Kind())
	}
	if !strings.Contains(tool.Description(), "RAG") {
		t.Errorf("Description() = %q", tool.Description())
	}

	var schema map[string]any
	if err := json.Unmarshal(tool.Schema(), &schema); err != nil {
		t.Fatalf("Schema() không hợp lệ: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
}

// Chưa có Mongo/Voyage → trả thông báo hướng dẫn, KHÔNG lỗi (degrade êm).
func TestRAGSearchTool_NotConfigured(t *testing.T) {
	tool := NewRAGSearchTool(nil, "db", "", false, false)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"x"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Content, "RAG not configured") {
		t.Errorf("Content = %q", res.Content)
	}
}

// --- Helper thuần của rag.search ---

func TestTokenizeKeywords(t *testing.T) {
	got := tokenizeKeywords("Tối ưu INDEX Postgres, 2024! a")
	// TrimFunc chỉ cắt ký tự không phải a-z0-9 ở HAI ĐẦU token: "Postgres," →
	// "postgres", "2024!" → "2024", "tối" giữ nguyên (dấu nằm giữa), "ưu" bị
	// cắt sạch còn rỗng, token 1 ký tự ("a") bị loại.
	want := map[string]bool{"tối": true, "index": true, "postgres": true, "2024": true}
	for _, tk := range got {
		if !want[tk] {
			t.Errorf("token lạ: %q (đủ bộ: %v)", tk, got)
		}
	}
	for tk := range want {
		found := false
		for _, g := range got {
			if g == tk {
				found = true
			}
		}
		if !found {
			t.Errorf("thiếu token %q trong %v", tk, got)
		}
	}

	if got := tokenizeKeywords(""); len(got) != 0 {
		t.Errorf("tokenizeKeywords(\"\") = %v, want rỗng", got)
	}
}

func TestRankOf(t *testing.T) {
	results := []ragSearchResult{
		{DocumentID: "a"}, {DocumentID: "b"}, {DocumentID: "c"},
	}

	if got := rankOf(results, "a"); got != 1 {
		t.Errorf("rankOf(a) = %d, want 1", got)
	}
	if got := rankOf(results, "c"); got != 3 {
		t.Errorf("rankOf(c) = %d, want 3", got)
	}
	if got := rankOf(results, "không-có"); got != 0 {
		t.Errorf("rankOf(lạ) = %d, want 0", got)
	}
	if got := rankOf(nil, "a"); got != 0 {
		t.Errorf("rankOf(nil) = %d, want 0", got)
	}
}

// Rerank đẩy kết quả trùng nhiều từ khoá lên trên dù điểm gốc thấp hơn.
func TestRerankKeyword(t *testing.T) {
	tool := &ragSearchTool{}

	results := []ragSearchResult{
		{DocumentID: "không-liên-quan", Score: 0.9, Snippet: "chuyện bâng quơ"},
		{DocumentID: "khớp", Score: 0.6, Snippet: "postgres index tuning postgres index"},
	}

	got := tool.rerankKeyword("postgres index", results)

	if got[0].DocumentID != "khớp" {
		t.Errorf("thứ tự sau rerank = %+v, want doc 'khớp' đứng đầu", got)
	}
}

func TestRerankKeyword_EmptyQuery(t *testing.T) {
	tool := &ragSearchTool{}

	results := []ragSearchResult{{DocumentID: "a", Score: 0.5, Snippet: "gì đó"}}
	got := tool.rerankKeyword("", results)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	// Query rỗng → overlap 0 → điểm chỉ còn 70% điểm gốc.
	if got[0].Score != 0.35 {
		t.Errorf("Score = %v, want 0.35", got[0].Score)
	}
}

// --- version tool ---

// stubRT trả response cấu hình sẵn theo tiền tố URL.
type stubRT struct {
	body   string
	status int
	gotURL string
	err    error
}

func (s *stubRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.gotURL = req.URL.String()
	code := s.status
	if code == 0 {
		code = http.StatusOK
	}
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func versionToolWith(rt http.RoundTripper) *VersionTool {
	tool := NewVersionTool()
	tool.httpClient = &http.Client{Transport: rt}
	return tool
}

func TestVersionTool_NPM(t *testing.T) {
	rt := &stubRT{body: `{"name":"react","version":"19.2.0"}`}
	tool := versionToolWith(rt)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"source":"npm","package":"react"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Content, `"latest":"19.2.0"`) || !strings.Contains(res.Content, `"source":"npm"`) {
		t.Errorf("Content = %q", res.Content)
	}
	if !strings.Contains(rt.gotURL, "registry.npmjs.org/react/latest") {
		t.Errorf("URL = %q", rt.gotURL)
	}
}

func TestVersionTool_GitHub(t *testing.T) {
	rt := &stubRT{body: `{"tag_name":"v1.24.0","name":"Go 1.24","published_at":"2025-02-11T00:00:00Z"}`}
	tool := versionToolWith(rt)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"source":"github","owner":"golang","repo":"go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Content, "v1.24.0") {
		t.Errorf("Content = %q", res.Content)
	}
	if !strings.Contains(rt.gotURL, "repos/golang/go/releases/latest") {
		t.Errorf("URL = %q", rt.gotURL)
	}
}

func TestVersionTool_Errors(t *testing.T) {
	tool := versionToolWith(&stubRT{body: "{}"})

	if _, err := tool.Execute(context.Background(), json.RawMessage(`{không phải json`)); err == nil {
		t.Error("args hỏng = nil error, want lỗi")
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"source":"gitlab"}`)); err == nil {
		t.Error("source lạ = nil error, want lỗi")
	}

	// Lỗi mạng.
	netErrTool := versionToolWith(&stubRT{err: http.ErrServerClosed})
	if _, err := netErrTool.Execute(context.Background(), json.RawMessage(`{"source":"npm","package":"react"}`)); err == nil {
		t.Error("lỗi mạng npm = nil error, want lỗi")
	}
	if _, err := netErrTool.Execute(context.Background(), json.RawMessage(`{"source":"github","owner":"a","repo":"b"}`)); err == nil {
		t.Error("lỗi mạng github = nil error, want lỗi")
	}

	// JSON trả về hỏng.
	badJSON := versionToolWith(&stubRT{body: "không-phải-json"})
	if _, err := badJSON.Execute(context.Background(), json.RawMessage(`{"source":"npm","package":"react"}`)); err == nil {
		t.Error("JSON hỏng = nil error, want lỗi parse")
	}
}
