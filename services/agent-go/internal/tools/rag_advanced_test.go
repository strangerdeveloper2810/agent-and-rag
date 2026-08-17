package tools

import (
	"context"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// mockLLMProvider trả về đúng 1 response cố định — dùng để test rerankLLM /
// generateHypotheticalAnswer mà không cần gọi API thật.
type mockLLMProvider struct {
	response string
	err      error
}

func (m *mockLLMProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: m.response}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (m *mockLLMProvider) Name() string { return "mock-llm" }

// --- buildParentWindowFilter (Parent Document Retrieval) ---

func TestBuildParentWindowFilter_ScopesToTenant(t *testing.T) {
	filter := buildParentWindowFilter("doc-1", 5, 1, "tenant-a")
	filterHasTenant(t, filter, "tenant-a")

	var gotRange bson.D
	for _, e := range filter {
		if e.Key == "chunkIndex" {
			gotRange = e.Value.(bson.D)
		}
	}
	if gotRange == nil {
		t.Fatal("filter thiếu điều kiện chunkIndex")
	}
	for _, e := range gotRange {
		switch e.Key {
		case "$gte":
			if e.Value != 4 {
				t.Errorf("$gte = %v, want 4 (chunkIndex-radius)", e.Value)
			}
		case "$lte":
			if e.Value != 6 {
				t.Errorf("$lte = %v, want 6 (chunkIndex+radius)", e.Value)
			}
		}
	}
}

func TestBuildParentWindowFilter_DefaultTenantNotScoped(t *testing.T) {
	filterHasNoTenant(t, buildParentWindowFilter("doc-1", 0, 1, ""))
	filterHasNoTenant(t, buildParentWindowFilter("doc-1", 0, 1, "default"))
}

func TestBuildParentWindowFilter_ChunkIndexZero_NoUnderflowIssue(t *testing.T) {
	// chunkIndex=0, radius=1 -> $gte phải là -1 (Mongo tự không match gì, không
	// panic/lỗi do "âm") chứ KHÔNG được clamp về 0 rồi lỡ khớp nhầm range.
	filter := buildParentWindowFilter("doc-1", 0, 1, "")
	for _, e := range filter {
		if e.Key != "chunkIndex" {
			continue
		}
		for _, sub := range e.Value.(bson.D) {
			if sub.Key == "$gte" && sub.Value != -1 {
				t.Errorf("$gte = %v, want -1", sub.Value)
			}
		}
	}
}

// --- parseRerankOrder ---

func TestParseRerankOrder_ValidPermutation(t *testing.T) {
	order, ok := parseRerankOrder(`[2,0,3,1]`, 4)
	if !ok {
		t.Fatal("expected ok=true cho hoán vị hợp lệ")
	}
	want := []int{2, 0, 3, 1}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %d, want %d", i, order[i], want[i])
		}
	}
}

func TestParseRerankOrder_StripsMarkdownFence(t *testing.T) {
	_, ok := parseRerankOrder("```json\n[1,0]\n```", 2)
	if !ok {
		t.Fatal("expected ok=true khi output bọc trong ```json fence")
	}
}

func TestParseRerankOrder_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		n    int
	}{
		{"không phải JSON", "tôi không biết", 3},
		{"thiếu phần tử", "[0,1]", 3},
		{"thừa phần tử", "[0,1,2,3]", 3},
		{"trùng index", "[0,0,1]", 3},
		{"index ngoài phạm vi", "[0,1,5]", 3},
		{"index âm", "[-1,0,1]", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := parseRerankOrder(tc.raw, tc.n); ok {
				t.Errorf("expected ok=false cho raw=%q n=%d", tc.raw, tc.n)
			}
		})
	}
}

// --- truncateRunes (UTF-8 safety) ---

func TestTruncateRunes_UTF8Safe(t *testing.T) {
	// Tiếng Việt có dấu: mỗi ký tự có thể chiếm 2-3 byte UTF-8. Cắt theo byte
	// (thay vì rune) sẽ chẻ đôi ký tự, tạo chuỗi lỗi encoding ở cuối.
	s := "Đây là một đoạn văn bản tiếng Việt có dấu để test cắt chuỗi"
	got := truncateRunes(s, 10)
	wantRuneCount := 10
	if gotRuneCount := len([]rune(got)); gotRuneCount != wantRuneCount {
		t.Errorf("truncateRunes trả %d rune, want %d (got=%q)", gotRuneCount, wantRuneCount, got)
	}
	// Kết quả phải là UTF-8 hợp lệ (string([]rune) luôn đảm bảo điều này,
	// nhưng assert rõ để không hồi quy nếu code đổi sang cắt byte).
	if !isValidUTF8(got) {
		t.Errorf("truncateRunes trả chuỗi UTF-8 không hợp lệ: %q", got)
	}
}

func TestTruncateRunes_ShorterThanMax_Unchanged(t *testing.T) {
	s := "ngắn"
	if got := truncateRunes(s, 100); got != s {
		t.Errorf("truncateRunes(%q, 100) = %q, want unchanged", s, got)
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

// --- rerankLLM ---

func TestRerankLLM_ReordersUsingLLMResponse(t *testing.T) {
	tool := &ragSearchTool{
		prov:  &mockLLMProvider{response: "[2,0,1]"},
		model: "mock-model",
	}
	results := []ragSearchResult{
		{DocumentID: "a", Snippet: "kém liên quan nhất"},
		{DocumentID: "b", Snippet: "liên quan trung bình"},
		{DocumentID: "c", Snippet: "liên quan nhất"},
	}

	got := tool.rerankLLM(context.Background(), "câu hỏi test", results)

	want := []string{"c", "a", "b"} // index [2,0,1]
	for i, w := range want {
		if got[i].DocumentID != w {
			t.Errorf("got[%d].DocumentID = %q, want %q (full=%v)", i, got[i].DocumentID, w, got)
		}
	}
}

func TestRerankLLM_GenerateError_KeepsOriginalOrder(t *testing.T) {
	tool := &ragSearchTool{
		prov:  &mockLLMProvider{err: context.DeadlineExceeded},
		model: "mock-model",
	}
	results := []ragSearchResult{
		{DocumentID: "a"}, {DocumentID: "b"},
	}

	got := tool.rerankLLM(context.Background(), "q", results)
	if got[0].DocumentID != "a" || got[1].DocumentID != "b" {
		t.Errorf("expected thứ tự gốc khi Generate lỗi, got %v", got)
	}
}

func TestRerankLLM_MalformedResponse_KeepsOriginalOrder(t *testing.T) {
	tool := &ragSearchTool{
		prov:  &mockLLMProvider{response: "không phải JSON hợp lệ"},
		model: "mock-model",
	}
	results := []ragSearchResult{
		{DocumentID: "a"}, {DocumentID: "b"},
	}

	got := tool.rerankLLM(context.Background(), "q", results)
	if got[0].DocumentID != "a" || got[1].DocumentID != "b" {
		t.Errorf("expected thứ tự gốc khi response không parse được, got %v", got)
	}
}

// --- generateHypotheticalAnswer (HyDE) ---

func TestGenerateHypotheticalAnswer_ReturnsTrimmedText(t *testing.T) {
	tool := &ragSearchTool{
		prov:  &mockLLMProvider{response: "  Đây là câu trả lời giả định.  \n"},
		model: "mock-model",
	}
	got := tool.generateHypotheticalAnswer(context.Background(), "postgres index là gì")
	if got != "Đây là câu trả lời giả định." {
		t.Errorf("generateHypotheticalAnswer = %q", got)
	}
}

func TestGenerateHypotheticalAnswer_GenerateError_ReturnsEmpty(t *testing.T) {
	tool := &ragSearchTool{
		prov:  &mockLLMProvider{err: context.DeadlineExceeded},
		model: "mock-model",
	}
	got := tool.generateHypotheticalAnswer(context.Background(), "q")
	if got != "" {
		t.Errorf("expected rỗng khi Generate lỗi (Execute() tự fallback dùng câu hỏi gốc), got %q", got)
	}
}
