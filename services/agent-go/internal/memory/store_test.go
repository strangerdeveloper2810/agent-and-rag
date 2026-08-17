package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// stubEmbedder trả về vector cố định theo text; nếu err != nil thì mọi lời gọi
// Embed đều trả lỗi. Text không có trong map nhận vector {0, 0}.
type stubEmbedder struct {
	vectors map[string][]float64
	err     error
}

func (s *stubEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([][]float64, len(texts))
	for i, t := range texts {
		if v, ok := s.vectors[t]; ok {
			out[i] = v
		} else {
			out[i] = []float64{0, 0}
		}
	}
	return out, nil
}

func TestEmbedderFunc(t *testing.T) {
	called := 0
	f := EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
		called++
		return [][]float64{{1, 0}}, nil
	})

	vecs, err := f.Embed(context.Background(), []string{"x"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if called != 1 {
		t.Fatalf("called = %d, want 1", called)
	}
	if len(vecs) != 1 || vecs[0][0] != 1 || vecs[0][1] != 0 {
		t.Fatalf("vecs = %v, want [[1 0]]", vecs)
	}
}

func TestStore_SetGetDeleteLen(t *testing.T) {
	s := NewStore()
	if s.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", s.Len())
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("Get(missing) should return ok=false")
	}

	s.Set("k1", "v1")
	s.Set("k2", "v2")
	if s.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", s.Len())
	}
	if v, ok := s.Get("k1"); !ok || v != "v1" {
		t.Fatalf("Get(k1) = (%q, %v), want (v1, true)", v, ok)
	}

	// Ghi đè giữ nguyên số lượng key.
	s.Set("k1", "v1b")
	if v, _ := s.Get("k1"); v != "v1b" {
		t.Fatalf("Get(k1) sau ghi đè = %q, want v1b", v)
	}
	if s.Len() != 2 {
		t.Fatalf("Len() sau ghi đè = %d, want 2", s.Len())
	}

	s.Delete("k2")
	if _, ok := s.Get("k2"); ok {
		t.Fatal("Get(k2) sau Delete nên không tồn tại")
	}
	s.Delete("missing") // xoá key không tồn tại — không lỗi
	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", s.Len())
	}
}

func TestStore_Search(t *testing.T) {
	s := NewStore()
	s.Set("user_name", "Linh")
	s.Set("Note", "Thích Trà Đào")

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"khớp theo value", "linh", 1},
		{"khớp theo key không phân biệt hoa thường", "NOTE", 1},
		{"khớp value tiếng Việt không phân biệt hoa thường", "trà đào", 1},
		{"không khớp", "không tồn tại", 0},
		{"query rỗng khớp tất cả", "", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.Search(tc.query)
			if len(got) != tc.want {
				t.Fatalf("Search(%q) = %v, want %d kết quả", tc.query, got, tc.want)
			}
		})
	}

	if got := s.Search("linh"); got["user_name"] != "Linh" {
		t.Fatalf("Search(linh)[user_name] = %q, want Linh", got["user_name"])
	}
}

func TestStore_SetEmbedderStoresVectors(t *testing.T) {
	s := NewStore()
	s.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{"banana": {1, 0}}})
	s.Set("a", "banana")

	res, err := s.SemanticSearch("banana", 5)
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("len = %d, want 1", len(res))
	}
	if res[0].Key != "a" || res[0].Value != "banana" {
		t.Fatalf("res[0] = %+v, want key a value banana", res[0])
	}
	if res[0].Confidence != 1 {
		t.Fatalf("Confidence = %v, want 1", res[0].Confidence)
	}
	if len(res[0].Embedding) != 2 {
		t.Fatalf("Embedding = %v, want vector đã lưu", res[0].Embedding)
	}
}

func TestStore_SetEmbedderNilDisablesSemantic(t *testing.T) {
	s := NewStore()
	s.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{"banana": {1, 0}}})
	s.Set("a", "banana")

	s.SetEmbedder(nil)
	res, err := s.SemanticSearch("banana", 5)
	if err != nil || res != nil {
		t.Fatalf("SemanticSearch sau khi disable = (%v, %v), want (nil, nil)", res, err)
	}

	// Set sau khi disable không lưu embedding → chỉ entry "a" (đã có vector từ
	// trước) được semantic trả về, entry "b" bị bỏ qua.
	s.Set("b", "banana")
	s.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{"banana": {1, 0}}})
	res, err = s.SemanticSearch("banana", 5)
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(res) != 1 || res[0].Key != "a" {
		t.Fatalf("res = %+v, want chỉ entry a (b không có embedding)", res)
	}
}

func TestStore_SetEmbedderErrorStillStoresValue(t *testing.T) {
	s := NewStore()
	s.SetEmbedder(&stubEmbedder{err: errors.New("embed down")})
	s.Set("a", "banana")

	if v, ok := s.Get("a"); !ok || v != "banana" {
		t.Fatalf("Get(a) = (%q, %v), want (banana, true) dù embedder lỗi", v, ok)
	}
	// Keyword search vẫn hoạt động; semantic trả lỗi từ embedder.
	if got := s.Search("banana"); len(got) != 1 {
		t.Fatalf("Search = %v, want 1 kết quả", got)
	}
	if _, err := s.SemanticSearch("banana", 5); err == nil {
		t.Fatal("SemanticSearch nên trả lỗi embedder")
	}
}

func TestStore_SemanticSearch_TopKSelection(t *testing.T) {
	s := NewStore()
	s.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{
		"banana": {1, 0},     // cos với query {1,0} = 1
		"apple":  {0, 1},     // cos = 0
		"orange": {-1, 0},    // cos = -1
		"mango":  {0.6, 0.8}, // cos = 0.6
	}})
	s.Set("a", "banana")
	s.Set("b", "apple")
	s.Set("c", "orange")
	s.Set("d", "mango")

	res, err := s.SemanticSearch("banana", 2)
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("len = %d, want 2 (topK)", len(res))
	}
	// Thứ tự giảm dần theo cos: banana (1) trước mango (0.6).
	if res[0].Key != "a" || res[1].Key != "d" {
		t.Fatalf("thứ tự = %v, want [a d]", []string{res[0].Key, res[1].Key})
	}
	if res[0].Confidence != 1 {
		t.Fatalf("res[0].Confidence = %v, want 1", res[0].Confidence)
	}
	if res[1].Confidence != 0.6 {
		t.Fatalf("res[1].Confidence = %v, want 0.6", res[1].Confidence)
	}
}

func TestStore_SemanticSearch_TopKClamped(t *testing.T) {
	s := NewStore()
	s.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{
		"banana": {1, 0},
		"apple":  {0, 1},
	}})
	s.Set("a", "banana")
	s.Set("b", "apple")

	// topK > số ứng viên → trả toàn bộ.
	res, err := s.SemanticSearch("banana", 10)
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("len = %d, want 2", len(res))
	}
	if res[0].Confidence <= res[1].Confidence {
		t.Fatalf("kết quả không giảm dần: %v", []float64{res[0].Confidence, res[1].Confidence})
	}
}

func TestStore_SemanticSearch_EdgeCases(t *testing.T) {
	s := NewStore()
	s.Set("a", "banana")

	// Không có embedder.
	res, err := s.SemanticSearch("banana", 5)
	if err != nil || res != nil {
		t.Fatalf("không embedder = (%v, %v), want (nil, nil)", res, err)
	}

	s.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{"banana": {1, 0}}})
	// topK <= 0 → rỗng.
	res, err = s.SemanticSearch("banana", 0)
	if err != nil || res != nil {
		t.Fatalf("topK=0 = (%v, %v), want (nil, nil)", res, err)
	}
	// topK âm → rỗng.
	res, err = s.SemanticSearch("banana", -1)
	if err != nil || res != nil {
		t.Fatalf("topK=-1 = (%v, %v), want (nil, nil)", res, err)
	}
	// Embedder trả vector rỗng → nil, nil.
	s.SetEmbedder(EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
		return nil, nil
	}))
	res, err = s.SemanticSearch("banana", 5)
	if err != nil || res != nil {
		t.Fatalf("vector rỗng = (%v, %v), want (nil, nil)", res, err)
	}
}

func TestStore_SemanticSearch_SkipsEntriesWithoutEmbedding(t *testing.T) {
	s := NewStore()
	s.Set("a", "pear") // chưa có embedder → không có embedding

	s.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{
		"banana": {1, 0},
		"pear":   {1, 0},
	}})
	s.Set("b", "banana")

	res, err := s.SemanticSearch("banana", 5)
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(res) != 1 || res[0].Key != "b" {
		t.Fatalf("res = %+v, want chỉ entry b (a không có embedding)", res)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{"giống nhau", []float64{1, 0}, []float64{1, 0}, 1},
		{"trực giao", []float64{1, 0}, []float64{0, 1}, 0},
		{"ngược hướng", []float64{1, 0}, []float64{-1, 0}, -1},
		{"tỉ lệ dương", []float64{2, 0}, []float64{4, 0}, 1},
		{"vector không", []float64{0, 0}, []float64{1, 1}, 0},
		{"cả hai rỗng", []float64{}, []float64{}, 0},
		{"khác độ dài", []float64{1, 0, 0}, []float64{1, 0}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cosineSimilarity(tc.a, tc.b); got != tc.want {
				t.Fatalf("cosineSimilarity = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				key := fmt.Sprintf("k%d", (i+j)%16)
				s.Set(key, "v")
				_, _ = s.Get(key)
				_ = s.Search("v")
				if (i+j)%32 == 0 {
					s.Delete(key)
				}
			}
		}(i)
	}
	wg.Wait()
	if s.Len() > 16 {
		t.Fatalf("Len() = %d, want <= 16", s.Len())
	}
}
