// Package guardrails provides safety checks for the agent runtime:
// circuit breaker (stuck loop detection) and tool guard (read/write/destructive).
package guardrails

import (
	"encoding/json"
	"fmt"
	"sync"
)

// StuckLoopError is returned by CircuitBreaker.Record when the same tool+args
// has been called consecutively maxRepeats times, indicating a stuck loop.
type StuckLoopError struct {
	Tool  string
	Count int
}

func (e *StuckLoopError) Error() string {
	return fmt.Sprintf("guardrails: stuck loop detected — %q called %d times consecutively with same args", e.Tool, e.Count)
}

// callKey identifies a unique (tool, args) pair for dedup tracking.
type callKey struct {
	tool string
	args string // string(json.RawMessage) — stable comparison
}

// CircuitBreaker tracks consecutive identical tool calls.
// When the same tool+args is called maxRepeats times in a row, Record returns
// a StuckLoopError. A different call resets the counter.
type CircuitBreaker struct {
	mu         sync.Mutex
	last       callKey
	count      int
	maxRepeats int
}

// NewCircuitBreaker creates a CircuitBreaker. If maxRepeats <= 0, defaults to 3.
func NewCircuitBreaker(maxRepeats int) *CircuitBreaker {
	if maxRepeats <= 0 {
		maxRepeats = 3
	}
	return &CircuitBreaker{maxRepeats: maxRepeats}
}

// Record registers a tool call and returns an error if the same tool+args has
// been called maxRepeats times consecutively. Thread-safe.
func (cb *CircuitBreaker) Record(toolName string, args json.RawMessage) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	key := callKey{tool: toolName, args: string(args)}
	if key == cb.last {
		cb.count++
	} else {
		cb.last = key
		cb.count = 1
	}

	if cb.count >= cb.maxRepeats {
		return &StuckLoopError{Tool: toolName, Count: cb.count}
	}
	return nil
}

// Reset clears the circuit breaker state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.last = callKey{}
	cb.count = 0
}
