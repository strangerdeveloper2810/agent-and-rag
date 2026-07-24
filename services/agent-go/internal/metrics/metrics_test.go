package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestRecordRequest(t *testing.T) {
	m := New()

	m.RecordRequest(250*time.Millisecond, 100, 50)

	s := m.Snapshot()
	if s.Requests != 1 {
		t.Errorf("Requests = %d, want 1", s.Requests)
	}
	if s.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", s.InputTokens)
	}
	if s.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", s.OutputTokens)
	}
	if s.ToolCalls != 0 {
		t.Errorf("ToolCalls = %d, want 0", s.ToolCalls)
	}
	if s.Errors != 0 {
		t.Errorf("Errors = %d, want 0", s.Errors)
	}
	if s.LatencySumUs <= 0 {
		t.Error("LatencySumUs should be > 0")
	}
	if s.TakenAt.IsZero() {
		t.Error("TakenAt should be non-zero")
	}
}

func TestRecordToolCall(t *testing.T) {
	m := New()
	m.RecordToolCall()
	m.RecordToolCall()

	s := m.Snapshot()
	if s.ToolCalls != 2 {
		t.Errorf("ToolCalls = %d, want 2", s.ToolCalls)
	}
}

func TestRecordToolCallsBatch(t *testing.T) {
	m := New()
	m.RecordToolCalls(5)

	s := m.Snapshot()
	if s.ToolCalls != 5 {
		t.Errorf("ToolCalls = %d, want 5", s.ToolCalls)
	}
}

func TestRecordError(t *testing.T) {
	m := New()
	m.RecordError()
	m.RecordError()
	m.RecordError()

	s := m.Snapshot()
	if s.Errors != 3 {
		t.Errorf("Errors = %d, want 3", s.Errors)
	}
}

func TestRecordErrorsBatch(t *testing.T) {
	m := New()
	m.RecordErrors(10)

	s := m.Snapshot()
	if s.Errors != 10 {
		t.Errorf("Errors = %d, want 10", s.Errors)
	}
}

func TestLatencyHistogram(t *testing.T) {
	m := New()

	// Record requests with different latencies
	m.RecordRequest(50*time.Millisecond, 10, 5)    // <100ms
	m.RecordRequest(200*time.Millisecond, 10, 5)   // 100-500ms
	m.RecordRequest(750*time.Millisecond, 10, 5)   // 500ms-1s
	m.RecordRequest(2*time.Second, 10, 5)          // 1-3s
	m.RecordRequest(5*time.Second, 10, 5)          // 3-10s
	m.RecordRequest(15*time.Second, 10, 5)         // >10s

	s := m.Snapshot()
	if s.Requests != 6 {
		t.Errorf("Requests = %d, want 6", s.Requests)
	}
}

func TestReset(t *testing.T) {
	m := New()

	m.RecordRequest(time.Second, 100, 200)
	m.RecordToolCall()
	m.RecordError()

	m.Reset()

	s := m.Snapshot()
	if s.Requests != 0 {
		t.Errorf("Requests = %d, want 0 after reset", s.Requests)
	}
	if s.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", s.InputTokens)
	}
	if s.ToolCalls != 0 {
		t.Errorf("ToolCalls = %d, want 0", s.ToolCalls)
	}
	if s.Errors != 0 {
		t.Errorf("Errors = %d, want 0", s.Errors)
	}
}

func TestSnapshot_DoesNotReset(t *testing.T) {
	m := New()
	m.RecordRequest(time.Second, 10, 20)

	_ = m.Snapshot()
	s := m.Snapshot()

	if s.Requests != 1 {
		t.Errorf("Requests = %d, want 1 (snapshot should not reset)", s.Requests)
	}
}

func TestConcurrent(t *testing.T) {
	m := New()
	var wg sync.WaitGroup

	// 100 goroutines each recording 10 requests
	for g := 0; g < 100; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				m.RecordRequest(time.Millisecond*time.Duration(i*10), int64(i), int64(i*2))
				m.RecordToolCall()
				if i%5 == 0 {
					m.RecordError()
				}
			}
		}()
	}
	wg.Wait()

	s := m.Snapshot()

	expectedRequests := int64(100 * 10)
	if s.Requests != expectedRequests {
		t.Errorf("Requests = %d, want %d", s.Requests, expectedRequests)
	}
	if s.ToolCalls != expectedRequests {
		t.Errorf("ToolCalls = %d, want %d", s.ToolCalls, expectedRequests)
	}
	// 2 errors per goroutine (i=0,5 in 0..9) * 100 = 200
	expectedErrors := int64(200)
	if s.Errors != expectedErrors {
		t.Errorf("Errors = %d, want %d", s.Errors, expectedErrors)
	}
}

func TestSnapshot_IndependentCopy(t *testing.T) {
	m := New()
	m.RecordRequest(time.Second, 1, 1)

	s := m.Snapshot()
	// Modify the returned Snapshot — should not affect Metrics
	s.Requests = 999

	s2 := m.Snapshot()
	if s2.Requests != 1 {
		t.Errorf("Snapshot returned shared reference: Requests = %d, want 1", s2.Requests)
	}
}
