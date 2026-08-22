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

// FileSearchTool tìm file theo pattern (glob) trong thư mục được phép.
// Kind=KindRead vì không gây side-effect.
type fileSearchTool struct {
	allowedPaths []string
}

// FileSearchArgs là input schema cho file.search.
type FileSearchArgs struct {
	Pattern string `json:"pattern"`        // glob pattern, e.g. "*.go", "**/*.txt"
	Path    string `json:"path,omitempty"` // base dir (mặc định: allowedPaths[0])
}

// NewFileSearchTool tạo file search tool với danh sách thư mục được phép.
func NewFileSearchTool(allowedPaths []string) Tool {
	return &fileSearchTool{allowedPaths: allowedPaths}
}

func (t *fileSearchTool) Name() string { return "file.search" }

func (t *fileSearchTool) Description() string {
	return "Tìm file theo pattern (glob) trong thư mục được phép. Trả danh sách JSON đường dẫn khớp."
}

func (t *fileSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"pattern":{"type":"string","description":"glob pattern, e.g. *.go, **/*.txt"},
			"path":{"type":"string","description":"base directory (optional, defaults to first allowed path)"}
		},
		"required":["pattern"],
		"additionalProperties":false
	}`)
}

func (t *fileSearchTool) Kind() Kind { return KindRead }

func (t *fileSearchTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args FileSearchArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("file.search: invalid args: %w", err)
	}
	if args.Pattern == "" {
		return Result{}, fmt.Errorf("file.search: pattern is required")
	}

	// Timeout 10s
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tenantID := middleware.GetTenantID(ctx)

	basePath := args.Path
	if basePath == "" {
		if len(t.allowedPaths) == 0 {
			return Result{}, fmt.Errorf("file.search: no allowed paths configured")
		}
		basePath = t.allowedPaths[0]
	}
	// Nest into a tenant-specific subdirectory so a tenant can never list
	// files that belong to another tenant's subdirectory, even if they
	// explicitly pass a "path" trying to point elsewhere.
	basePath = filepath.Join(basePath, tenantID)

	// Validate basePath nằm trong allowedPaths
	if !t.isAllowed(basePath) {
		return Result{}, fmt.Errorf("file.search: path %q is not in allowed paths", basePath)
	}

	// Dùng filepath.Walk để hỗ trợ ** pattern
	var matches []string
	pattern := args.Pattern

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			return nil // bỏ qua file lỗi
		}
		if info.IsDir() {
			return nil
		}
		matched, matchErr := filepath.Match(pattern, info.Name())
		if matchErr != nil {
			return nil
		}
		if matched {
			matches = append(matches, path)
		}
		// Cũng kiểm tra match với full relative path từ basePath
		rel, _ := filepath.Rel(basePath, path)
		if rel != info.Name() {
			if matched2, _ := filepath.Match(pattern, rel); matched2 {
				// tránh trùng lặp
				found := false
				for _, m := range matches {
					if m == path {
						found = true
						break
					}
				}
				if !found {
					matches = append(matches, path)
				}
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("file.search: walk error: %w", err)
	}

	out, _ := json.Marshal(map[string]any{
		"pattern": args.Pattern,
		"count":   len(matches),
		"matches": matches,
	})
	return Result{Content: string(out)}, nil
}

// isAllowed kiểm tra path (tuyệt đối hoá) nằm trong ít nhất 1 allowedPath.
func (t *fileSearchTool) isAllowed(path string) bool {
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

// ---------------------------------------------------------------------------
// FileReadTool — đọc nội dung file
// ---------------------------------------------------------------------------

// fileReadTool đọc nội dung file trong thư mục được phép.
// Kind=KindRead, giới hạn kích thước (mặc định 24000 ký tự).
type fileReadTool struct {
	allowedPaths []string
	maxSize      int64
}

const defaultMaxSize = 24_000

// NewFileReadTool tạo file read tool với danh sách thư mục được phép.
func NewFileReadTool(allowedPaths []string) Tool {
	return &fileReadTool{allowedPaths: allowedPaths, maxSize: defaultMaxSize}
}

// NewFileReadToolWithLimit tạo file read tool với giới hạn kích thước tuỳ chỉnh (số ký tự).
func NewFileReadToolWithLimit(allowedPaths []string, maxSize int64) Tool {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	return &fileReadTool{allowedPaths: allowedPaths, maxSize: maxSize}
}

func (t *fileReadTool) Name() string { return "file.read" }

func (t *fileReadTool) Description() string {
	return "Đọc nội dung file text trong thư mục được phép. Trả về text (cắt bớt nếu quá dài)."
}

func (t *fileReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"path":{"type":"string","description":"absolute or relative path to the file"}
		},
		"required":["path"],
		"additionalProperties":false
	}`)
}

func (t *fileReadTool) Kind() Kind { return KindRead }

func (t *fileReadTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("file.read: invalid args: %w", err)
	}
	if args.Path == "" {
		return Result{}, fmt.Errorf("file.read: path is required")
	}

	tenantID := middleware.GetTenantID(ctx)
	// Nest into the tenant-specific subdirectory so a tenant can only read
	// files under its own subdirectory (matching where file.write puts them).
	args.Path = filepath.Join(filepath.Dir(args.Path), tenantID, filepath.Base(args.Path))

	// Validate path
	if !t.isAllowed(args.Path) {
		return Result{}, fmt.Errorf("file.read: access denied: %q is outside allowed paths", args.Path)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return Result{}, fmt.Errorf("file.read: %w", err)
	}

	content := string(data)
	if int64(len(content)) > t.maxSize {
		content = content[:t.maxSize] + "\n... [truncated]"
	}

	out, _ := json.Marshal(map[string]any{
		"path":    args.Path,
		"content": content,
		"size":    len(data),
	})
	return Result{Content: string(out)}, nil
}

// isAllowed kiểm tra path nằm trong allowedPaths.
func (t *fileReadTool) isAllowed(path string) bool {
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
