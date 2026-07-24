package proactive

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeRunner implements PromptRunner for testing.
type fakeRunner struct {
	mu       sync.Mutex
	response string
	err      error
	calls    []string // prompts received
}

func (f *fakeRunner) RunPrompt(_ context.Context, prompt string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, prompt)
	f.mu.Unlock()
	return f.response, f.err
}

func TestAddTask(t *testing.T) {
	runner := &fakeRunner{response: "ok"}
	engine := NewProactiveEngine(runner)

	err := engine.AddTask("morning-brief", "0 0 8 * * *", "Summarize today")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	tasks := engine.Tasks()
	if len(tasks) != 1 {
		t.Fatalf("Tasks len = %d, want 1", len(tasks))
	}
	if tasks[0].Name != "morning-brief" {
		t.Errorf("Task.Name = %q, want %q", tasks[0].Name, "morning-brief")
	}
	if tasks[0].CronExpr != "0 0 8 * * *" {
		t.Errorf("Task.CronExpr = %q", tasks[0].CronExpr)
	}
}

func TestAddTaskDuplicate(t *testing.T) {
	engine := NewProactiveEngine(&fakeRunner{})

	if err := engine.AddTask("task1", "0 */5 * * * *", "ping"); err != nil {
		t.Fatalf("first AddTask: %v", err)
	}

	err := engine.AddTask("task1", "0 */10 * * * *", "ping again")
	if err == nil {
		t.Fatal("expected error for duplicate task name")
	}
	var taskErr *TaskExistsError
	if !errors.As(err, &taskErr) {
		t.Fatalf("err = %v, want *TaskExistsError", err)
	}
	if taskErr.Name != "task1" {
		t.Errorf("TaskExistsError.Name = %q, want %q", taskErr.Name, "task1")
	}
}

func TestAddTaskInvalidCron(t *testing.T) {
	engine := NewProactiveEngine(&fakeRunner{})

	err := engine.AddTask("bad", "not-a-cron-expression", "test")
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
	// Invalid cron should not leave the task in the map
	if len(engine.Tasks()) != 0 {
		t.Errorf("Tasks len = %d, want 0 after invalid cron", len(engine.Tasks()))
	}
}

func TestRemoveTask(t *testing.T) {
	engine := NewProactiveEngine(&fakeRunner{})

	if err := engine.AddTask("temp", "0 0 0 * * *", "test"); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	if err := engine.RemoveTask("temp"); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}

	if len(engine.Tasks()) != 0 {
		t.Errorf("Tasks len = %d, want 0 after remove", len(engine.Tasks()))
	}

	// Remove non-existent task
	err := engine.RemoveTask("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
	var notFound *TaskNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v, want *TaskNotFoundError", err)
	}
}

func TestStartStop(t *testing.T) {
	engine := NewProactiveEngine(&fakeRunner{})
	engine.Start()
	ctx := engine.Stop()

	// Stop returns a context; wait for it to signal done
	select {
	case <-ctx.Done():
		// ok — cron stopped
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() context did not complete within 2s")
	}
}

func TestTaskExecution(t *testing.T) {
	runner := &fakeRunner{response: "hello world"}
	engine := NewProactiveEngine(runner)

	// Use a cron expression that fires every second for quick testing
	err := engine.AddTask("fast", "* * * * * *", "test prompt")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	engine.Start()

	// Wait for at least one execution
	time.Sleep(1500 * time.Millisecond)
	engine.Stop()

	runner.mu.Lock()
	calls := runner.calls
	runner.mu.Unlock()

	if len(calls) == 0 {
		t.Fatal("task did not execute within 1.5s")
	}
	if calls[0] != "test prompt" {
		t.Errorf("prompt = %q, want %q", calls[0], "test prompt")
	}

	results := engine.RecentResults(10)
	if len(results) == 0 {
		t.Fatal("no results recorded")
	}
	if results[0].TaskName != "fast" {
		t.Errorf("result.TaskName = %q, want %q", results[0].TaskName, "fast")
	}
	if results[0].Response != "hello world" {
		t.Errorf("result.Response = %q, want %q", results[0].Response, "hello world")
	}
}

func TestTaskExecutionError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("simulated failure")}
	engine := NewProactiveEngine(runner)

	err := engine.AddTask("failing", "* * * * * *", "bad prompt")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	engine.Start()
	time.Sleep(1500 * time.Millisecond)
	engine.Stop()

	results := engine.RecentResults(10)
	if len(results) == 0 {
		t.Fatal("no results recorded for failing task")
	}
	if results[0].Error != "simulated failure" {
		t.Errorf("result.Error = %q, want %q", results[0].Error, "simulated failure")
	}
}

func TestRecentResults_Bounds(t *testing.T) {
	engine := NewProactiveEngine(&fakeRunner{response: "ok"})

	// Directly inject results (bypass cron)
	for i := 0; i < 10; i++ {
		engine.mu.Lock()
		engine.results = append(engine.results, TaskResult{TaskName: "t"})
		engine.mu.Unlock()
	}

	results := engine.RecentResults(3)
	if len(results) != 3 {
		t.Fatalf("RecentResults(3) len = %d, want 3", len(results))
	}

	results = engine.RecentResults(100)
	if len(results) != 10 {
		t.Fatalf("RecentResults(100) len = %d, want 10 (all)", len(results))
	}
}

func TestTasks_SnapshotIsolation(t *testing.T) {
	engine := NewProactiveEngine(&fakeRunner{})
	engine.AddTask("a", "0 0 0 * * *", "a")
	engine.AddTask("b", "0 0 0 * * *", "b")

	tasks := engine.Tasks()
	// Mutating returned slice should not affect engine
	tasks[0].Name = "hacked"

	actual := engine.Tasks()
	if actual[0].Name != "a" {
		t.Errorf("Tasks returned shared reference: got %q", actual[0].Name)
	}
}
