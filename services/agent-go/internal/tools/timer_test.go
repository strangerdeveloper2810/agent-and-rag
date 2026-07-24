package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTimerTool(t *testing.T) {
	t.Run("set timer with valid duration", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action":   "set",
			"message":  "take a break",
			"duration": "5s",
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			ID       string `json:"id"`
			Message  string `json:"message"`
			Duration string `json:"duration"`
			Set      bool   `json:"set"`
		}
		if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if out.ID == "" {
			t.Error("expected non-empty ID")
		}
		if out.Message != "take a break" {
			t.Errorf("message: got %q, want %q", out.Message, "take a break")
		}
		if out.Duration != "5s" {
			t.Errorf("duration: got %q, want %q", out.Duration, "5s")
		}
		if !out.Set {
			t.Error("expected set=true")
		}
	})

	t.Run("set timer with 1m duration", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action":   "set",
			"message":  "check email",
			"duration": "1m",
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Duration string `json:"duration"`
			Set      bool   `json:"set"`
		}
		json.Unmarshal([]byte(res.Content), &out)
		if out.Duration != "1m" {
			t.Errorf("duration: got %q, want %q", out.Duration, "1m")
		}
		if !out.Set {
			t.Error("expected set=true")
		}
	})

	t.Run("set timer missing message", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action":   "set",
			"duration": "5s",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing message, got nil")
		}
		if !strings.Contains(err.Error(), "message is required") {
			t.Errorf("expected 'message is required' error, got: %v", err)
		}
	})

	t.Run("set timer missing duration", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action":  "set",
			"message": "test",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing duration, got nil")
		}
		if !strings.Contains(err.Error(), "duration is required") {
			t.Errorf("expected 'duration is required' error, got: %v", err)
		}
	})

	t.Run("set timer invalid duration", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action":   "set",
			"message":  "test",
			"duration": "not-a-duration",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for invalid duration, got nil")
		}
		if !strings.Contains(err.Error(), "invalid duration") {
			t.Errorf("expected 'invalid duration' error, got: %v", err)
		}
	})

	t.Run("set timer zero duration", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action":   "set",
			"message":  "test",
			"duration": "0s",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for zero duration, got nil")
		}
		if !strings.Contains(err.Error(), "duration must be positive") {
			t.Errorf("expected 'duration must be positive' error, got: %v", err)
		}
	})

	t.Run("set timer exceeds max duration", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action":   "set",
			"message":  "test",
			"duration": "25h",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for exceeding max duration, got nil")
		}
		if !strings.Contains(err.Error(), "max duration is 24h") {
			t.Errorf("expected 'max duration is 24h' error, got: %v", err)
		}
	})

	t.Run("list timers shows active timers", func(t *testing.T) {
		tool := NewTimerTool()

		// Set a timer first
		setArgs, _ := json.Marshal(map[string]string{
			"action":   "set",
			"message":  "test timer",
			"duration": "10m",
		})
		_, err := tool.Execute(context.Background(), setArgs)
		if err != nil {
			t.Fatalf("failed to set timer: %v", err)
		}

		// List timers
		listArgs, _ := json.Marshal(map[string]string{
			"action": "list",
		})
		res, err := tool.Execute(context.Background(), listArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Count  int `json:"count"`
			Timers []struct {
				ID       string `json:"id"`
				Message  string `json:"message"`
				Duration string `json:"duration"`
			} `json:"timers"`
		}
		if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if out.Count < 1 {
			t.Errorf("expected at least 1 timer, got %d", out.Count)
		}
		found := false
		for _, timer := range out.Timers {
			if timer.Message == "test timer" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected timer with message 'test timer' not found in list")
		}
	})

	t.Run("cancel timer removes timer", func(t *testing.T) {
		tool := NewTimerTool()

		// Set a timer
		setArgs, _ := json.Marshal(map[string]string{
			"action":   "set",
			"message":  "to be cancelled",
			"duration": "10m",
		})
		setRes, err := tool.Execute(context.Background(), setArgs)
		if err != nil {
			t.Fatalf("failed to set timer: %v", err)
		}
		var setOut struct {
			ID string `json:"id"`
		}
		json.Unmarshal([]byte(setRes.Content), &setOut)

		// Cancel it
		cancelArgs, _ := json.Marshal(map[string]string{
			"action": "cancel",
			"id":     setOut.ID,
		})
		cancelRes, err := tool.Execute(context.Background(), cancelArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var cancelOut struct {
			ID        string `json:"id"`
			Message   string `json:"message"`
			Cancelled bool   `json:"cancelled"`
		}
		json.Unmarshal([]byte(cancelRes.Content), &cancelOut)
		if !cancelOut.Cancelled {
			t.Error("expected cancelled=true")
		}
		if cancelOut.ID != setOut.ID {
			t.Errorf("cancelled id: got %q, want %q", cancelOut.ID, setOut.ID)
		}

		// List should not contain it
		listArgs, _ := json.Marshal(map[string]string{"action": "list"})
		listRes, _ := tool.Execute(context.Background(), listArgs)
		var listOut struct {
			Count int `json:"count"`
		}
		json.Unmarshal([]byte(listRes.Content), &listOut)
		if listOut.Count != 0 {
			t.Errorf("expected 0 timers after cancel, got %d", listOut.Count)
		}
	})

	t.Run("cancel non-existent timer", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action": "cancel",
			"id":     "nonexistent",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for non-existent timer, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("cancel missing id", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action": "cancel",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing id, got nil")
		}
		if !strings.Contains(err.Error(), "id is required") {
			t.Errorf("expected 'id is required' error, got: %v", err)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		tool := NewTimerTool()
		args, _ := json.Marshal(map[string]string{
			"action": "invalid",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for unknown action, got nil")
		}
		if !strings.Contains(err.Error(), "unknown action") {
			t.Errorf("expected 'unknown action' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		tool := NewTimerTool()
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "invalid args") {
			t.Errorf("expected 'invalid args' error, got: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		tool := NewTimerTool()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		args, _ := json.Marshal(map[string]string{
			"action": "list",
		})
		_, err := tool.Execute(ctx, args)
		if err == nil {
			t.Fatal("expected error for cancelled context, got nil")
		}
	})
}

func TestTimerToolInterface(t *testing.T) {
	var tool Tool = NewTimerTool()
	if tool.Name() != "timer" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "timer")
	}
	if tool.Kind() != KindRead {
		t.Errorf("Kind: got %v, want KindRead", tool.Kind())
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema is empty")
	}
}
