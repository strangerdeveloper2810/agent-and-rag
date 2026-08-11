package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotesSearchTool(t *testing.T) {
	tool := NewNotesSearchTool("")

	tests := []struct {
		name    string
		args    json.RawMessage
		wantErr bool
	}{
		{
			name:    "valid query",
			args:    json.RawMessage(`{"query":"test"}`),
			wantErr: false,
		},
		{
			name:    "empty query",
			args:    json.RawMessage(`{"query":""}`),
			wantErr: true,
		},
		{
			name:    "missing query",
			args:    json.RawMessage(`{}`),
			wantErr: true,
		},
		{
			name:    "invalid json",
			args:    json.RawMessage(`{bad`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotesSearchTool_FindsNotes(t *testing.T) {
	dir := t.TempDir()
	// Create test notes in tenant subdirectory (default tenant = "default")
	tenantDir := filepath.Join(dir, "default")
	os.MkdirAll(tenantDir, 0755)
	os.WriteFile(filepath.Join(tenantDir, "note1.md"), []byte("# Alpha\nThis is a test note about golang."), 0644)
	os.WriteFile(filepath.Join(tenantDir, "note2.md"), []byte("# Beta\nNothing relevant here."), 0644)
	os.WriteFile(filepath.Join(tenantDir, "note3.md"), []byte("# Gamma\nAnother test entry with TEST keyword."), 0644)

	tool := NewNotesSearchTool(dir)
	ctx := context.Background()

	result, err := tool.Execute(ctx, json.RawMessage(`{"query":"test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		Count   int `json:"count"`
		Results []struct {
			File string `json:"file"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(result.Content), &out); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if out.Count < 2 {
		t.Errorf("expected at least 2 matches, got %d", out.Count)
	}
}

func TestNotesCreateTool(t *testing.T) {
	dir := t.TempDir()
	tool := NewNotesCreateTool(dir)

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "valid create",
			args:    `{"title":"My Note","content":"# Hello\nWorld","tags":["go","agent"]}`,
			wantErr: false,
		},
		{
			name:    "create without tags",
			args:    `{"title":"Simple","content":"Just text"}`,
			wantErr: false,
		},
		{
			name:    "missing title",
			args:    `{"content":"no title"}`,
			wantErr: true,
		},
		{
			name:    "empty title",
			args:    `{"title":"","content":"empty"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v, result = %s", err, tt.wantErr, result.Content)
			}
			if err == nil {
				var out struct {
					Created bool `json:"created"`
				}
				json.Unmarshal([]byte(result.Content), &out)
				if !out.Created {
					t.Error("expected created=true")
				}
			}
		})
	}
}

func TestNotesCreateTool_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	tool := NewNotesCreateTool(dir)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"title":"Test Note","content":"Hello World"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Notes are created in tenant subdirectory (default = "default")
	tenantDir := filepath.Join(dir, "default")
	entries, _ := os.ReadDir(tenantDir)
	if len(entries) == 0 {
		t.Fatal("expected at least one file created")
	}
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			found = true
			break
		}
	}
	if !found {
		t.Error("no .md file created")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello-World"},
		{"test/file", "test-file"},
		{"valid_name-123", "valid_name-123"},
		{"spaces   and   tabs", "spaces---and---tabs"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
