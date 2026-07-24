// Package metrics cung cấp metrics đơn giản cho agent: requests, tokens, latency,
// tool calls, errors. Thread-safe, snapshot-able.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Snapshot là ảnh chụp metrics tại một thời điểm.
type Snapshot struct {
	// Counters
	Requests  int64 `json:"requests"`
	ToolCalls int64 `json:"tool_calls"`
	Errors    int64 `json:"errors"`

	// Tokens
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`

	// Latency (cumulative microseconds → tính avg ở client)
	LatencySumUs int64 `json:"latency_sum_us"`

	// Timestamp
	TakenAt time.Time `json:"taken_at"`
}

// Metrics là collector thread-safe cho agent metrics.
// Dùng atomic cho counters (hiệu năng cao, không lock) và mutex cho histogram.
type Metrics struct {
	requests     atomic.Int64
	toolCalls    atomic.Int64
	errors       atomic.Int64
	inputTokens  atomic.Int64
	outputTokens atomic.Int64
	latencySumUs atomic.Int64

	// Histogram buckets (latency distribution)
	mu          sync.RWMutex
	latencyDist map[string]int64 // "<100ms", "100-500ms", etc.
}

// New tạo Metrics collector mới.
func New() *Metrics {
	return &Metrics{
		latencyDist: map[string]int64{
			"<100ms":    0,
			"100-500ms": 0,
			"500ms-1s":  0,
			"1-3s":      0,
			"3-10s":     0,
			">10s":      0,
		},
	}
}

// RecordRequest ghi nhận 1 request hoàn thành.
// latency: thời gian xử lý request, tokensIn/Out: số token input/output.
func (m *Metrics) RecordRequest(latency time.Duration, tokensIn, tokensOut int64) {
	m.requests.Add(1)
	m.inputTokens.Add(tokensIn)
	m.outputTokens.Add(tokensOut)
	m.latencySumUs.Add(latency.Microseconds())

	m.recordLatencyBucket(latency)
}

// RecordToolCall ghi nhận 1 tool call.
func (m *Metrics) RecordToolCall() {
	m.toolCalls.Add(1)
}

// RecordError ghi nhận 1 lỗi.
func (m *Metrics) RecordError() {
	m.errors.Add(1)
}

// RecordToolCalls ghi nhận n tool calls (batch).
func (m *Metrics) RecordToolCalls(n int64) {
	m.toolCalls.Add(n)
}

// RecordErrors ghi nhận n errors.
func (m *Metrics) RecordErrors(n int64) {
	m.errors.Add(n)
}

// Snapshot trả về ảnh chụp metrics hiện tại, không reset.
func (m *Metrics) Snapshot() Snapshot {
	m.mu.RLock()
	dist := make(map[string]int64, len(m.latencyDist))
	for k, v := range m.latencyDist {
		dist[k] = v
	}
	m.mu.RUnlock()

	return Snapshot{
		Requests:     m.requests.Load(),
		ToolCalls:    m.toolCalls.Load(),
		Errors:       m.errors.Load(),
		InputTokens:  m.inputTokens.Load(),
		OutputTokens: m.outputTokens.Load(),
		LatencySumUs: m.latencySumUs.Load(),
		TakenAt:      time.Now(),
	}
}

// Reset đưa tất cả counters về 0.
func (m *Metrics) Reset() {
	m.requests.Store(0)
	m.toolCalls.Store(0)
	m.errors.Store(0)
	m.inputTokens.Store(0)
	m.outputTokens.Store(0)
	m.latencySumUs.Store(0)

	m.mu.Lock()
	for k := range m.latencyDist {
		m.latencyDist[k] = 0
	}
	m.mu.Unlock()
}

// recordLatencyBucket thêm 1 vào bucket phù hợp dựa trên latency.
func (m *Metrics) recordLatencyBucket(d time.Duration) {
	var bucket string
	switch {
	case d < 100*time.Millisecond:
		bucket = "<100ms"
	case d < 500*time.Millisecond:
		bucket = "100-500ms"
	case d < 1*time.Second:
		bucket = "500ms-1s"
	case d < 3*time.Second:
		bucket = "1-3s"
	case d < 10*time.Second:
		bucket = "3-10s"
	default:
		bucket = ">10s"
	}

	m.mu.Lock()
	m.latencyDist[bucket]++
	m.mu.Unlock()
}
