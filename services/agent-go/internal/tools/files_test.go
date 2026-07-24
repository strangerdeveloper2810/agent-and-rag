package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSearchTool(t *testing.T) {
	// Tạo cấu trúc thư mục test
	tmpDir := t.TempDir()

	// Tạo files
	files := []string{
		"main.go",
		"handler.go",
		"README.md",
		"notes.txt",
		"sub/helper.go",
		"sub/data.json",
	}
	for _, f := range files {
		p := filepath.Join(tmpDir, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("test"), 0o644)
	}

	tool := NewFileSearchTool([]string{tmpDir})

	t.Run("search go files", func(t *testing.T) {
		args, _ := json.Marshal(FileSearchArgs{Pattern: "*.go", Path: tmpDir})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count   int      `json:"count"`
			Matches []string `json:"matches"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count < 2 {
			t.Errorf("expected at least 2 go files, got %d. matches=%v", out.Count, out.Matches)
		}
	})

	t.Run("search txt files", func(t *testing.T) {
		args, _ := json.Marshal(FileSearchArgs{Pattern: "*.txt", Path: tmpDir})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count   int      `json:"count"`
			Matches []string `json:"matches"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count != 1 {
			t.Errorf("expected 1 txt file, got %d. matches=%v", out.Count, out.Matches)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		args, _ := json.Marshal(FileSearchArgs{Pattern: "*.py", Path: tmpDir})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count   int      `json:"count"`
			Matches []string `json:"matches"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Count != 0 {
			t.Errorf("expected 0 matches, got %d", out.Count)
		}
	})

	t.Run("path outside allowed", func(t *testing.T) {
		args, _ := json.Marshal(FileSearchArgs{Pattern: "*.go", Path: "/etc"})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for path outside allowed, got nil")
		}
		if !strings.Contains(err.Error(), "not in allowed paths") {
			t.Errorf("expected 'not in allowed paths' error, got: %v", err)
		}
	})

	t.Run("missing pattern", func(t *testing.T) {
		args, _ := json.Marshal(FileSearchArgs{Pattern: "", Path: tmpDir})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing pattern, got nil")
		}
		if !strings.Contains(err.Error(), "pattern is required") {
			t.Errorf("expected 'pattern is required' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad json`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "invalid args") {
			t.Errorf("expected 'invalid args' error, got: %v", err)
		}
	})
}

func TestFileReadTool(t *testing.T) {
	tmpDir := t.TempDir()

	// Tạo file test
	content := "Hello, world!\nThis is a test file.\nLine 3."
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte(content), 0o644)

	t.Run("read existing file", func(t *testing.T) {
		tool := NewFileReadTool([]string{tmpDir})
		args, _ := json.Marshal(map[string]string{"path": testFile})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Content string `json:"content"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Content != content {
			t.Errorf("expected content %q, got %q", content, out.Content)
		}
	})

	t.Run("file outside allowed", func(t *testing.T) {
		tool := NewFileReadTool([]string{tmpDir})
		args, _ := json.Marshal(map[string]string{"path": "/etc/passwd"})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for outside path, got nil")
		}
		if !strings.Contains(err.Error(), "access denied") {
			t.Errorf("expected 'access denied' error, got: %v", err)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		tool := NewFileReadTool([]string{tmpDir})
		args, _ := json.Marshal(map[string]string{"path": filepath.Join(tmpDir, "nope.txt")})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})

	t.Run("truncation with custom limit", func(t *testing.T) {
		bigContent := strings.Repeat("A", 200)
		bigFile := filepath.Join(tmpDir, "big.txt")
		os.WriteFile(bigFile, []byte(bigContent), 0o644)

		tool := NewFileReadToolWithLimit([]string{tmpDir}, 50)
		args, _ := json.Marshal(map[string]string{"path": bigFile})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Content string `json:"content"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if !strings.Contains(out.Content, "[truncated]") {
			t.Errorf("expected truncated content, got: %s", out.Content[:min(100, len(out.Content))])
		}
		if len(out.Content) > 50+len("\n... [truncated]") {
			t.Errorf("content longer than expected: len=%d", len(out.Content))
		}
	})

	t.Run("missing path arg", func(t *testing.T) {
		tool := NewFileReadTool([]string{tmpDir})
		args, _ := json.Marshal(map[string]string{})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing path, got nil")
		}
		if !strings.Contains(err.Error(), "path is required") {
			t.Errorf("expected 'path is required' error, got: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		tool := NewFileReadTool([]string{tmpDir})
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel ngay
		args, _ := json.Marshal(map[string]string{"path": testFile})
		_, err := tool.Execute(ctx, args)
		// Có thể lỗi do context cancelled hoặc đọc thành công trước khi cancel
		// (vì file nhỏ). Ta chỉ cần đảm bảo không panic.
		_ = err
	})
}

func TestFileToolInterface(t *testing.T) {
	// Kiểm tra các tool implement Tool interface đúng
	tmpDir := t.TempDir()

	t.Run("FileSearchTool interface", func(t *testing.T) {
		var tool Tool = NewFileSearchTool([]string{tmpDir})
		if tool.Name() != "file.search" {
			t.Errorf("Name: got %q, want %q", tool.Name(), "file.search")
		}
		if tool.Kind() != KindRead {
			t.Errorf("Kind: got %v, want KindRead", tool.Kind())
		}
		if tool.Description() == "" {
			t.Error("Description is empty")
		}
		schema := tool.Schema()
		if len(schema) == 0 {
			t.Error("Schema is empty")
		}
	})

	t.Run("FileReadTool interface", func(t *testing.T) {
		var tool Tool = NewFileReadTool([]string{tmpDir})
		if tool.Name() != "file.read" {
			t.Errorf("Name: got %q, want %q", tool.Name(), "file.read")
		}
		if tool.Kind() != KindRead {
			t.Errorf("Kind: got %v, want KindRead", tool.Kind())
		}
		if tool.Description() == "" {
			t.Error("Description is empty")
		}
		schema := tool.Schema()
		if len(schema) == 0 {
			t.Error("Schema is empty")
		}
	})
}
