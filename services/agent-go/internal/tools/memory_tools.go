package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
)

// memoryStore is an in-memory key-value store shared across memory tools.
// Each tenant has its own isolated namespace.
type memoryStore struct {
	mu   sync.RWMutex
	data map[string]map[string]string // tenantID -> key -> value
}

var globalMemoryStore = &memoryStore{data: make(map[string]map[string]string)}

// ---------------------------------------------------------------------------
// SaveMemoryTool — lưu key+value vào in-memory store
// ---------------------------------------------------------------------------

// saveMemoryTool saves a key-value pair to the in-memory store.
// Kind=KindWrite.
type saveMemoryTool struct {
	store *memoryStore
}

// NewSaveMemoryTool creates a save-memory tool.
func NewSaveMemoryTool() Tool {
	return &saveMemoryTool{store: globalMemoryStore}
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

	t.store.mu.Lock()
	if t.store.data[tenantID] == nil {
		t.store.data[tenantID] = make(map[string]string)
	}
	t.store.data[tenantID][args.Key] = args.Value
	t.store.mu.Unlock()

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
	store *memoryStore
}

// NewRecallMemoryTool creates a recall-memory tool.
func NewRecallMemoryTool() Tool {
	return &recallMemoryTool{store: globalMemoryStore}
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
	keyword := strings.ToLower(args.Keyword)

	t.store.mu.RLock()
	defer t.store.mu.RUnlock()

	matches := make([]map[string]string, 0)
	tenantData := t.store.data[tenantID]
	for k, v := range tenantData {
		if strings.Contains(strings.ToLower(k), keyword) || strings.Contains(strings.ToLower(v), keyword) {
			matches = append(matches, map[string]string{"key": k, "value": v})
		}
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
	store *memoryStore
}

// NewListMemoriesTool creates a list-memories tool.
func NewListMemoriesTool() Tool {
	return &listMemoriesTool{store: globalMemoryStore}
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

	t.store.mu.RLock()
	defer t.store.mu.RUnlock()

	tenantData := t.store.data[tenantID]
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
