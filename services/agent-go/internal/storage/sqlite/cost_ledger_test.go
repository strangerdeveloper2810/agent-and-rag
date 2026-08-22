package sqlite

import (
	"context"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
)

func TestRecordCost_InsertsRow(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	err := st.RecordCost(ctx, agent.CostEntry{
		TenantID:               "tenant-a",
		Provider:               "gemini",
		Model:                  "gemini-3.1-flash-lite",
		InputTokens:            1000,
		OutputTokens:           200,
		CostUSD:                0.001,
		HypotheticalMaxCostUSD: 0.01,
	})
	if err != nil {
		t.Fatalf("RecordCost: %v", err)
	}

	summary, err := st.TenantCostSummary(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("TenantCostSummary: %v", err)
	}
	if summary.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", summary.RequestCount)
	}
	if summary.TotalInputTokens != 1000 || summary.TotalOutputTokens != 200 {
		t.Errorf("tokens = (%d, %d), want (1000, 200)", summary.TotalInputTokens, summary.TotalOutputTokens)
	}
	if summary.TotalCostUSD != 0.001 {
		t.Errorf("TotalCostUSD = %v, want 0.001", summary.TotalCostUSD)
	}
	if summary.TotalHypotheticalCostUSD != 0.01 {
		t.Errorf("TotalHypotheticalCostUSD = %v, want 0.01", summary.TotalHypotheticalCostUSD)
	}
}

func TestTenantCostSummary_SavingsUSD(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	entries := []agent.CostEntry{
		{TenantID: "tenant-b", Provider: "deepseek", Model: "deepseek-v4-flash", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.0001, HypotheticalMaxCostUSD: 0.003},
		{TenantID: "tenant-b", Provider: "gemini", Model: "gemini-3.1-flash-lite", InputTokens: 2000, OutputTokens: 800, CostUSD: 0.0004, HypotheticalMaxCostUSD: 0.006},
	}
	for _, e := range entries {
		if err := st.RecordCost(ctx, e); err != nil {
			t.Fatalf("RecordCost: %v", err)
		}
	}

	summary, err := st.TenantCostSummary(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("TenantCostSummary: %v", err)
	}

	const wantCost = 0.0001 + 0.0004
	const wantHypo = 0.003 + 0.006
	const wantSavings = wantHypo - wantCost

	if diff := summary.TotalCostUSD - wantCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("TotalCostUSD = %v, want %v", summary.TotalCostUSD, wantCost)
	}
	if diff := summary.TotalHypotheticalCostUSD - wantHypo; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("TotalHypotheticalCostUSD = %v, want %v", summary.TotalHypotheticalCostUSD, wantHypo)
	}
	if diff := summary.SavingsUSD - wantSavings; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("SavingsUSD = %v, want %v", summary.SavingsUSD, wantSavings)
	}
	if summary.RequestCount != 2 {
		t.Errorf("RequestCount = %d, want 2", summary.RequestCount)
	}

	if len(summary.ByProvider) != 2 {
		t.Fatalf("ByProvider len = %d, want 2 (deepseek, gemini): %+v", len(summary.ByProvider), summary.ByProvider)
	}
	byProv := map[string]ProviderCostBreakdown{}
	for _, b := range summary.ByProvider {
		byProv[b.Provider] = b
	}
	if b, ok := byProv["deepseek"]; !ok || b.RequestCount != 1 || b.InputTokens != 1000 {
		t.Errorf("deepseek breakdown = %+v, ok=%v", b, ok)
	}
	if b, ok := byProv["gemini"]; !ok || b.RequestCount != 1 || b.InputTokens != 2000 {
		t.Errorf("gemini breakdown = %+v, ok=%v", b, ok)
	}
}

func TestTenantCostSummary_UnknownTenant_ReturnsZeroNotError(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	summary, err := st.TenantCostSummary(ctx, "tenant-khong-ton-tai")
	if err != nil {
		t.Fatalf("TenantCostSummary: %v", err)
	}
	if summary.RequestCount != 0 || summary.TotalCostUSD != 0 || summary.SavingsUSD != 0 {
		t.Errorf("summary = %+v, want all-zero for unknown tenant", summary)
	}
}

// TestTenantCostSummary_TenantIsolation đảm bảo chi phí của tenant này KHÔNG
// lẫn vào tổng của tenant khác — cost ledger PHẢI cô lập theo tenant, đúng
// tinh thần multi-tenant của toàn bộ JARVIS.
func TestTenantCostSummary_TenantIsolation(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	if err := st.RecordCost(ctx, agent.CostEntry{TenantID: "tenant-x", Provider: "gemini", InputTokens: 100, OutputTokens: 10, CostUSD: 0.01, HypotheticalMaxCostUSD: 0.05}); err != nil {
		t.Fatalf("RecordCost tenant-x: %v", err)
	}
	if err := st.RecordCost(ctx, agent.CostEntry{TenantID: "tenant-y", Provider: "gemini", InputTokens: 999, OutputTokens: 999, CostUSD: 99, HypotheticalMaxCostUSD: 999}); err != nil {
		t.Fatalf("RecordCost tenant-y: %v", err)
	}

	summaryX, err := st.TenantCostSummary(ctx, "tenant-x")
	if err != nil {
		t.Fatalf("TenantCostSummary tenant-x: %v", err)
	}
	if summaryX.RequestCount != 1 || summaryX.TotalCostUSD != 0.01 {
		t.Errorf("tenant-x summary = %+v, want RequestCount=1 TotalCostUSD=0.01 (không lẫn tenant-y)", summaryX)
	}
}
