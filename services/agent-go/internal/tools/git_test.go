package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// setupGitRepo creates a temp directory, inits a git repo, and creates an initial commit.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit(t, dir, "init")
	// Configure git user for the test repo
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create and commit a file
	os.WriteFile(dir+"/README.md", []byte("# Test Repo\n"), 0o644)
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "initial commit")

	// Create second commit for richer test data
	os.WriteFile(dir+"/main.go", []byte("package main\n"), 0o644)
	runGit(t, dir, "add", "main.go")
	runGit(t, dir, "commit", "-m", "add main.go")

	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func TestGitTool(t *testing.T) {
	repoDir := setupGitRepo(t)

	t.Run("git status", func(t *testing.T) {
		tool := NewGitTool(repoDir)
		args, _ := json.Marshal(map[string]any{
			"operation": "status",
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
		if !strings.Contains(out.Output, "nothing to commit") && !strings.Contains(out.Output, "branch") {
			t.Errorf("expected status output, got: %s", out.Output)
		}
	})

	t.Run("git log", func(t *testing.T) {
		tool := NewGitTool(repoDir)
		args, _ := json.Marshal(map[string]any{
			"operation": "log",
			"args":      []string{"--oneline"},
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
		if !strings.Contains(out.Output, "initial commit") {
			t.Errorf("expected log to contain 'initial commit', got: %s", out.Output)
		}
		if !strings.Contains(out.Output, "add main.go") {
			t.Errorf("expected log to contain 'add main.go', got: %s", out.Output)
		}
	})

	t.Run("git branch", func(t *testing.T) {
		tool := NewGitTool(repoDir)
		args, _ := json.Marshal(map[string]any{
			"operation": "branch",
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
		// Should show at least the current branch (marked with *)
		if !strings.Contains(out.Output, "*") {
			t.Errorf("expected branch output with '*', got: %s", out.Output)
		}
	})

	t.Run("git diff", func(t *testing.T) {
		// Create an unstaged change first
		os.WriteFile(repoDir+"/README.md", []byte("# Test Repo\nModified line\n"), 0o644)

		tool := NewGitTool(repoDir)
		args, _ := json.Marshal(map[string]any{
			"operation": "diff",
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

		if !strings.Contains(out.Output, "Modified line") {
			t.Errorf("expected diff to contain 'Modified line', got: %s", out.Output)
		}
	})

	t.Run("git show", func(t *testing.T) {
		tool := NewGitTool(repoDir)
		args, _ := json.Marshal(map[string]any{
			"operation": "show",
			"args":      []string{"HEAD"},
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
		if !strings.Contains(out.Output, "add main.go") {
			t.Errorf("expected show to contain 'add main.go', got: %s", out.Output)
		}
	})

	t.Run("disallowed operation blocked", func(t *testing.T) {
		tool := NewGitTool(repoDir)
		args, _ := json.Marshal(map[string]any{
			"operation": "push",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for disallowed operation, got nil")
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("expected 'not allowed' error, got: %v", err)
		}
	})

	t.Run("missing operation", func(t *testing.T) {
		tool := NewGitTool(repoDir)
		args, _ := json.Marshal(map[string]any{"operation": ""})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing operation, got nil")
		}
		if !strings.Contains(err.Error(), "operation is required") {
			t.Errorf("expected 'operation is required' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		tool := NewGitTool(repoDir)
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "invalid args") {
			t.Errorf("expected 'invalid args' error, got: %v", err)
		}
	})
}

func TestGitToolInterface(t *testing.T) {
	tool := NewGitTool("")
	var _ Tool = tool // compile-time check

	if tool.Name() != "git" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "git")
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
