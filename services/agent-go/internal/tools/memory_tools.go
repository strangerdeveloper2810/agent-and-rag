package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
)

// memoryBackend là interface tối thiểu mà memory.save/recall/list cần.
//
// Trước đây 3 tool này dùng 1 kho riêng (globalMemoryStore, package-level var
// trong file này), HOÀN TOÀN tách biệt với internal/memory.Store — kho mà
// RecallNode/ExtractNode/Learner dùng để tự động bơm "[BỘ NHỚ]" vào system
// prompt mỗi lượt. Hệ quả: model chủ động gọi memory.save để "nhớ giúp" 1
// điều gì đó, nhưng lượt sau RecallNode không hề thấy nó — dữ liệu chỉ đọc
// lại được nếu model TÌNH CỜ tự gọi memory.recall, không đáng tin cậy.
//
// Khai báo interface CỤC BỘ (thay vì import internal/memory.Store trực tiếp)
// để tránh import cycle: internal/memory → internal/agent → internal/tools.
// *memory.Store đã thoả interface này qua structural typing (đúng 3 method
// bên dưới) — không cần đổi gì ở package memory ngoài thêm All(). Wiring
// thực tế (cùng 1 *memory.Store cho cả recall pipeline lẫn 3 tool này) nằm ở
// cmd/server/main.go, nơi cả 2 package đã được import sẵn nên không có cycle.
type memoryBackend interface {
	Set(tenantID, key, value string)
	Search(tenantID, query string) map[string]string
	All(tenantID string) map[string]string
}

// ---------------------------------------------------------------------------
// SaveMemoryTool — lưu key+value vào memoryBackend dùng chung
// ---------------------------------------------------------------------------

// saveMemoryTool saves a key-value pair into the shared memory backend.
// Kind=KindWrite.
type saveMemoryTool struct {
	store memoryBackend
}

// NewSaveMemoryTool creates a save-memory tool. store PHẢI là cùng 1 instance
// truyền cho memory.RecallNode/ExtractNode/Learner — xem doc memoryBackend.
func NewSaveMemoryTool(store memoryBackend) Tool {
	return &saveMemoryTool{store: store}
}

func (t *saveMemoryTool) Name() string { return "memory.save" }

func (t *saveMemoryTool) Description() string {
	return "Lưu một key-value pair vào bộ nhớ tạm. Key phải là string duy nhất."
}

func (t *saveMemoryTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"key":{"type":"string","description":"unique key for the memory"},
			"value":{"type":"string","description":"value to store"}
		},
		"required":["key","value"],
		"additionalProperties":false
	}`)
}

func (t *saveMemoryTool) Kind() Kind { return KindWrite }

func (t *saveMemoryTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("memory.save: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Key) == "" {
		return Result{}, fmt.Errorf("memory.save: key is required")
	}
	if strings.TrimSpace(args.Value) == "" {
		return Result{}, fmt.Errorf("memory.save: value is required")
	}

	tenantID := middleware.GetTenantID(ctx)
	t.store.Set(tenantID, args.Key, args.Value)

	out, _ := json.Marshal(map[string]any{
		"key":    args.Key,
		"stored": true,
	})
	return Result{Content: string(out)}, nil
}

// ---------------------------------------------------------------------------
// RecallMemoryTool — tìm kiếm memory theo keyword
// ---------------------------------------------------------------------------

// recallMemoryTool searches memories by keyword (case-insensitive substring match).
// Kind=KindRead.
type recallMemoryTool struct {
	store memoryBackend
}

// NewRecallMemoryTool creates a recall-memory tool. store PHẢI là cùng 1
// instance truyền cho memory.RecallNode/ExtractNode/Learner — xem doc
// memoryBackend.
func NewRecallMemoryTool(store memoryBackend) Tool {
	return &recallMemoryTool{store: store}
}

func (t *recallMemoryTool) Name() string { return "memory.recall" }

func (t *recallMemoryTool) Description() string {
	return "Tìm kiếm memories theo keyword (case-insensitive). Trả về JSON các matches."
}

func (t *recallMemoryTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"keyword":{"type":"string","description":"keyword to search for in keys and values"}
		},
		"required":["keyword"],
		"additionalProperties":false
	}`)
}

func (t *recallMemoryTool) Kind() Kind { return KindRead }

func (t *recallMemoryTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("memory.recall: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Keyword) == "" {
		return Result{}, fmt.Errorf("memory.recall: keyword is required")
	}

	tenantID := middleware.GetTenantID(ctx)

	found := t.store.Search(tenantID, args.Keyword)
	matches := make([]map[string]string, 0, len(found))
	for k, v := range found {
		matches = append(matches, map[string]string{"key": k, "value": v})
	}

	out, _ := json.Marshal(map[string]any{
		"keyword": args.Keyword,
		"count":   len(matches),
		"matches": matches,
	})
	return Result{Content: string(out)}, nil
}

// ---------------------------------------------------------------------------
// ListMemoriesTool — liệt kê tất cả memories
// ---------------------------------------------------------------------------

// listMemoriesTool lists all stored memories.
// Kind=KindRead.
type listMemoriesTool struct {
	store memoryBackend
}

// NewListMemoriesTool creates a list-memories tool. store PHẢI là cùng 1
// instance truyền cho memory.RecallNode/ExtractNode/Learner — xem doc
// memoryBackend.
func NewListMemoriesTool(store memoryBackend) Tool {
	return &listMemoriesTool{store: store}
}

func (t *listMemoriesTool) Name() string { return "memory.list" }

func (t *listMemoriesTool) Description() string {
	return "Liệt kê tất cả memories đã lưu dưới dạng JSON array."
}

func (t *listMemoriesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{},
		"additionalProperties":false
	}`)
}

func (t *listMemoriesTool) Kind() Kind { return KindRead }

func (t *listMemoriesTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	// Accept empty args or empty object
	if len(rawArgs) > 0 {
		var check struct{}
		if err := json.Unmarshal(rawArgs, &check); err != nil {
			return Result{}, fmt.Errorf("memory.list: invalid args: %w", err)
		}
	}

	tenantID := middleware.GetTenantID(ctx)

	tenantData := t.store.All(tenantID)
	items := make([]map[string]string, 0, len(tenantData))
	for k, v := range tenantData {
		items = append(items, map[string]string{"key": k, "value": v})
	}

	out, _ := json.Marshal(map[string]any{
		"count":    len(items),
		"memories": items,
	})
	return Result{Content: string(out)}, nil
}
