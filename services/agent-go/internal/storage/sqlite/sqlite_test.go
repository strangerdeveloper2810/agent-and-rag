package sqlite

import (
	"testing"
)

func TestConversations(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Create
	conv, err := store.CreateConversation("Test Chat")
	if err != nil {
		t.Fatal(err)
	}
	if conv.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if conv.Title != "Test Chat" {
		t.Errorf("title = %q, want %q", conv.Title, "Test Chat")
	}

	// Get
	got, err := store.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != conv.ID {
		t.Errorf("ID mismatch: %q vs %q", got.ID, conv.ID)
	}

	// List
	list, err := store.ListConversations(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
}

func TestMessages(t *testing.T) {
	store, _ := Open(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation("Test")

	// Add messages
	m1, err := store.AddMessage(conv.ID, "user", "Hello", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m1.Role != "user" || m1.Content != "Hello" {
		t.Errorf("m1 = %+v", m1)
	}

	_, err = store.AddMessage(conv.ID, "assistant", "Hi!", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Get messages
	msgs, err := store.GetMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Content != "Hello" || msgs[1].Content != "Hi!" {
		t.Errorf("order wrong: %+v", msgs)
	}
}

func TestMemories(t *testing.T) {
	store, _ := Open(":memory:")
	defer store.Close()

	// Upsert
	err := store.UpsertMemory("preference", "coffee", "black, no sugar", 0.9, "ai_extracted")
	if err != nil {
		t.Fatal(err)
	}

	// Lookup
	m, err := store.LookupMemory("preference", "coffee")
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected memory, got nil")
	}
	if m.Value != "black, no sugar" {
		t.Errorf("value = %q", m.Value)
	}

	// Upsert with lower confidence — should keep old value
	err = store.UpsertMemory("preference", "coffee", "with sugar", 0.5, "ai_extracted")
	if err != nil {
		t.Fatal(err)
	}
	m, _ = store.LookupMemory("preference", "coffee")
	if m.Value != "black, no sugar" {
		t.Errorf("value = %q, want original (higher confidence)", m.Value)
	}

	// Upsert with higher confidence — should update
	err = store.UpsertMemory("preference", "coffee", "espresso", 1.0, "manual")
	if err != nil {
		t.Fatal(err)
	}
	m, _ = store.LookupMemory("preference", "coffee")
	if m.Value != "espresso" {
		t.Errorf("value = %q, want updated (higher confidence)", m.Value)
	}

	// Lookup non-existent
	missing, err := store.LookupMemory("preference", "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing memory, got %+v", missing)
	}

	// List by type
	err = store.UpsertMemory("preference", "tea", "green", 0.8, "ai_extracted")
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.ListMemoriesByType("preference")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 2 {
		t.Fatalf("list len = %d, want >=2", len(list))
	}
}
