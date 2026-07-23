package memory

import (
	"reflect"
	"testing"
)

func TestMergeMemories_DuplicateKeepsHigherConfidence(t *testing.T) {
	structured := []Item{
		{Type: MemoryPreference, Key: "lang", Value: "vi", Confidence: 0.6, Source: "structured"},
	}
	vector := []Item{
		{Type: MemoryPreference, Key: "lang", Value: "en", Confidence: 0.9, Source: "vector"},
	}

	got := MergeMemories(structured, vector)

	if len(got) != 1 {
		t.Fatalf("want 1 item sau khử trùng, got %d", len(got))
	}
	if got[0].Confidence != 0.9 || got[0].Value != "en" || got[0].Source != "vector" {
		t.Fatalf("want item confidence cao hơn (vector), got %+v", got[0])
	}
}

func TestMergeMemories_DuplicateKeepsStructuredWhenHigher(t *testing.T) {
	structured := []Item{
		{Type: MemoryFact, Key: "city", Value: "Hanoi", Confidence: 0.95, Source: "structured"},
	}
	vector := []Item{
		{Type: MemoryFact, Key: "city", Value: "HCMC", Confidence: 0.5, Source: "vector"},
	}

	got := MergeMemories(structured, vector)

	if len(got) != 1 {
		t.Fatalf("want 1 item, got %d", len(got))
	}
	if got[0].Confidence != 0.95 || got[0].Value != "Hanoi" {
		t.Fatalf("want giữ structured (confidence cao hơn), got %+v", got[0])
	}
}

func TestMergeMemories_NoOverlapKeepsBoth(t *testing.T) {
	structured := []Item{
		{Type: MemoryPreference, Key: "lang", Value: "vi", Confidence: 0.6},
	}
	vector := []Item{
		{Type: MemoryFact, Key: "city", Value: "Hanoi", Confidence: 0.8},
	}

	got := MergeMemories(structured, vector)

	if len(got) != 2 {
		t.Fatalf("want 2 item (không trùng), got %d", len(got))
	}
}

func TestMergeMemories_StableOrder(t *testing.T) {
	structured := []Item{
		{Type: MemoryPreference, Key: "a", Value: "1", Confidence: 0.5},
		{Type: MemoryPreference, Key: "b", Value: "2", Confidence: 0.5},
	}
	vector := []Item{
		// trùng "a" nhưng confidence cao hơn → cập nhật tại chỗ, giữ vị trí đầu.
		{Type: MemoryPreference, Key: "a", Value: "1b", Confidence: 0.9},
		// mới → thêm cuối.
		{Type: MemoryFact, Key: "c", Value: "3", Confidence: 0.5},
	}

	got := MergeMemories(structured, vector)

	wantKeys := []string{"a", "b", "c"}
	gotKeys := make([]string, len(got))
	for i, it := range got {
		gotKeys[i] = it.Key
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("thứ tự không ổn định: want %v, got %v", wantKeys, gotKeys)
	}
	if got[0].Value != "1b" || got[0].Confidence != 0.9 {
		t.Fatalf("item trùng phải được cập nhật tại chỗ, got %+v", got[0])
	}
}

func TestValidateItem(t *testing.T) {
	tests := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{
			name: "hợp lệ",
			item: Item{Type: MemoryPreference, Key: "lang", Value: "vi", Confidence: 0.7},
		},
		{
			name: "hợp lệ biên confidence 0",
			item: Item{Type: MemoryFact, Key: "k", Value: "v", Confidence: 0},
		},
		{
			name: "hợp lệ biên confidence 1",
			item: Item{Type: MemoryEntity, Key: "k", Value: "v", Confidence: 1},
		},
		{
			name:    "type sai",
			item:    Item{Type: MemoryType("random"), Key: "k", Value: "v", Confidence: 0.5},
			wantErr: true,
		},
		{
			name:    "key rỗng",
			item:    Item{Type: MemoryFact, Key: "", Value: "v", Confidence: 0.5},
			wantErr: true,
		},
		{
			name:    "value rỗng",
			item:    Item{Type: MemoryFact, Key: "k", Value: "", Confidence: 0.5},
			wantErr: true,
		},
		{
			name:    "confidence dưới 0",
			item:    Item{Type: MemoryFact, Key: "k", Value: "v", Confidence: -0.1},
			wantErr: true,
		},
		{
			name:    "confidence trên 1",
			item:    Item{Type: MemoryFact, Key: "k", Value: "v", Confidence: 1.1},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateItem(tc.item)
			if tc.wantErr && err == nil {
				t.Fatalf("want lỗi, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}
