package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestShellTool(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("echo hello", func(t *testing.T) {
		tool := NewShellTool(nil) // allow all
		args, _ := json.Marshal(map[string]any{
			"command": "echo",
			"args":    []string{"hello"},
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			ExitCode int    `json:"exitCode"`
			Output   string `json:"output"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.ExitCode != 0 {
			t.Errorf("expected exitCode 0, got %d", out.ExitCode)
		}
		if !strings.Contains(out.Output, "hello") {
			t.Errorf("expected output to contain 'hello', got %q", out.Output)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		tool := NewShellTool(nil)
		args, _ := json.Marshal(map[string]any{
			"command": "sleep",
			"args":    []string{"60"},
		})
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := tool.Execute(ctx, args)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		// The tool wraps with 30s context, but our ctx is 100ms -> should fail
	})

	t.Run("disallowed command blocked", func(t *testing.T) {
		tool := NewShellTool([]string{"echo", "ls"}) // only echo and ls allowed
		args, _ := json.Marshal(map[string]any{
			"command": "cat",
			"args":    []string{"/etc/hosts"},
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for disallowed command, got nil")
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("expected 'not allowed' error, got: %v", err)
		}
	})

	t.Run("allowed command passes", func(t *testing.T) {
		tool := NewShellTool([]string{"echo"})
		args, _ := json.Marshal(map[string]any{
			"command": "echo",
			"args":    []string{"allowed!"},
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out struct {
			ExitCode int    `json:"exitCode"`
			Output   string `json:"output"`
		}
		json.Unmarshal([]byte(res.Content), &out)
		if out.ExitCode != 0 {
			t.Errorf("expected exitCode 0, got %d. output=%q", out.ExitCode, out.Output)
		}
	})

	t.Run("command with stderr", func(t *testing.T) {
		tool := NewShellTool(nil)
		// ls on a nonexistent path produces stderr
		args, _ := json.Marshal(map[string]any{
			"command": "ls",
			"args":    []string{tmpDir + "/nonexistent"},
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			ExitCode int    `json:"exitCode"`
			Output   string `json:"output"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.ExitCode == 0 {
			t.Errorf("expected non-zero exit code for ls on nonexistent path")
		}
	})

	t.Run("missing command", func(t *testing.T) {
		tool := NewShellTool(nil)
		args, _ := json.Marshal(map[string]any{"command": ""})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing command, got nil")
		}
		if !strings.Contains(err.Error(), "command is required") {
			t.Errorf("expected 'command is required' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		tool := NewShellTool(nil)
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "invalid args") {
			t.Errorf("expected 'invalid args' error, got: %v", err)
		}
	})

	t.Run("output truncation", func(t *testing.T) {
		tool := NewShellTool(nil)
		// Generate a long string via printf
		longStr := strings.Repeat("A", 9000)
		args, _ := json.Marshal(map[string]any{
			"command": "printf",
			"args":    []string{"%s", longStr},
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Output string `json:"output"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if !strings.Contains(out.Output, "[truncated]") {
			t.Errorf("expected truncated output, got length %d", len(out.Output))
		}
		if len(out.Output) > shellMaxOutput+len("\n... [truncated]")+100 {
			t.Errorf("output too long: %d chars", len(out.Output))
		}
	})
}

func TestShellToolInterface(t *testing.T) {
	tool := NewShellTool(nil)
	var _ Tool = tool // compile-time check

	if tool.Name() != "shell.exec" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "shell.exec")
	}
	if tool.Kind() != KindDestructive {
		t.Errorf("Kind: got %v, want KindDestructive", tool.Kind())
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema is empty")
	}
}

func TestNewShellTool_DefaultTimeout(t *testing.T) {
	tool := NewShellTool(nil).(*shellTool)
	if tool.Timeout() != defaultShellTimeout {
		t.Errorf("Timeout() = %v, want %v", tool.Timeout(), defaultShellTimeout)
	}
}

func TestNewShellToolWithTimeout(t *testing.T) {
	tool := NewShellToolWithTimeout(nil, 5*time.Second).(*shellTool)
	if tool.Timeout() != 5*time.Second {
		t.Errorf("Timeout() = %v, want 5s", tool.Timeout())
	}

	// timeout <= 0 rơi về default thay vì tắt hẳn deadline.
	fallback := NewShellToolWithTimeout(nil, 0).(*shellTool)
	if fallback.Timeout() != defaultShellTimeout {
		t.Errorf("Timeout() với 0 = %v, want default %v", fallback.Timeout(), defaultShellTimeout)
	}
}

// shell.exec chạy qua Registry với timeout ngắn phải bị TimeoutTool cắt sớm,
// không chờ lệnh "sleep" chạy hết.
func TestShellTool_TimeoutViaRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(NewShellToolWithTimeout([]string{"sleep"}, 50*time.Millisecond))

	start := time.Now()
	results := r.RunParallel(context.Background(), []provider.ToolCall{
		{ID: "c1", Name: "shell.exec", Args: json.RawMessage(`{"command":"sleep","args":["5"]}`)},
	})
	elapsed := time.Since(start)

	if elapsed > 4*time.Second {
		t.Fatalf("elapsed = %v, want bị cắt ở ~50ms, không chờ sleep 5s", elapsed)
	}
	var timeoutErr *TimeoutError
	if !errors.As(results[0].Err, &timeoutErr) {
		t.Fatalf("Err = %v, want *TimeoutError", results[0].Err)
	}
}
