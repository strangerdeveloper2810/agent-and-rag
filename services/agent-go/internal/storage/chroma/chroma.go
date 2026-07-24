// Package chroma cung cấp in-memory vector store cho semantic search.
// MVP dùng cosine similarity trên in-memory store.
// Sau này có thể thay bằng Chroma embedded hoặc pgvector.
package chroma

import (
	"math"
	"sort"
)

// SearchResult represents a vector search result.
type SearchResult struct {
	ID       string
	Score    float64
	Metadata map[string]any
}

// VectorStore là in-memory vector store với cosine similarity search.
type VectorStore struct {
	entries map[string]*entry
}

type entry struct {
	embedding []float32
	metadata  map[string]any
}

// NewVectorStore tạo vector store rỗng.
func NewVectorStore() *VectorStore {
	return &VectorStore{entries: make(map[string]*entry)}
}

// Add thêm hoặc cập nhật vector embedding cho id.
func (vs *VectorStore) Add(id string, embedding []float32, metadata map[string]any) {
	vs.entries[id] = &entry{
		embedding: embedding,
		metadata:  metadata,
	}
}

// Delete xóa vector theo id.
func (vs *VectorStore) Delete(id string) {
	delete(vs.entries, id)
}

// Search tìm topK vectors gần nhất bằng cosine similarity.
// Trả về kết quả sắp xếp giảm dần theo similarity score (0-1).
func (vs *VectorStore) Search(query []float32, topK int) []SearchResult {
	if topK <= 0 {
		topK = 5
	}

	type scored struct {
		id    string
		score float64
		meta  map[string]any
	}

	var results []scored
	for id, e := range vs.entries {
		sim := cosineSimilarity(query, e.embedding)
		results = append(results, scored{id: id, score: sim, meta: e.metadata})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{
			ID:       r.id,
			Score:    r.score,
			Metadata: r.meta,
		}
	}
	return out
}

// Size trả về số lượng vectors đã lưu.
func (vs *VectorStore) Size() int {
	return len(vs.entries)
}

// cosineSimilarity tính cosine similarity giữa 2 vectors.
// Trả về giá trị [-1, 1]; 1 = giống hệt, 0 = vuông góc, -1 = đối nghịch.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
