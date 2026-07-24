package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileWriteTool(t *testing.T) {
	dir := t.TempDir()
	tool := NewFileWriteTool([]string{dir})

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "write to allowed path",
			args:    `{"path":"` + filepath.Join(dir, "test.txt") + `","content":"hello world"}`,
			wantErr: false,
		},
		{
			name:    "write to nested path",
			args:    `{"path":"` + filepath.Join(dir, "sub", "dir", "file.txt") + `","content":"nested"}`,
			wantErr: false,
		},
		{
			name:    "write empty file",
			args:    `{"path":"` + filepath.Join(dir, "empty.txt") + `","content":""}`,
			wantErr: false,
		},
		{
			name:    "missing path",
			args:    `{"content":"data"}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			args:    `{bad`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.wantErr == false {
				var out struct {
					Written bool `json:"written"`
				}
				json.Unmarshal([]byte(result.Content), &out)
				if !out.Written {
					t.Error("expected written=true")
				}
			}
		})
	}
}

func TestFileWriteTool_OutsideAllowedPath(t *testing.T) {
	dir := t.TempDir()
	otherDir := t.TempDir()
	tool := NewFileWriteTool([]string{dir})

	_, err := tool.Execute(context.Background(), json.RawMessage(
		`{"path":"`+filepath.Join(otherDir, "test.txt")+`","content":"data"}`,
	))
	if err == nil {
		t.Fatal("expected error for write outside allowed path")
	}
	if !strings.Contains(err.Error(), "outside allowed paths") {
		t.Errorf("expected 'outside allowed paths' error, got %v", err)
	}
}

func TestFileWriteTool_ContentTooLarge(t *testing.T) {
	dir := t.TempDir()
	tool := NewFileWriteTool([]string{dir})

	// Create content > 100KB
	largeContent := strings.Repeat("x", 100_001)

	_, err := tool.Execute(context.Background(), json.RawMessage(
		`{"path":"`+filepath.Join(dir, "large.txt")+`","content":"`+largeContent+`"}`,
	))
	if err == nil {
		t.Fatal("expected error for content too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' error, got %v", err)
	}
}

func TestFileWriteTool_WritesContent(t *testing.T) {
	dir := t.TempDir()
	tool := NewFileWriteTool([]string{dir})
	filePath := filepath.Join(dir, "hello.txt")

	_, err := tool.Execute(context.Background(), json.RawMessage(
		`{"path":"`+filePath+`","content":"Hello, JARVIS!"}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "Hello, JARVIS!" {
		t.Errorf("expected 'Hello, JARVIS!', got %q", string(data))
	}
}

func TestFileWriteTool_NestedDirectories(t *testing.T) {
	dir := t.TempDir()
	tool := NewFileWriteTool([]string{dir})
	filePath := filepath.Join(dir, "a", "b", "c", "deep.txt")

	_, err := tool.Execute(context.Background(), json.RawMessage(
		`{"path":"`+filePath+`","content":"deep"}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("deep file not created: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("expected 'deep', got %q", string(data))
	}
}

func TestFileWriteTool_NoRestrictions(t *testing.T) {
	dir := t.TempDir()
	// Empty allowedPaths means all paths allowed
	tool := NewFileWriteTool(nil)
	filePath := filepath.Join(dir, "unrestricted.txt")

	_, err := tool.Execute(context.Background(), json.RawMessage(
		`{"path":"`+filePath+`","content":"free"}`,
	))
	if err != nil {
		t.Fatalf("unexpected error with no restrictions: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	if string(data) != "free" {
		t.Errorf("expected 'free', got %q", string(data))
	}
}
