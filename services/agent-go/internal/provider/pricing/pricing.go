// Package pricing cung cấp bảng giá ƯỚC TÍNH (USD / 1M token) cho các model
// LLM mà agent-go dùng, phục vụ cost ledger per-tenant (xem
// internal/storage/sqlite/cost_ledger.go, internal/agent.CostLedger).
//
// QUAN TRỌNG — bảng giá trong file này là ƯỚC TÍNH, KHÔNG PHẢI sự thật tuyệt
// đối: giá LLM thay đổi liên tục và code này không có cách gọi API để xác
// minh giá hiện tại chính xác 100%. Số liệu dưới đây dựa trên mặt bằng giá
// công khai phổ biến của Google (Gemini), Anthropic (Claude) và DeepSeek tại
// thời điểm viết code — verify lại trang pricing chính thức của từng nhà
// cung cấp trước khi dùng số $ tuyệt đối cho quyết định billing/tài chính:
//   - https://ai.google.dev/pricing
//   - https://www.anthropic.com/pricing
//   - https://platform.deepseek.com/api-docs/pricing
//
// Để tránh giá sai lệch trở thành "sự thật cứng", bảng giá có thể override
// qua biến môi trường PRICING_OVERRIDE_JSON (đường dẫn tới 1 file JSON dạng
// map["provider:model"]ModelPricing) — xem LoadOverrideFile.
package pricing

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// ModelPricing là giá ước tính USD trên mỗi 1 triệu token cho 1 model.
type ModelPricing struct {
	InputPer1M  float64 `json:"inputPer1M"`
	OutputPer1M float64 `json:"outputPer1M"`
}

// defaultPricing là bảng giá ƯỚC TÍNH mặc định, key dạng "provider:model"
// khớp với provider.Name() (gemini/anthropic/deepseek/ollama/openai_compat)
// và tên model default/fallback khai báo trong internal/config/config.go.
//
// giá ước tính, cần verify lại trang pricing chính thức trước khi tin tưởng
// số $ tuyệt đối — xem comment package ở trên.
var defaultPricing = map[string]ModelPricing{
	// --- Gemini (Google) — flash-lite/flash tier, rẻ nhất trong dòng Gemini.
	// giá ước tính, cần verify lại trang pricing chính thức trước khi tin tưởng số $ tuyệt đối
	"gemini:gemini-3.1-flash-lite": {InputPer1M: 0.075, OutputPer1M: 0.30},
	"gemini:gemini-3.5-flash-lite": {InputPer1M: 0.075, OutputPer1M: 0.30},
	"gemini:gemini-3.7-flash":      {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gemini:gemini-3.6-flash":      {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gemini:gemini-3.5-flash":      {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gemini:gemini-2.5-flash-lite": {InputPer1M: 0.10, OutputPer1M: 0.40},
	"gemini:gemini-2.5-flash":      {InputPer1M: 0.30, OutputPer1M: 2.50},

	// --- Anthropic (Claude) — thường là provider ĐẮT NHẤT trong 3 bên, dùng
	// làm "hypothetical max" mặc định (xem MaxPricing). giá ước tính, cần
	// verify lại trang pricing chính thức trước khi tin tưởng số $ tuyệt đối.
	"anthropic:claude-haiku-4-5-20251001": {InputPer1M: 1.00, OutputPer1M: 5.00},

	// --- DeepSeek — rẻ nhất, dùng cho fastModel (tác vụ phụ trợ). giá ước
	// tính, cần verify lại trang pricing chính thức trước khi tin tưởng số $
	// tuyệt đối.
	"deepseek:deepseek-v4-flash": {InputPer1M: 0.14, OutputPer1M: 0.28},
	"deepseek:deepseek-v4-pro":   {InputPer1M: 0.55, OutputPer1M: 2.19},

	// --- Self-hosted / local — không tính phí theo token (chi phí hạ tầng
	// vận hành máy chủ không nằm trong phạm vi cost ledger này).
	"ollama:*":        {InputPer1M: 0, OutputPer1M: 0},
	"openai_compat:*": {InputPer1M: 0, OutputPer1M: 0},
}

var (
	mu    sync.RWMutex
	table = cloneTable(defaultPricing)
)

func cloneTable(src map[string]ModelPricing) map[string]ModelPricing {
	out := make(map[string]ModelPricing, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func init() {
	// Best-effort: nếu PRICING_OVERRIDE_JSON được cấu hình nhưng file lỗi/không
	// đọc được, KHÔNG panic — chạy tiếp với bảng giá mặc định, chỉ log warn.
	if path := os.Getenv("PRICING_OVERRIDE_JSON"); path != "" {
		if err := LoadOverrideFile(path); err != nil {
			slog.Warn("pricing: không nạp được PRICING_OVERRIDE_JSON, dùng bảng giá ước tính mặc định", "path", path, "err", err)
		} else {
			slog.Info("pricing: đã nạp override giá từ file", "path", path)
		}
	}
}

// LoadOverrideFile đọc 1 file JSON dạng map["provider:model"]ModelPricing và
// MERGE đè lên bảng giá hiện tại (entry nào không có trong file vẫn giữ giá
// mặc định — không xoá gì). Cho phép operator sửa giá mà không cần build lại
// binary khi giá nhà cung cấp thay đổi.
func LoadOverrideFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("pricing: đọc file override: %w", err)
	}
	var overrides map[string]ModelPricing
	if err := json.Unmarshal(data, &overrides); err != nil {
		return fmt.Errorf("pricing: parse JSON override: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for k, v := range overrides {
		table[k] = v
	}
	return nil
}

// ResetForTest khôi phục bảng giá về mặc định (bỏ mọi override đã nạp qua
// LoadOverrideFile) — CHỈ dùng trong test để cô lập giữa các test case.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	table = cloneTable(defaultPricing)
}

// Lookup trả về ModelPricing cho "provider:model". Nếu không có entry riêng
// cho model đó, thử fallback "provider:*" (dùng cho provider tự host không
// phân biệt model, vd ollama). ok=false nếu không tìm thấy gì.
func Lookup(providerName, model string) (ModelPricing, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if p, ok := table[providerName+":"+model]; ok {
		return p, true
	}
	if p, ok := table[providerName+":*"]; ok {
		return p, true
	}
	return ModelPricing{}, false
}

// MaxPricing trả về giá INPUT cao nhất và giá OUTPUT cao nhất (ĐỘC LẬP, có
// thể không cùng 1 model) qua toàn bộ bảng giá hiện tại — đại diện cho
// "provider/model đắt nhất có thể" dùng để tính hypothetical_max_cost_usd
// (xem Calculate). Thường rơi vào Claude/Anthropic nhưng không hardcode tên
// provider — để bảng giá override tự quyết định "đắt nhất" là gì.
func MaxPricing() ModelPricing {
	mu.RLock()
	defer mu.RUnlock()
	var maxIn, maxOut float64
	for _, p := range table {
		if p.InputPer1M > maxIn {
			maxIn = p.InputPer1M
		}
		if p.OutputPer1M > maxOut {
			maxOut = p.OutputPer1M
		}
	}
	return ModelPricing{InputPer1M: maxIn, OutputPer1M: maxOut}
}

// Calculate tính chi phí ước tính (USD) THẬT SỰ đã dùng (theo provider/model
// cụ thể) và chi phí GIẢ ĐỊNH nếu lượt gọi đó dùng provider/model ĐẮT NHẤT
// hiện có trong bảng giá (MaxPricing) — hiệu số giữa 2 giá trị này là phần
// "tiết kiệm" nhờ dùng provider rẻ hơn (xem sqlite.CostSummary.SavingsUSD).
//
// Nếu không tìm thấy giá cho provider/model (chưa cấu hình/override thiếu),
// actualUSD = 0 thay vì đoán mò — tránh báo sai một khoản tiền dương không có
// căn cứ. hypotheticalMaxUSD vẫn tính bình thường.
func Calculate(providerName, model string, inputTokens, outputTokens int) (actualUSD, hypotheticalMaxUSD float64) {
	p, _ := Lookup(providerName, model)
	actualUSD = cost(p, inputTokens, outputTokens)
	hypotheticalMaxUSD = cost(MaxPricing(), inputTokens, outputTokens)
	return actualUSD, hypotheticalMaxUSD
}

func cost(p ModelPricing, inputTokens, outputTokens int) float64 {
	return float64(inputTokens)/1_000_000*p.InputPer1M + float64(outputTokens)/1_000_000*p.OutputPer1M
}
