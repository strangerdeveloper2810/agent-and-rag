package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// fakeCostLedger ghi lại mọi lời gọi RecordCost — dùng để kiểm tra Engine gọi
// đúng ledger với usage đã biết trước, không cần SQLite thật.
type fakeCostLedger struct {
	mu      sync.Mutex
	entries []CostEntry
	err     error // nếu set, RecordCost trả lỗi này (test fail-safe)
}

func (f *fakeCostLedger) RecordCost(_ context.Context, entry CostEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry)
	return f.err
}

func (f *fakeCostLedger) recorded() []CostEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]CostEntry, len(f.entries))
	copy(out, f.entries)
	return out
}

// namedFakeProvider bọc provider.FakeProvider để có Name() tuỳ chỉnh (khác
// "fake" cố định) và implement Model() string — engine.recordCost dùng type
// assertion interface{ Model() string } để lấy tên model, giống pattern đã
// có trong internal/provider/factory (namedOverride.Model()).
type namedFakeProvider struct {
	*provider.FakeProvider
	name  string
	model string
}

func (p *namedFakeProvider) Name() string  { return p.name }
func (p *namedFakeProvider) Model() string { return p.model }

func TestEngine_SetCostLedger_RecordsCostAfterRun(t *testing.T) {
	prov := &namedFakeProvider{
		FakeProvider: provider.NewFake(
			provider.StreamChunk{Kind: provider.ChunkText, Text: "xin chao"},
			provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 1234, OutputTokens: 56}},
			provider.StreamChunk{Kind: provider.ChunkDone},
		),
		name:  "gemini",
		model: "gemini-3.1-flash-lite",
	}

	eng := NewEngine(prov, tools.NewRegistry())
	ledger := &fakeCostLedger{}
	eng.SetCostLedger(ledger)

	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-test")

	_, err := eng.Run(ctx, RunInput{UserMessage: "chao", MaxSteps: 5}, func(Event) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries := ledger.recorded()
	if len(entries) != 1 {
		t.Fatalf("RecordCost calls = %d, want 1: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.TenantID != "tenant-test" {
		t.Errorf("TenantID = %q, want %q", e.TenantID, "tenant-test")
	}
	if e.Provider != "gemini" {
		t.Errorf("Provider = %q, want %q", e.Provider, "gemini")
	}
	if e.Model != "gemini-3.1-flash-lite" {
		t.Errorf("Model = %q, want %q", e.Model, "gemini-3.1-flash-lite")
	}
	if e.InputTokens != 1234 || e.OutputTokens != 56 {
		t.Errorf("tokens = (%d, %d), want (1234, 56)", e.InputTokens, e.OutputTokens)
	}
	if e.CostUSD < 0 {
		t.Errorf("CostUSD = %v, want >= 0", e.CostUSD)
	}
	if e.HypotheticalMaxCostUSD < e.CostUSD {
		t.Errorf("HypotheticalMaxCostUSD (%v) < CostUSD (%v) — hypothetical phải luôn >= actual", e.HypotheticalMaxCostUSD, e.CostUSD)
	}
}

// TestEngine_NilCostLedger_DoesNotPanic đảm bảo tính năng TẮT mặc định (nil)
// không ảnh hưởng gì tới hành vi engine bình thường.
func TestEngine_NilCostLedger_DoesNotPanic(t *testing.T) {
	prov := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 10, OutputTokens: 2}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := NewEngine(prov, tools.NewRegistry())
	// Không gọi SetCostLedger — costLedger giữ nil.

	if _, err := eng.Run(context.Background(), RunInput{UserMessage: "hi", MaxSteps: 5}, func(Event) {}); err != nil {
		t.Fatalf("Run với costLedger=nil lỗi: %v", err)
	}
}

// TestEngine_CostLedgerError_DoesNotFailRun đảm bảo lỗi ghi cost ledger CHỈ
// log warn, KHÔNG làm Run() trả lỗi hay chặn response user — đúng triết lý
// fail-safe giống SetInterruptStore.
func TestEngine_CostLedgerError_DoesNotFailRun(t *testing.T) {
	prov := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 10, OutputTokens: 2}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := NewEngine(prov, tools.NewRegistry())
	ledger := &fakeCostLedger{err: errors.New("boom: disk full")}
	eng.SetCostLedger(ledger)

	if _, err := eng.Run(context.Background(), RunInput{UserMessage: "hi", MaxSteps: 5}, func(Event) {}); err != nil {
		t.Fatalf("Run phải thành công dù RecordCost lỗi, got: %v", err)
	}
	if len(ledger.recorded()) != 1 {
		t.Fatalf("RecordCost vẫn phải được GỌI (dù trả lỗi) — got %d calls", len(ledger.recorded()))
	}
}
