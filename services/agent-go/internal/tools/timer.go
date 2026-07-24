package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type timerEntry struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expiresAt"`
	Duration  string    `json:"duration"`
}

type timerTool struct {
	mu     sync.RWMutex
	timers map[string]*timerEntry
}

func NewTimerTool() Tool {
	return &timerTool{
		timers: make(map[string]*timerEntry),
	}
}

func (t *timerTool) Name() string { return "timer" }

func (t *timerTool) Description() string {
	return "Set, list, and cancel reminder timers. Timer durations: 30s, 5m, 1h, etc."
}

func (t *timerTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"action":{"type":"string","enum":["set","list","cancel"],"description":"set a timer, list active timers, or cancel a timer"},
			"message":{"type":"string","description":"reminder message (required for set)"},
			"duration":{"type":"string","description":"duration like 30s, 5m, 1h (required for set)"},
			"id":{"type":"string","description":"timer ID (required for cancel)"}
		},
		"required":["action"],
		"additionalProperties":false
	}`)
}

func (t *timerTool) Kind() Kind { return KindRead }

func (t *timerTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Action   string `json:"action"`
		Message  string `json:"message"`
		Duration string `json:"duration"`
		ID       string `json:"id"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("timer: invalid args: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	switch args.Action {
	case "set":
		return t.setTimer(ctx, args.Message, args.Duration)
	case "list":
		return t.listTimers(ctx)
	case "cancel":
		return t.cancelTimer(ctx, args.ID)
	default:
		return Result{}, fmt.Errorf("timer: unknown action %q, use 'set', 'list', or 'cancel'", args.Action)
	}
}

func (t *timerTool) setTimer(ctx context.Context, message, duration string) (Result, error) {
	if message == "" {
		return Result{}, fmt.Errorf("timer.set: message is required")
	}
	if duration == "" {
		return Result{}, fmt.Errorf("timer.set: duration is required (e.g. 30s, 5m, 1h)")
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return Result{}, fmt.Errorf("timer.set: invalid duration %q: %w", duration, err)
	}
	if d <= 0 {
		return Result{}, fmt.Errorf("timer.set: duration must be positive")
	}
	if d > 24*time.Hour {
		return Result{}, fmt.Errorf("timer.set: max duration is 24h")
	}

	id := generateID()
	entry := &timerEntry{
		ID:        id,
		Message:   message,
		ExpiresAt: time.Now().Add(d),
		Duration:  duration,
	}

	t.mu.Lock()
	t.timers[id] = entry
	t.mu.Unlock()

	// Fire goroutine
	go func() {
		select {
		case <-time.After(d):
			t.mu.Lock()
			delete(t.timers, id)
			t.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}()

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"id":       id,
		"message":  message,
		"duration": duration,
		"expires":  entry.ExpiresAt.Format(time.RFC3339),
		"set":      true,
	})
	return Result{Content: string(out)}, nil
}

func (t *timerTool) listTimers(ctx context.Context) (Result, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	var entries []map[string]any
	now := time.Now()
	for _, e := range t.timers {
		entries = append(entries, map[string]any{
			"id":        e.ID,
			"message":   e.Message,
			"expiresAt": e.ExpiresAt.Format(time.RFC3339),
			"duration":  e.Duration,
			"remaining": e.ExpiresAt.Sub(now).String(),
		})
	}

	out, _ := json.Marshal(map[string]any{
		"count":  len(entries),
		"timers": entries,
	})
	return Result{Content: string(out)}, nil
}

func (t *timerTool) cancelTimer(ctx context.Context, id string) (Result, error) {
	if id == "" {
		return Result{}, fmt.Errorf("timer.cancel: id is required")
	}

	t.mu.Lock()
	entry, ok := t.timers[id]
	if ok {
		delete(t.timers, id)
	}
	t.mu.Unlock()

	if !ok {
		return Result{}, fmt.Errorf("timer.cancel: timer %q not found", id)
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"id":        id,
		"message":   entry.Message,
		"cancelled": true,
	})
	return Result{Content: string(out)}, nil
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
