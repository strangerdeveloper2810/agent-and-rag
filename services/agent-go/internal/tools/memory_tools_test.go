package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func clearMemoryStore() {
	globalMemoryStore.mu.Lock()
	globalMemoryStore.data = make(map[string]map[string]string)
	globalMemoryStore.mu.Unlock()
}

func TestSaveMemoryTool(t *testing.T) {
	clearMemoryStore()

	tool := NewSaveMemoryTool()

	t.Run("save valid key-value", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{
			"key":   "greeting",
			"value": "hello world",
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Key    string `json:"key"`
			Stored bool   `json:"stored"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Key != "greeting" {
			t.Errorf("expected key 'greeting', got %q", out.Key)
		}
		if !out.Stored {
			t.Error("expected stored=true")
		}
	})

	t.Run("save overwrites existing key", func(t *testing.T) {
		clearMemoryStore()

		args1, _ := json.Marshal(map[string]string{"key": "x", "value": "first"})
		tool.Execute(context.Background(), args1)

		args2, _ := json.Marshal(map[string]string{"key": "x", "value": "second"})
		res, err := tool.Execute(context.Background(), args2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Stored bool `json:"stored"`
		}
		json.Unmarshal([]byte(res.Content), &out)
		if !out.Stored {
			t.Error("expected stored=true for overwrite")
		}

		// Verify value was overwritten (uses "default" tenant since no X-Tenant-ID in test context)
		globalMemoryStore.mu.RLock()
		tenantData := globalMemoryStore.data["default"]
		val := tenantData["x"]
		globalMemoryStore.mu.RUnlock()
		if val != "second" {
			t.Errorf("expected 'second', got %q", val)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"value": "no key"})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing key, got nil")
		}
		if !strings.Contains(err.Error(), "key is required") {
			t.Errorf("expected 'key is required' error, got: %v", err)
		}
	})

	t.Run("missing value", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"key": "k"})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing value, got nil")
		}
		if !strings.Contains(err.Error(), "value is required") {
			t.Errorf("expected 'value is required' error, got: %v", err)
		}
	})
}

func TestRecallMemoryTool(t *testing.T) {
	clearMemoryStore()

	// Seed data
	save := NewSaveMemoryTool()
	seeds := []map[string]string{
		{"key": "greeting", "value": "hello world"},
		{"key": "farewell", "value": "goodbye everyone"},
		{"key": "task-1", "value": "Complete the project report"},
		{"key": "note", "value": "Hello again, remember the deadline"},
	}
	for _, s := range seeds {
		args, _ := json.Marshal(s)
		save.Execute(context.Background(), args)
	}

	tool := NewRecallMemoryTool()

	t.Run("recall by keyword in value", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"keyword": "hello"})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count   int `json:"count"`
			Matches []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"matches"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count < 2 {
			t.Errorf("expected at least 2 matches for 'hello', got %d", out.Count)
		}
	})

	t.Run("recall by keyword in key", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"keyword": "task"})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count int `json:"count"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count < 1 {
			t.Errorf("expected at least 1 match for 'task', got %d", out.Count)
		}
	})

	t.Run("recall case-insensitive", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"keyword": "HELLO"})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count int `json:"count"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count < 2 {
			t.Errorf("expected case-insensitive matches for 'HELLO', got %d", out.Count)
		}
	})

	t.Run("recall no matches", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"keyword": "zzznonexistent"})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count int `json:"count"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count != 0 {
			t.Errorf("expected 0 matches, got %d", out.Count)
		}
	})

	t.Run("missing keyword", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing keyword, got nil")
		}
		if !strings.Contains(err.Error(), "keyword is required") {
			t.Errorf("expected 'keyword is required' error, got: %v", err)
		}
	})
}

func TestListMemoriesTool(t *testing.T) {
	clearMemoryStore()

	// Seed data
	save := NewSaveMemoryTool()
	seeds := []map[string]string{
		{"key": "a", "value": "alpha"},
		{"key": "b", "value": "beta"},
		{"key": "c", "value": "gamma"},
	}
	for _, s := range seeds {
		args, _ := json.Marshal(s)
		save.Execute(context.Background(), args)
	}

	tool := NewListMemoriesTool()

	t.Run("list all memories", func(t *testing.T) {
		res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count    int `json:"count"`
			Memories []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"memories"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count != 3 {
			t.Errorf("expected 3 memories, got %d", out.Count)
		}
	})

	t.Run("list empty store", func(t *testing.T) {
		clearMemoryStore()

		res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count int `json:"count"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count != 0 {
			t.Errorf("expected 0 memories, got %d", out.Count)
		}
	})
}

func TestMemoryToolsInterface(t *testing.T) {
	t.Run("SaveMemoryTool interface", func(t *testing.T) {
		var tool Tool = NewSaveMemoryTool()
		if tool.Name() != "memory.save" {
			t.Errorf("Name: got %q, want %q", tool.Name(), "memory.save")
		}
		if tool.Kind() != KindWrite {
			t.Errorf("Kind: got %v, want KindWrite", tool.Kind())
		}
		if tool.Description() == "" {
			t.Error("Description is empty")
		}
		if len(tool.Schema()) == 0 {
			t.Error("Schema is empty")
		}
	})

	t.Run("RecallMemoryTool interface", func(t *testing.T) {
		var tool Tool = NewRecallMemoryTool()
		if tool.Name() != "memory.recall" {
			t.Errorf("Name: got %q, want %q", tool.Name(), "memory.recall")
		}
		if tool.Kind() != KindRead {
			t.Errorf("Kind: got %v, want KindRead", tool.Kind())
		}
		if tool.Description() == "" {
			t.Error("Description is empty")
		}
		if len(tool.Schema()) == 0 {
			t.Error("Schema is empty")
		}
	})

	t.Run("ListMemoriesTool interface", func(t *testing.T) {
		var tool Tool = NewListMemoriesTool()
		if tool.Name() != "memory.list" {
			t.Errorf("Name: got %q, want %q", tool.Name(), "memory.list")
		}
		if tool.Kind() != KindRead {
			t.Errorf("Kind: got %v, want KindRead", tool.Kind())
		}
		if tool.Description() == "" {
			t.Error("Description is empty")
		}
		if len(tool.Schema()) == 0 {
			t.Error("Schema is empty")
		}
	})
}

func TestSaveRecallFlow(t *testing.T) {
	clearMemoryStore()

	save := NewSaveMemoryTool()
	recall := NewRecallMemoryTool()

	// Save a memory
	saveArgs, _ := json.Marshal(map[string]string{
		"key":   "project-deadline",
		"value": "Q3 report due August 15th",
	})
	_, err := save.Execute(context.Background(), saveArgs)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Recall it
	recallArgs, _ := json.Marshal(map[string]string{"keyword": "deadline"})
	res, err := recall.Execute(context.Background(), recallArgs)
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}

	var out struct {
		Count   int `json:"count"`
		Matches []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"matches"`
	}
	json.Unmarshal([]byte(res.Content), &out)

	if out.Count != 1 {
		t.Fatalf("expected 1 match, got %d", out.Count)
	}
	if out.Matches[0].Key != "project-deadline" {
		t.Errorf("expected key 'project-deadline', got %q", out.Matches[0].Key)
	}
	if out.Matches[0].Value != "Q3 report due August 15th" {
		t.Errorf("unexpected value: %q", out.Matches[0].Value)
	}
}
