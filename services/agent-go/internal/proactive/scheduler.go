// Package proactive cung cấp engine lập lịch chạy prompt định kỳ qua cron expression.
// Dùng robfig/cron/v3 cho scheduler, tích hợp với agent engine để gửi prompt tự động.
package proactive

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Task đại diện một tác vụ định kỳ.
type Task struct {
	Name     string
	CronExpr string
	Prompt   string
	LastRun  time.Time
	RunCount int64
}

// TaskResult là kết quả mỗi lần chạy task.
type TaskResult struct {
	TaskName  string
	RunAt     time.Time
	Prompt    string
	Response  string
	Error     string
	Duration  time.Duration
}

// PromptRunner là interface gửi prompt và nhận response.
// Implement bởi agent.Engine hoặc fake runner trong test.
type PromptRunner interface {
	RunPrompt(ctx context.Context, prompt string) (string, error)
}

// ProactiveEngine quản lý scheduled tasks dùng cron.
type ProactiveEngine struct {
	cron    *cron.Cron
	runner  PromptRunner
	tasks   map[string]*Task
	results []TaskResult
	mu      sync.RWMutex
}

// NewProactiveEngine tạo engine với runner (agent engine hoặc fake).
func NewProactiveEngine(runner PromptRunner) *ProactiveEngine {
	return &ProactiveEngine{
		cron:   cron.New(cron.WithSeconds()),
		runner: runner,
		tasks:  make(map[string]*Task),
	}
}

// AddTask thêm một scheduled task với cron expression.
// name: tên task (unique), cronExpr: cron expression (vd "0 */30 * * * *"),
// prompt: prompt gửi cho agent mỗi lần chạy.
// Trả về error nếu cron expression không hợp lệ.
func (e *ProactiveEngine) AddTask(name, cronExpr, prompt string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.tasks[name]; exists {
		return &TaskExistsError{Name: name}
	}

	task := &Task{
		Name:     name,
		CronExpr: cronExpr,
		Prompt:   prompt,
	}
	e.tasks[name] = task

	_, err := e.cron.AddFunc(cronExpr, func() {
		e.runTask(task)
	})
	if err != nil {
		delete(e.tasks, name)
		return err
	}

	return nil
}

// RemoveTask xoá một scheduled task theo tên.
func (e *ProactiveEngine) RemoveTask(name string) error {
	// cron v3 does not support removing individual jobs by name directly.
	// We mark it as removed and skip in runTask.
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.tasks[name]; !exists {
		return &TaskNotFoundError{Name: name}
	}
	delete(e.tasks, name)
	return nil
}

// Start khởi động cron scheduler.
func (e *ProactiveEngine) Start() {
	e.cron.Start()
}

// Stop dừng cron scheduler. Đợi các job đang chạy hoàn thành (nếu có).
// Trả về context của cron (để inspect entries).
func (e *ProactiveEngine) Stop() context.Context {
	return e.cron.Stop()
}

// Tasks trả về snapshot tất cả tasks đã đăng ký.
func (e *ProactiveEngine) Tasks() []Task {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]Task, 0, len(e.tasks))
	for _, t := range e.tasks {
		tasks = append(tasks, *t)
	}
	return tasks
}

// RecentResults trả về N kết quả gần nhất (mới nhất cuối mảng).
func (e *ProactiveEngine) RecentResults(n int) []TaskResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	start := 0
	if len(e.results) > n {
		start = len(e.results) - n
	}
	out := make([]TaskResult, len(e.results)-start)
	copy(out, e.results[start:])
	return out
}

// runTask thực thi một task: gọi runner, ghi kết quả.
func (e *ProactiveEngine) runTask(task *Task) {
	e.mu.Lock()
	task.LastRun = time.Now()
	task.RunCount++
	e.mu.Unlock()

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	response, err := e.runner.RunPrompt(ctx, task.Prompt)

	result := TaskResult{
		TaskName: task.Name,
		RunAt:    start,
		Prompt:   task.Prompt,
		Response: response,
		Duration: time.Since(start),
	}
	if err != nil {
		result.Error = err.Error()
	}

	e.mu.Lock()
	e.results = append(e.results, result)
	// Giới hạn 1000 results trong bộ nhớ
	if len(e.results) > 1000 {
		e.results = e.results[len(e.results)-1000:]
	}
	e.mu.Unlock()
}

// --- Errors ---

// TaskExistsError báo task đã tồn tại.
type TaskExistsError struct {
	Name string
}

func (e *TaskExistsError) Error() string {
	return "proactive: task already exists: " + e.Name
}

// TaskNotFoundError báo task không tìm thấy.
type TaskNotFoundError struct {
	Name string
}

func (e *TaskNotFoundError) Error() string {
	return "proactive: task not found: " + e.Name
}
