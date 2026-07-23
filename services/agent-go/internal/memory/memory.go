// Package memory định nghĩa bộ nhớ 3 tầng cho agent (design mục 10, P7).
// File này CHỈ chứa types + hàm thuần (merge/validate); phần store Mongo và
// recall/extract để phase sau (bson tag đã khai báo sẵn cho phase Mongo đó).
package memory

import (
	"errors"
	"fmt"
)

// MemoryType phân loại một mẩu bộ nhớ dài hạn.
type MemoryType string

const (
	MemoryPreference MemoryType = "preference" // sở thích/thiết lập của user
	MemoryFact       MemoryType = "fact"       // sự kiện/thông tin bền
	MemoryEntity     MemoryType = "entity"     // thực thể (người/nơi/vật)
)

// Item là một mẩu bộ nhớ dài hạn.
// bson tag phục vụ phase Mongo sau; json tag cho serialize API/stream.
type Item struct {
	Type       MemoryType `bson:"type"                json:"type"`
	Key        string     `bson:"key"                 json:"key"`
	Value      string     `bson:"value"               json:"value"`
	Confidence float64    `bson:"confidence"          json:"confidence"`
	Source     string     `bson:"source"              json:"source"`
	Embedding  []float64  `bson:"embedding,omitempty" json:"embedding,omitempty"`
}

// dedupKey khoá khử trùng theo (Type + Key).
type dedupKey struct {
	t MemoryType
	k string
}

// MergeMemories gộp 2 nguồn bộ nhớ (structured trước, rồi vector) và khử trùng
// theo (Type+Key): khi trùng giữ item có Confidence CAO hơn. Thứ tự ổn định —
// item structured giữ nguyên vị trí xuất hiện đầu tiên, vector chỉ thêm item MỚI.
func MergeMemories(structured, vector []Item) []Item {
	out := make([]Item, 0, len(structured)+len(vector))
	// index vị trí trong out theo dedupKey để cập nhật tại chỗ khi trùng.
	pos := make(map[dedupKey]int, len(structured)+len(vector))

	add := func(it Item) {
		key := dedupKey{t: it.Type, k: it.Key}
		if i, ok := pos[key]; ok {
			// Trùng: giữ item có confidence cao hơn, không đổi vị trí.
			if it.Confidence > out[i].Confidence {
				out[i] = it
			}
			return
		}
		pos[key] = len(out)
		out = append(out, it)
	}

	for _, it := range structured {
		add(it)
	}
	for _, it := range vector {
		add(it)
	}
	return out
}

// ValidateItem kiểm tra một Item hợp lệ: Type thuộc 3 giá trị cho phép,
// Key/Value không rỗng, Confidence trong [0,1].
func ValidateItem(it Item) error {
	switch it.Type {
	case MemoryPreference, MemoryFact, MemoryEntity:
	default:
		return fmt.Errorf("memory: type không hợp lệ: %q", it.Type)
	}
	if it.Key == "" {
		return errors.New("memory: key rỗng")
	}
	if it.Value == "" {
		return errors.New("memory: value rỗng")
	}
	if it.Confidence < 0 || it.Confidence > 1 {
		return fmt.Errorf("memory: confidence ngoài [0,1]: %v", it.Confidence)
	}
	return nil
}
