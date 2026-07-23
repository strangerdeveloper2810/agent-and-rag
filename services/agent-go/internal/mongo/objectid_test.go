package mongo

import "testing"

func TestToObjectID(t *testing.T) {
	if _, err := ToObjectID("64b7f0000000000000000000"); err != nil {
		t.Fatalf("hex hợp lệ phải ok, lỗi: %v", err)
	}
	if _, err := ToObjectID("abc"); err == nil {
		t.Fatal("id sai định dạng phải trả error")
	}
	if _, err := ToObjectID(""); err == nil {
		t.Fatal("id rỗng phải trả error")
	}
}
