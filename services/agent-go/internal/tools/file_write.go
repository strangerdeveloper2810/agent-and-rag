package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
)

// ---------------------------------------------------------------------------
// FileWriteTool -- write content to a file within allowed paths
// ---------------------------------------------------------------------------

type fileWriteTool struct {
	allowedPaths []string
}

// NewFileWriteTool creates a file write tool restricted to allowedPaths.
// An empty allowedPaths means all paths are allowed (use with caution).
func NewFileWriteTool(allowedPaths []string) Tool {
	return &fileWriteTool{allowedPaths: allowedPaths}
}

func (t *fileWriteTool) Name() string { return "file.write" }

func (t *fileWriteTool) Description() string {
	return "Write content to a file within allowed paths. Creates parent directories automatically. Max content size 100KB."
}

func (t *fileWriteTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"path":{"type":"string","description":"file path (must be within allowed paths)"},
			"content":{"type":"string","description":"file content to write"}
		},
		"required":["path","content"],
		"additionalProperties":false
	}`)
}

func (t *fileWriteTool) Kind() Kind { return KindWrite }

func (t *fileWriteTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("file.write: invalid args: %w", err)
	}
	if args.Path == "" {
		return Result{}, fmt.Errorf("file.write: path is required")
	}

	if len(args.Content) > 100_000 {
		return Result{}, fmt.Errorf("file.write: content too large (max 100KB, got %d bytes)", len(args.Content))
	}

	tenantID := middleware.GetTenantID(ctx)
	// Nest files under a tenant-specific subdirectory
	args.Path = filepath.Join(filepath.Dir(args.Path), tenantID, filepath.Base(args.Path))

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Validate path is within allowed paths
	if !t.isAllowed(args.Path) {
		return Result{}, fmt.Errorf("file.write: access denied: %q is outside allowed paths", args.Path)
	}

	// Create parent directories
	dir := filepath.Dir(args.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return Result{}, fmt.Errorf("file.write: create parent dirs: %w", err)
	}

	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
		return Result{}, fmt.Errorf("file.write: %w", err)
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"path":    args.Path,
		"written": true,
		"size":    len(args.Content),
	})
	return Result{Content: string(out)}, nil
}

func (t *fileWriteTool) isAllowed(path string) bool {
	if len(t.allowedPaths) == 0 {
		return true // no restrictions configured
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, allowed := range t.allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(abs, absAllowed+string(filepath.Separator)) || abs == absAllowed {
			return true
		}
	}
	return false
}
