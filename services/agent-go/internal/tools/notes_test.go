package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
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
			// ĐỔI HÀNH VI CÓ CHỦ ĐÍCH: query rỗng nghĩa là "liệt kê tất cả",
			// không còn là lỗi. Log dev thật cho thấy model gọi notes.search với
			// args rỗng khi muốn liệt kê toàn bộ note và nhận lỗi
			// "query is required" — trong khi khi nó đoán query="*" thì lại khớp
			// NHẦM theo nghĩa literal (file markdown đầy dấu *) nên ra kết quả
			// trông đúng mà thực chất tình cờ.
			name:    "empty query = liệt kê tất cả",
			args:    json.RawMessage(`{"query":""}`),
			wantErr: false,
		},
		{
			name:    "missing query = liệt kê tất cả",
			args:    json.RawMessage(`{}`),
			wantErr: false,
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

// TestNotesSearchTool_ListAll: query rỗng / "*" phải trả về TẤT CẢ note, kể cả
// note không chứa từ khoá nào. Trước fix, args rỗng trả lỗi và "*" chỉ khớp
// tình cờ nhờ dấu * trong markdown — nên note không có dấu * sẽ bị bỏ sót.
func TestNotesSearchTool_ListAll(t *testing.T) {
	dir := t.TempDir()
	tenantDir := filepath.Join(dir, "default")
	if err := os.MkdirAll(tenantDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Cố tình KHÔNG có dấu "*" và không có từ khoá chung nào giữa 3 file.
	notes := map[string]string{
		"alpha.md": "# Alpha\nnoi dung ve golang",
		"beta.md":  "# Beta\nkhong lien quan gi",
		"gamma.md": "# Gamma\nchu de hoan toan khac",
	}
	for name, content := range notes {
		if err := os.WriteFile(filepath.Join(tenantDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := NewNotesSearchTool(dir)

	for _, args := range []string{`{}`, `{"query":""}`, `{"query":"*"}`, `{"query":"  "}`} {
		t.Run(args, func(t *testing.T) {
			res, err := tool.Execute(context.Background(), json.RawMessage(args))
			if err != nil {
				t.Fatalf("Execute(%s) lỗi: %v", args, err)
			}

			var out struct {
				Count   int  `json:"count"`
				ListAll bool `json:"listAll"`
				Results []struct {
					File string `json:"file"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
				t.Fatalf("parse kết quả: %v", err)
			}

			if !out.ListAll {
				t.Errorf("listAll = false, want true cho args %s", args)
			}
			if out.Count != len(notes) {
				t.Errorf("count = %d, want %d (phải liệt kê TẤT CẢ note)", out.Count, len(notes))
			}
			seen := map[string]bool{}
			for _, r := range out.Results {
				seen[r.File] = true
			}
			for name := range notes {
				if !seen[name] {
					t.Errorf("thiếu note %q trong kết quả liệt kê tất cả", name)
				}
			}
		})
	}
}

// TestNotesSearchTool_TenantIsolation khoá một lỗ RÒ RỈ DỮ LIỆU GIỮA CÁC USER:
// notes.create ghi vào <notesDir>/<tenantID>/ nhưng notes.search lại walk TOÀN
// BỘ <notesDir>, nên note của tenant khác cũng bị trả về. Kể cả khi liệt kê tất
// cả (query rỗng) cũng chỉ được thấy note của chính tenant mình.
func TestNotesSearchTool_TenantIsolation(t *testing.T) {
	dir := t.TempDir()

	for tenant, note := range map[string]string{
		"tenant-a": "bi-mat-cua-a.md",
		"tenant-b": "bi-mat-cua-b.md",
	} {
		tenantDir := filepath.Join(dir, tenant)
		if err := os.MkdirAll(tenantDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tenantDir, note), []byte("# secret\nthong tin rieng cua "+tenant), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := NewNotesSearchTool(dir)
	ctxA := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-a")

	for _, args := range []string{`{}`, `{"query":"secret"}`, `{"query":"thong tin"}`} {
		t.Run(args, func(t *testing.T) {
			res, err := tool.Execute(ctxA, json.RawMessage(args))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if strings.Contains(res.Content, "tenant-b") || strings.Contains(res.Content, "bi-mat-cua-b") {
				t.Errorf("RÒ RỈ note của tenant khác với args %s:\n%s", args, res.Content)
			}
			if !strings.Contains(res.Content, "bi-mat-cua-a") {
				t.Errorf("thiếu note của chính tenant mình với args %s:\n%s", args, res.Content)
			}
		})
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
