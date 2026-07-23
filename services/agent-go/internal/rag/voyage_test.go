package rag

import "testing"

func TestBuildEmbeddingRequest(t *testing.T) {
	r := buildEmbeddingRequest([]string{"a", "b"}, "document")
	if r.Model != "voyage-3" {
		t.Fatalf("model: %s", r.Model)
	}
	if r.InputType != "document" {
		t.Fatalf("input_type: %s", r.InputType)
	}
	if len(r.Input) != 2 {
		t.Fatalf("input len: %d", len(r.Input))
	}
}

func TestBatchTexts(t *testing.T) {
	got := batchTexts([]string{"a", "b", "c", "d", "e"}, 2)
	if len(got) != 3 {
		t.Fatalf("want 3 batches, got %d", len(got))
	}
	if len(got[2]) != 1 {
		t.Fatalf("last batch len: %d", len(got[2]))
	}
	if len(batchTexts(nil, 96)) != 0 {
		t.Fatal("empty input → 0 batches")
	}
	if len(batchTexts([]string{"a"}, 0)) != 1 {
		t.Fatal("size<1 → coi như 1")
	}
}
