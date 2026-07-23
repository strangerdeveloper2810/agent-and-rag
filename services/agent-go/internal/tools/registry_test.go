package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	if _, ok := r.Get("echo"); ok {
		t.Fatal("Get on empty registry should return ok=false")
	}

	echo := NewEchoTool()
	r.Register(echo)

	got, ok := r.Get("echo")
	if !ok {
		t.Fatal("Get(\"echo\") ok=false after Register")
	}
	if got.Name() != "echo" {
		t.Fatalf("Get returned wrong tool: name=%q", got.Name())
	}
	if got.Kind() != KindRead {
		t.Fatalf("echo Kind = %v, want KindRead", got.Kind())
	}

	if all := r.All(); len(all) != 1 || all[0].Name() != "echo" {
		t.Fatalf("All() = %v, want single echo tool", all)
	}
}

func TestRegistry_ToolDefs(t *testing.T) {
	r := NewRegistry()
	echo := NewEchoTool()
	r.Register(echo)

	defs := r.ToolDefs()
	if len(defs) != 1 {
		t.Fatalf("ToolDefs len = %d, want 1", len(defs))
	}
	d := defs[0]
	if d.Name != echo.Name() {
		t.Errorf("ToolDef.Name = %q, want %q", d.Name, echo.Name())
	}
	if d.Description != echo.Description() {
		t.Errorf("ToolDef.Description = %q, want %q", d.Description, echo.Description())
	}
	if string(d.Schema) != string(echo.Schema()) {
		t.Errorf("ToolDef.Schema = %s, want %s", d.Schema, echo.Schema())
	}
}

func TestRegistry_RunParallel_OrderAndError(t *testing.T) {
	r := NewRegistry()
	r.Register(NewEchoTool())

	calls := []provider.ToolCall{
		{ID: "c1", Name: "echo", Args: json.RawMessage(`{"n":1}`)},
		{ID: "c2", Name: "echo", Args: json.RawMessage(`{"n":2}`)},
		{ID: "c3", Name: "missing", Args: json.RawMessage(`{}`)},
	}

	results := r.RunParallel(context.Background(), calls)

	if len(results) != len(calls) {
		t.Fatalf("results len = %d, want %d", len(results), len(calls))
	}

	// Thứ tự đầu vào phải được giữ nguyên.
	for i, res := range results {
		if res.Call.ID != calls[i].ID {
			t.Fatalf("results[%d].Call.ID = %q, want %q (order not preserved)", i, res.Call.ID, calls[i].ID)
		}
	}

	// Hai echo call đầu: không lỗi, Content == args.
	for i := 0; i < 2; i++ {
		if results[i].Err != nil {
			t.Fatalf("results[%d].Err = %v, want nil", i, results[i].Err)
		}
		if got, want := results[i].Result.Content, string(calls[i].Args); got != want {
			t.Errorf("results[%d].Content = %q, want %q", i, got, want)
		}
	}

	// Call tool không tồn tại: phải có Err (NotFoundError), không panic.
	if results[2].Err == nil {
		t.Fatal("results[2].Err = nil, want error for missing tool")
	}
	var nf *NotFoundError
	if !errors.As(results[2].Err, &nf) {
		t.Fatalf("results[2].Err = %v, want *NotFoundError", results[2].Err)
	}
	if nf.Name != "missing" {
		t.Errorf("NotFoundError.Name = %q, want %q", nf.Name, "missing")
	}
}

// barrierTool chỉ hoàn thành khi có ĐỦ số goroutine cùng chạy — chứng minh
// RunParallel thực sự chạy song song chứ không tuần tự.
type barrierTool struct {
	wg *sync.WaitGroup
}

func (barrierTool) Name() string             { return "barrier" }
func (barrierTool) Description() string       { return "test barrier tool" }
func (barrierTool) Schema() json.RawMessage   { return json.RawMessage(`{"type":"object"}`) }
func (barrierTool) Kind() Kind                { return KindRead }
func (b barrierTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	b.wg.Done() // báo "tôi đã bắt đầu"
	// Chờ mọi goroutine cùng tới đây; nếu chạy tuần tự sẽ deadlock → test timeout.
	done := make(chan struct{})
	go func() { b.wg.Wait(); close(done) }()
	select {
	case <-done:
		return Result{Content: "ok"}, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

func TestRegistry_RunParallel_ActuallyConcurrent(t *testing.T) {
	const n = 3
	var wg sync.WaitGroup
	wg.Add(n)

	r := NewRegistry()
	r.Register(barrierTool{wg: &wg})

	calls := make([]provider.ToolCall, n)
	for i := range calls {
		calls[i] = provider.ToolCall{ID: string(rune('a' + i)), Name: "barrier", Args: json.RawMessage(`{}`)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	results := r.RunParallel(ctx, calls)
	for i, res := range results {
		if res.Err != nil {
			t.Fatalf("results[%d].Err = %v (barrier not concurrent → likely deadlock/timeout)", i, res.Err)
		}
		if res.Result.Content != "ok" {
			t.Errorf("results[%d].Content = %q, want %q", i, res.Result.Content, "ok")
		}
	}
}
