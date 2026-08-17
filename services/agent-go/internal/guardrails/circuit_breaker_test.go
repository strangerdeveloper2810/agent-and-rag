package guardrails

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCircuitBreaker_SameThreeTimes_ReturnsError(t *testing.T) {
	cb := NewCircuitBreaker(3)
	args := json.RawMessage(`{"key":"value"}`)

	// 1st call — allowed
	if err := cb.Record("echo", args); err != nil {
		t.Fatalf("1st call should not error, got: %v", err)
	}

	// 2nd call — allowed
	if err := cb.Record("echo", args); err != nil {
		t.Fatalf("2nd call should not error, got: %v", err)
	}

	// 3rd call — should error (stuck loop)
	err := cb.Record("echo", args)
	if err == nil {
		t.Fatal("3rd call with same tool+args should return StuckLoopError")
	}
	if _, ok := err.(*StuckLoopError); !ok {
		t.Fatalf("expected *StuckLoopError, got %T: %v", err, err)
	}
}

func TestCircuitBreaker_DifferentTools_NoError(t *testing.T) {
	cb := NewCircuitBreaker(3)
	args := json.RawMessage(`{"key":"value"}`)

	// Call different tools — each resets the counter
	tools := []string{"echo", "search", "read", "echo", "search"}
	for i, name := range tools {
		if err := cb.Record(name, args); err != nil {
			t.Fatalf("call %d (%q) should not error, got: %v", i, name, err)
		}
	}
}

func TestCircuitBreaker_ResetAfterDifferent(t *testing.T) {
	cb := NewCircuitBreaker(3)
	args := json.RawMessage(`{"key":"value"}`)

	// Call same tool twice
	if err := cb.Record("echo", args); err != nil {
		t.Fatalf("1st call should not error: %v", err)
	}
	if err := cb.Record("echo", args); err != nil {
		t.Fatalf("2nd call should not error: %v", err)
	}

	// Call different tool → resets counter
	if err := cb.Record("search", args); err != nil {
		t.Fatalf("different tool should not error: %v", err)
	}

	// Call original tool again — counter starts from 1
	if err := cb.Record("echo", args); err != nil {
		t.Fatalf("should not error after reset: %v", err)
	}
	if err := cb.Record("echo", args); err != nil {
		t.Fatalf("2nd call after reset should not error: %v", err)
	}
	// Still only 2 consecutive "echo" calls after the reset → no error
}

func TestCircuitBreaker_DefaultMaxRepeats(t *testing.T) {
	args := json.RawMessage(`{}`)
	for _, n := range []int{0, -3} {
		cb := NewCircuitBreaker(n)
		// maxRepeats <= 0 → default 3: 2 lần đầu qua, lần 3 lỗi.
		if err := cb.Record("t", args); err != nil {
			t.Fatalf("NewCircuitBreaker(%d): call 1 = %v, want nil", n, err)
		}
		if err := cb.Record("t", args); err != nil {
			t.Fatalf("NewCircuitBreaker(%d): call 2 = %v, want nil", n, err)
		}
		if err := cb.Record("t", args); err == nil {
			t.Fatalf("NewCircuitBreaker(%d): call 3 = nil, want StuckLoopError", n)
		}
	}
}

func TestCircuitBreaker_DifferentArgsResets(t *testing.T) {
	cb := NewCircuitBreaker(3)
	a := json.RawMessage(`{"x":1}`)
	b := json.RawMessage(`{"x":2}`)

	for i, args := range []json.RawMessage{a, b, a, a} {
		if err := cb.Record("echo", args); err != nil {
			t.Fatalf("call %d = %v, want nil (khác args reset bộ đếm)", i, err)
		}
	}
	// Lần thứ 3 liên tiếp cùng a → lỗi.
	if err := cb.Record("echo", a); err == nil {
		t.Fatal("call 5 = nil, want StuckLoopError")
	}
}

func TestCircuitBreaker_ExplicitReset(t *testing.T) {
	cb := NewCircuitBreaker(3)
	args := json.RawMessage(`{}`)

	if err := cb.Record("t", args); err != nil {
		t.Fatalf("call 1 = %v", err)
	}
	if err := cb.Record("t", args); err != nil {
		t.Fatalf("call 2 = %v", err)
	}
	cb.Reset()
	if err := cb.Record("t", args); err != nil {
		t.Fatalf("call sau Reset = %v, want nil", err)
	}
	if err := cb.Record("t", args); err != nil {
		t.Fatalf("call 2 sau Reset = %v, want nil", err)
	}
	if err := cb.Record("t", args); err == nil {
		t.Fatal("call 3 sau Reset = nil, want StuckLoopError")
	}
}

func TestCircuitBreaker_MaxRepeatsOne(t *testing.T) {
	cb := NewCircuitBreaker(1)
	err := cb.Record("t", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("maxRepeats=1: call đầu nên lỗi ngay")
	}
	if _, ok := err.(*StuckLoopError); !ok {
		t.Fatalf("err = %T, want *StuckLoopError", err)
	}
}

func TestCircuitBreaker_KeepsErroring(t *testing.T) {
	cb := NewCircuitBreaker(3)
	args := json.RawMessage(`{}`)

	if err := cb.Record("t", args); err != nil {
		t.Fatalf("call 1 = %v", err)
	}
	if err := cb.Record("t", args); err != nil {
		t.Fatalf("call 2 = %v", err)
	}
	err := cb.Record("t", args)
	if err == nil {
		t.Fatal("call 3 = nil, want StuckLoopError")
	}
	loopErr, ok := err.(*StuckLoopError)
	if !ok {
		t.Fatalf("err = %T, want *StuckLoopError", err)
	}
	if loopErr.Count != 3 {
		t.Fatalf("Count = %d, want 3", loopErr.Count)
	}
	// Sau khi đã lỗi, count tiếp tục tăng → vẫn lỗi.
	err = cb.Record("t", args)
	loopErr, ok = err.(*StuckLoopError)
	if !ok {
		t.Fatalf("err = %T, want *StuckLoopError", err)
	}
	if loopErr.Count != 4 {
		t.Fatalf("Count = %d, want 4", loopErr.Count)
	}
}

func TestStuckLoopError_Error(t *testing.T) {
	e := &StuckLoopError{Tool: "echo", Count: 5}
	msg := e.Error()
	if !strings.Contains(msg, "echo") || !strings.Contains(msg, "5") {
		t.Fatalf("Error() = %q, want chứa tên tool và count", msg)
	}
}
