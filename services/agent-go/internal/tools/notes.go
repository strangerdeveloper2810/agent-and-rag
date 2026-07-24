package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// NotesSearchTool -- full-text search across markdown notes
// ---------------------------------------------------------------------------

type notesSearchTool struct {
	notesDir string
}

// NewNotesSearchTool creates a notes search tool. notesDir defaults to ~/.jarvis/notes.
func NewNotesSearchTool(notesDir string) Tool {
	if notesDir == "" {
		home, _ := os.UserHomeDir()
		notesDir = filepath.Join(home, ".jarvis", "notes")
	}
	return &notesSearchTool{notesDir: notesDir}
}

func (t *notesSearchTool) Name() string { return "notes.search" }

func (t *notesSearchTool) Description() string {
	return "Full-text search across markdown notes. Returns matching file names with content excerpts."
}

func (t *notesSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"query":{"type":"string","description":"search query string"}
		},
		"required":["query"],
		"additionalProperties":false
	}`)
}

func (t *notesSearchTool) Kind() Kind { return KindRead }

func (t *notesSearchTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("notes.search: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return Result{}, fmt.Errorf("notes.search: query is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var results []map[string]string
	query := strings.ToLower(args.Query)

	_ = filepath.Walk(t.notesDir, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		if strings.Contains(strings.ToLower(content), query) {
			excerpt := content
			if len(excerpt) > 500 {
				excerpt = excerpt[:500] + "..."
			}
			rel, _ := filepath.Rel(t.notesDir, path)
			results = append(results, map[string]string{
				"file":    rel,
				"excerpt": excerpt,
			})
		}
		return nil
	})

	if len(results) > 10 {
		results = results[:10]
	}

	out, _ := json.Marshal(map[string]any{
		"query":   args.Query,
		"count":   len(results),
		"results": results,
	})
	return Result{Content: string(out)}, nil
}

// ---------------------------------------------------------------------------
// NotesCreateTool -- create a markdown note
// ---------------------------------------------------------------------------

type notesCreateTool struct {
	notesDir string
}

// NewNotesCreateTool creates a notes create tool. notesDir defaults to ~/.jarvis/notes.
func NewNotesCreateTool(notesDir string) Tool {
	if notesDir == "" {
		home, _ := os.UserHomeDir()
		notesDir = filepath.Join(home, ".jarvis", "notes")
	}
	return &notesCreateTool{notesDir: notesDir}
}

func (t *notesCreateTool) Name() string { return "notes.create" }

func (t *notesCreateTool) Description() string {
	return "Create a new markdown note with optional tags."
}

func (t *notesCreateTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"title":{"type":"string","description":"note title (used as filename)"},
			"content":{"type":"string","description":"note content in markdown"},
			"tags":{"type":"array","items":{"type":"string"},"description":"optional tags"}
		},
		"required":["title","content"],
		"additionalProperties":false
	}`)
}

func (t *notesCreateTool) Kind() Kind { return KindWrite }

func (t *notesCreateTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags,omitempty"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("notes.create: invalid args: %w", err)
	}
	if args.Title == "" {
		return Result{}, fmt.Errorf("notes.create: title is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := os.MkdirAll(t.notesDir, 0755); err != nil {
		return Result{}, fmt.Errorf("notes.create: %w", err)
	}

	// Sanitize filename: keep only alphanumeric, dash, underscore
	filename := sanitizeFilename(args.Title)
	if len(filename) > 100 {
		filename = filename[:100]
	}
	filename = filename + ".md"
	filePath := filepath.Join(t.notesDir, filename)

	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(args.Title)
	sb.WriteString("\n\n")
	if len(args.Tags) > 0 {
		sb.WriteString("tags: ")
		sb.WriteString(strings.Join(args.Tags, ", "))
		sb.WriteString("\n\n")
	}
	sb.WriteString(args.Content)
	sb.WriteString("\n")

	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		return Result{}, fmt.Errorf("notes.create: %w", err)
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"title":    args.Title,
		"filename": filename,
		"path":     filePath,
		"created":  true,
	})
	return Result{Content: string(out)}, nil
}

func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}
