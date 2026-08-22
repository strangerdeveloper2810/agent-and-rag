package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/agent"
)

// migrateCostLedger tạo bảng cost_ledger (nếu chưa có) — bảng lưu chi phí ước
// tính (USD) mỗi lượt chạy engine, gắn theo tenant (xem agent.CostLedger,
// internal/provider/pricing).
//
// KHÔNG gọi từ Open() (sqlite.go) như migrate()/migratePausedRuns() — cố
// tình để KHÔNG phải sửa sqlite.go/paused_runs.go (2 file lõi đã ổn định).
// Thay vào đó gọi LAZY ngay trước mỗi lần đọc/ghi (RecordCost,
// TenantCostSummary) — CREATE TABLE IF NOT EXISTS là idempotent nên gọi lặp
// lại vô hại, chi phí thêm không đáng kể so với 1 lần INSERT/SELECT.
func migrateCostLedger(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS cost_ledger (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		hypothetical_max_cost_usd REAL NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_cost_ledger_tenant ON cost_ledger(tenant_id);
	`)
	if err != nil {
		return fmt.Errorf("sqlite: migrate cost_ledger: %w", err)
	}
	return nil
}

// RecordCost ghi 1 dòng chi phí ước tính vào bảng cost_ledger — implements
// agent.CostLedger (xem SetCostLedger trong internal/agent/engine.go).
//
// Insert PHẢI fail-safe: hàm TRẢ VỀ error nếu ghi thất bại, nhưng caller
// (Engine.recordCost) chỉ log warn — KHÔNG BAO GIỜ chặn response user, đúng
// triết lý đã áp dụng cho SaveInterruptedState (paused_runs.go).
func (s *Store) RecordCost(ctx context.Context, entry agent.CostEntry) error {
	if err := migrateCostLedger(s.db); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cost_ledger (tenant_id, provider, model, input_tokens, output_tokens, cost_usd, hypothetical_max_cost_usd)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.TenantID, entry.Provider, entry.Model, entry.InputTokens, entry.OutputTokens, entry.CostUSD, entry.HypotheticalMaxCostUSD,
	)
	if err != nil {
		return fmt.Errorf("sqlite: record cost: %w", err)
	}
	return nil
}

// ProviderCostBreakdown là chi phí cộng dồn theo 1 provider cho 1 tenant —
// xem CostSummary.ByProvider.
type ProviderCostBreakdown struct {
	Provider     string  `json:"provider"`
	RequestCount int     `json:"requestCount"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CostUSD      float64 `json:"costUsd"`
}

// CostSummary là tổng chi phí ước tính của 1 tenant, trả về bởi
// TenantCostSummary.
type CostSummary struct {
	TenantID                 string  `json:"tenantId"`
	TotalCostUSD             float64 `json:"totalCostUsd"`
	TotalHypotheticalCostUSD float64 `json:"totalHypotheticalCostUsd"`
	// SavingsUSD = TotalHypotheticalCostUSD - TotalCostUSD — ước tính số tiền
	// tiết kiệm được nhờ KHÔNG luôn dùng provider/model đắt nhất
	// (pricing.MaxPricing) cho mọi lượt gọi. Có thể âm về mặt lý thuyết nếu
	// bảng giá bị override sai (model thực tế đắt hơn "max" ghi nhận tại thời
	// điểm gọi) — cố tình KHÔNG ép về 0 để không che giấu dữ liệu bất thường.
	SavingsUSD        float64                 `json:"savingsUsd"`
	TotalInputTokens  int                     `json:"totalInputTokens"`
	TotalOutputTokens int                     `json:"totalOutputTokens"`
	RequestCount      int                     `json:"requestCount"`
	ByProvider        []ProviderCostBreakdown `json:"byProvider,omitempty"`
}

// TenantCostSummary tính tổng chi phí ước tính (+ tiết kiệm ước tính so với
// luôn dùng provider/model đắt nhất) của 1 tenant, cùng breakdown theo
// provider. tenantID không tồn tại (chưa có bản ghi nào) → trả về CostSummary
// rỗng (mọi field số = 0), KHÔNG lỗi.
func (s *Store) TenantCostSummary(ctx context.Context, tenantID string) (CostSummary, error) {
	if err := migrateCostLedger(s.db); err != nil {
		return CostSummary{}, err
	}

	summary := CostSummary{TenantID: tenantID}
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0), COALESCE(SUM(hypothetical_max_cost_usd), 0),
		        COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COUNT(*)
		 FROM cost_ledger WHERE tenant_id = ?`, tenantID,
	).Scan(&summary.TotalCostUSD, &summary.TotalHypotheticalCostUSD, &summary.TotalInputTokens, &summary.TotalOutputTokens, &summary.RequestCount)
	if err != nil {
		return CostSummary{}, fmt.Errorf("sqlite: tenant cost summary: %w", err)
	}
	summary.SavingsUSD = summary.TotalHypotheticalCostUSD - summary.TotalCostUSD

	rows, err := s.db.QueryContext(ctx,
		`SELECT provider, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(cost_usd), 0)
		 FROM cost_ledger WHERE tenant_id = ? GROUP BY provider ORDER BY provider`, tenantID,
	)
	if err != nil {
		return CostSummary{}, fmt.Errorf("sqlite: tenant cost breakdown: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b ProviderCostBreakdown
		if err := rows.Scan(&b.Provider, &b.RequestCount, &b.InputTokens, &b.OutputTokens, &b.CostUSD); err != nil {
			return CostSummary{}, fmt.Errorf("sqlite: scan cost breakdown: %w", err)
		}
		summary.ByProvider = append(summary.ByProvider, b)
	}
	if err := rows.Err(); err != nil {
		return CostSummary{}, fmt.Errorf("sqlite: cost breakdown rows: %w", err)
	}

	return summary, nil
}
