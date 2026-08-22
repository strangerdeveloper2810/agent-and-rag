package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/orchestrator"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// fakePausedStore implements both agent.InterruptStore (Save) and
// PausedRunStore (Load/Delete) — cùng 1 fake trong bộ nhớ dùng cho cả chiều
// Engine ghi lúc dừng ở NodeInterrupt lẫn chiều handler đọc/xoá lúc resume.
type fakePausedStore struct {
	mu        sync.Mutex
	data      map[string][]byte
	agentName map[string]string
	deleted   map[string]bool
}

func newFakePausedStore() *fakePausedStore {
	return &fakePausedStore{data: map[string][]byte{}, agentName: map[string]string{}, deleted: map[string]bool{}}
}

func (f *fakePausedStore) SaveInterruptedState(runID, agentName string, stateJSON []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[runID] = append([]byte(nil), stateJSON...)
	f.agentName[runID] = agentName
	return nil
}

func (f *fakePausedStore) LoadInterruptedState(runID string) ([]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.data[runID]
	if !ok {
		return nil, "", &notFoundError{runID}
	}
	return data, f.agentName[runID], nil
}

func (f *fakePausedStore) DeleteInterruptedState(runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted[runID] = true
	delete(f.data, runID)
	return nil
}

type notFoundError struct{ runID string }

func (e *notFoundError) Error() string { return "paused run not found: " + e.runID }

// scriptedFakeProvider trả kịch bản chunk KHÁC NHAU cho mỗi lần Generate được
// gọi — cần thế để mô phỏng: lượt 1 model gọi tool destructive, lượt 2 (sau
// resume) model trả lời bằng text.
type scriptedFakeProvider struct {
	mu      sync.Mutex
	scripts [][]provider.StreamChunk
	calls   int
}

func (s *scriptedFakeProvider) Name() string { return "scripted-fake" }

func (s *scriptedFakeProvider) Generate(_ context.Context, _ provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	script := s.scripts[len(s.scripts)-1]
	if s.calls < len(s.scripts) {
		script = s.scripts[s.calls]
	}
	s.calls++
	ch := make(chan provider.StreamChunk, len(script)+1)
	for _, c := range script {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// destructiveStubTool là tool destructive tối giản cho test.
type destructiveStubTool struct{ name, output string }

func (t *destructiveStubTool) Name() string            { return t.name }
func (t *destructiveStubTool) Description() string     { return "destructive stub" }
func (t *destructiveStubTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (t *destructiveStubTool) Kind() tools.Kind        { return tools.KindDestructive }
func (t *destructiveStubTool) Execute(context.Context, json.RawMessage) (tools.Result, error) {
	return tools.Result{Content: t.output}, nil
}

func TestChatResumeHandler_MissingRunID(t *testing.T) {
	h := NewChatResumeHandler(orchestrator.New(), newFakePausedStore())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat/resume", strings.NewReader(`{"answer":"yes"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestChatResumeHandler_RunIDNotFound(t *testing.T) {
	h := NewChatResumeHandler(orchestrator.New(), newFakePausedStore())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat/resume", strings.NewReader(`{"run_id":"does-not-exist","answer":"yes"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestChatResumeHandler_EndToEnd chứng minh trọn vẹn đường dẫn HTTP thật:
// POST /chat (giả lập qua Engine.Run trực tiếp) dừng ở interrupt → lưu qua
// fakePausedStore → POST /chat/resume đọc lại, resolve, resume, và trả SSE
// chứa đúng câu trả lời cuối.
func TestChatResumeHandler_EndToEnd(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&destructiveStubTool{name: "task.delete", output: "deleted ok"})

	prov := &scriptedFakeProvider{scripts: [][]provider.StreamChunk{
		{
			{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "c1", Name: "task.delete", Args: []byte(`{}`)}},
			{Kind: provider.ChunkDone},
		},
		{
			{Kind: provider.ChunkText, Text: "Đã xoá xong."},
			{Kind: provider.ChunkDone},
		},
	}}

	eng := agent.NewEngine(prov, reg)
	store := newFakePausedStore()
	eng.SetInterruptStore(store)
	eng.SetName("code")

	orch := orchestrator.New()
	orch.Register(&orchestrator.AgentSpec{Name: "code", Engine: eng})
	if err := orch.SetDefault("code"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	var runID string
	emit := func(e agent.Event) {
		if e.Type == "interrupt" {
			runID = e.RunID
		}
	}
	if _, err := eng.Run(context.Background(), agent.RunInput{UserMessage: "xoá report.txt", MaxSteps: 5}, emit); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if runID == "" {
		t.Fatal("không lấy được RunID từ event interrupt")
	}

	h := NewChatResumeHandler(orch, store)
	rec := httptest.NewRecorder()
	body := `{"run_id":"` + runID + `","answer":"yes"}`
	req := httptest.NewRequest(http.MethodPost, "/chat/resume", strings.NewReader(body))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Đã xoá xong.") {
		t.Errorf("response body = %q, muốn chứa %q", rec.Body.String(), "Đã xoá xong.")
	}
	if !store.deleted[runID] {
		t.Error("paused_runs phải được xoá sau khi resume xong (dùng 1 lần)")
	}
	if prov.calls != 2 {
		t.Errorf("prov.calls = %d, want 2 (không chạy lại từ đầu)", prov.calls)
	}
}
