package sqlite

import (
	"path/filepath"
	"testing"
)

// openTestStore mở store trên file tạm (FTS5 cần file thật hoạt động ổn định).
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// indexMessage nạp nội dung vào bảng FTS ngoài (content=messages nên không tự đồng bộ).
func indexMessage(t *testing.T, s *Store, rowID int64, content string) {
	t.Helper()
	if _, err := s.db.Exec("INSERT INTO messages_fts(rowid, content) VALUES (?, ?)", rowID, content); err != nil {
		t.Fatalf("index message: %v", err)
	}
}

func indexMemory(t *testing.T, s *Store, rowID int64, value string) {
	t.Helper()
	if _, err := s.db.Exec("INSERT INTO memories_fts(rowid, value) VALUES (?, ?)", rowID, value); err != nil {
		t.Fatalf("index memory: %v", err)
	}
}

func TestOpen_BadPath(t *testing.T) {
	// Thư mục không tồn tại → migrate thất bại.
	if _, err := Open(filepath.Join(t.TempDir(), "không-có-thư-mục", "x.db")); err == nil {
		t.Fatal("Open đường dẫn hỏng = nil error, want lỗi")
	}
}

func TestSearchMessages(t *testing.T) {
	s := openTestStore(t)

	conv, err := s.CreateConversation("hội thoại")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	msg, err := s.AddMessage(conv.ID, "user", "cách tối ưu index postgres", "", "")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	other, err := s.AddMessage(conv.ID, "assistant", "chuyện không liên quan", "", "")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	indexMessage(t, s, msg.ID, "cách tối ưu index postgres")
	indexMessage(t, s, other.ID, "chuyện không liên quan")

	got, err := s.SearchMessages("postgres", 10)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("kết quả = %d, want 1", len(got))
	}
	if got[0].ID != msg.ID || got[0].Role != "user" {
		t.Errorf("message = %+v", got[0])
	}
}

func TestSearchMessages_DefaultLimit(t *testing.T) {
	s := openTestStore(t)
	conv, _ := s.CreateConversation("c")

	for i := 0; i < 15; i++ {
		msg, err := s.AddMessage(conv.ID, "user", "golang tips", "", "")
		if err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
		indexMessage(t, s, msg.ID, "golang tips")
	}

	// limit <= 0 → mặc định 10.
	got, err := s.SearchMessages("golang", 0)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(got) != 10 {
		t.Errorf("kết quả = %d, want 10 (limit mặc định)", len(got))
	}

	got, err = s.SearchMessages("golang", 3)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("kết quả = %d, want 3", len(got))
	}
}

func TestSearchMessages_NoMatch(t *testing.T) {
	s := openTestStore(t)

	got, err := s.SearchMessages("chẳngcógìtrùng", 5)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("kết quả = %d, want 0", len(got))
	}
}

// Cú pháp FTS5 sai phải trả lỗi có ngữ cảnh, không panic.
func TestSearchMessages_InvalidQuery(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.SearchMessages(`"chưa đóng ngoặc`, 5); err == nil {
		t.Fatal("query FTS hỏng = nil error, want lỗi")
	}
}

func TestSearchMemories(t *testing.T) {
	s := openTestStore(t)

	if err := s.UpsertMemory("preference", "cà phê", "user thích cà phê sữa đá", 0.9, "ai_extracted"); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}
	if err := s.UpsertMemory("fact", "quê", "user ở Hà Nội", 1.0, "user_stated"); err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}

	mem, err := s.LookupMemory("preference", "cà phê")
	if err != nil || mem == nil {
		t.Fatalf("LookupMemory: %v / %+v", err, mem)
	}
	indexMemory(t, s, mem.ID, mem.Value)

	got, err := s.SearchMemories("sữa", 10)
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(got) != 1 || got[0].Key != "cà phê" {
		t.Errorf("kết quả = %+v", got)
	}
}

func TestSearchMemories_DefaultLimitAndNoMatch(t *testing.T) {
	s := openTestStore(t)

	got, err := s.SearchMemories("khôngcó", 0)
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("kết quả = %d, want 0", len(got))
	}

	if _, err := s.SearchMemories(`"hỏng`, 5); err == nil {
		t.Fatal("query FTS hỏng = nil error, want lỗi")
	}
}

// Sau khi Close, mọi truy vấn phải trả lỗi (không panic).
func TestQueriesAfterClose(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := s.CreateConversation("x"); err == nil {
		t.Error("CreateConversation sau Close = nil error")
	}
	if _, err := s.GetConversation("x"); err == nil {
		t.Error("GetConversation sau Close = nil error")
	}
	if _, err := s.ListConversations(10); err == nil {
		t.Error("ListConversations sau Close = nil error")
	}
	if _, err := s.AddMessage("c", "user", "x", "", ""); err == nil {
		t.Error("AddMessage sau Close = nil error")
	}
	if _, err := s.GetMessages("c"); err == nil {
		t.Error("GetMessages sau Close = nil error")
	}
	if _, err := s.SearchMessages("x", 10); err == nil {
		t.Error("SearchMessages sau Close = nil error")
	}
	if err := s.UpsertMemory("fact", "k", "v", 1, "src"); err == nil {
		t.Error("UpsertMemory sau Close = nil error")
	}
	if _, err := s.LookupMemory("fact", "k"); err == nil {
		t.Error("LookupMemory sau Close = nil error")
	}
	if _, err := s.SearchMemories("x", 10); err == nil {
		t.Error("SearchMemories sau Close = nil error")
	}
	if _, err := s.ListMemoriesByType("fact"); err == nil {
		t.Error("ListMemoriesByType sau Close = nil error")
	}
}

// Ràng buộc schema: role lạ bị CHECK constraint chặn.
func TestAddMessage_InvalidRole(t *testing.T) {
	s := openTestStore(t)
	conv, _ := s.CreateConversation("c")

	if _, err := s.AddMessage(conv.ID, "kẻ lạ mặt", "x", "", ""); err == nil {
		t.Fatal("role không hợp lệ = nil error, want lỗi CHECK constraint")
	}
}

func TestUpsertMemory_InvalidType(t *testing.T) {
	s := openTestStore(t)

	if err := s.UpsertMemory("kiểu-lạ", "k", "v", 1, "src"); err == nil {
		t.Fatal("type không hợp lệ = nil error, want lỗi CHECK constraint")
	}
}

func TestGetConversation_NotFound(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.GetConversation("không-tồn-tại"); err == nil {
		t.Fatal("GetConversation id lạ = nil error, want lỗi not found")
	}
}
