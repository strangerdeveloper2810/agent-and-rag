package guardrails

import (
	"encoding/json"
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
