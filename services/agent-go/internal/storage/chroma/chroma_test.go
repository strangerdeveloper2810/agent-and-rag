package chroma

import (
	"testing"
)

func TestVectorStore_AddSearch(t *testing.T) {
	vs := NewVectorStore()

	vs.Add("doc1", []float32{1, 0, 0}, map[string]any{"title": "Doc 1"})
	vs.Add("doc2", []float32{0, 1, 0}, map[string]any{"title": "Doc 2"})
	vs.Add("doc3", []float32{0, 0, 1}, map[string]any{"title": "Doc 3"})

	// Query similar to doc1
	results := vs.Search([]float32{1, 0.1, 0.1}, 2)
	if len(results) != 2 {
		t.Fatalf("len = %d, want 2", len(results))
	}
	if results[0].ID != "doc1" {
		t.Errorf("top result = %q, want doc1", results[0].ID)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("results not sorted by score: %f <= %f", results[0].Score, results[1].Score)
	}
}

func TestVectorStore_Delete(t *testing.T) {
	vs := NewVectorStore()
	vs.Add("x", []float32{1, 0}, nil)
	vs.Delete("x")

	if vs.Size() != 0 {
		t.Errorf("size = %d, want 0", vs.Size())
	}

	results := vs.Search([]float32{1, 0}, 5)
	if len(results) != 0 {
		t.Errorf("search after delete = %d results, want 0", len(results))
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors
	if s := cosineSimilarity([]float32{1, 2, 3}, []float32{1, 2, 3}); s < 0.99 {
		t.Errorf("identical = %f, want ~1.0", s)
	}

	// Orthogonal vectors
	if s := cosineSimilarity([]float32{1, 0}, []float32{0, 1}); s > 0.01 {
		t.Errorf("orthogonal = %f, want ~0.0", s)
	}

	// Opposite vectors
	s := cosineSimilarity([]float32{1, 0}, []float32{-1, 0})
	if s > -0.99 {
		t.Errorf("opposite = %f, want ~-1.0", s)
	}
}
