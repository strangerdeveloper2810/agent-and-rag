package pricing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCalculate_UsesDefaultTable(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	// gemini:gemini-3.1-flash-lite = {0.075, 0.30} theo bảng mặc định.
	actual, hypo := Calculate("gemini", "gemini-3.1-flash-lite", 1_000_000, 1_000_000)

	const wantActual = 0.075 + 0.30
	if diff := actual - wantActual; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("actualUSD = %v, want %v", actual, wantActual)
	}

	// hypothetical phải dùng giá cao nhất trong bảng (anthropic 1.00/5.00 theo default).
	const wantHypoMin = 1.00 + 5.00
	if hypo < wantHypoMin-1e-9 {
		t.Errorf("hypotheticalMaxUSD = %v, muốn ít nhất %v (giá đắt nhất mặc định)", hypo, wantHypoMin)
	}
}

func TestCalculate_UnknownModel_ActualZero(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	actual, hypo := Calculate("gemini", "model-khong-ton-tai", 1000, 1000)
	if actual != 0 {
		t.Errorf("actualUSD = %v, want 0 khi không có giá cho model", actual)
	}
	if hypo <= 0 {
		t.Errorf("hypotheticalMaxUSD = %v, muốn > 0 (vẫn dùng giá đắt nhất bảng)", hypo)
	}
}

func TestLookup_FallbackWildcard(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	p, ok := Lookup("ollama", "llama3.1:8b")
	if !ok {
		t.Fatal("Lookup(ollama, llama3.1:8b) không tìm thấy — muốn fallback ollama:*")
	}
	if p.InputPer1M != 0 || p.OutputPer1M != 0 {
		t.Errorf("ollama pricing = %+v, want {0,0} (self-hosted, miễn phí)", p)
	}
}

func TestMaxPricing_IsMaxAcrossTable(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	max := MaxPricing()
	for key, p := range table {
		if p.InputPer1M > max.InputPer1M {
			t.Errorf("MaxPricing().InputPer1M = %v < %v (entry %s)", max.InputPer1M, p.InputPer1M, key)
		}
		if p.OutputPer1M > max.OutputPer1M {
			t.Errorf("MaxPricing().OutputPer1M = %v < %v (entry %s)", max.OutputPer1M, p.OutputPer1M, key)
		}
	}
}

func TestLoadOverrideFile_MergesOverDefaults(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	dir := t.TempDir()
	path := filepath.Join(dir, "pricing.json")

	overrides := map[string]ModelPricing{
		"gemini:gemini-3.1-flash-lite": {InputPer1M: 9.99, OutputPer1M: 19.99},
		"custom:my-model":              {InputPer1M: 1.23, OutputPer1M: 4.56},
	}
	data, err := json.Marshal(overrides)
	if err != nil {
		t.Fatalf("marshal overrides: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write override file: %v", err)
	}

	if err := LoadOverrideFile(path); err != nil {
		t.Fatalf("LoadOverrideFile: %v", err)
	}

	p, ok := Lookup("gemini", "gemini-3.1-flash-lite")
	if !ok || p.InputPer1M != 9.99 || p.OutputPer1M != 19.99 {
		t.Errorf("override không áp dụng: got %+v, ok=%v", p, ok)
	}

	p2, ok2 := Lookup("custom", "my-model")
	if !ok2 || p2.InputPer1M != 1.23 {
		t.Errorf("override entry mới không được thêm vào bảng: got %+v, ok=%v", p2, ok2)
	}

	// Entry mặc định KHÔNG có trong file override vẫn phải còn nguyên.
	if _, ok := Lookup("deepseek", "deepseek-v4-flash"); !ok {
		t.Error("entry mặc định deepseek:deepseek-v4-flash bị mất sau khi override — LoadOverrideFile phải MERGE chứ không thay thế")
	}
}

func TestLoadOverrideFile_InvalidPath(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	if err := LoadOverrideFile("/duong/dan/khong/ton/tai.json"); err == nil {
		t.Error("LoadOverrideFile với path không tồn tại phải trả lỗi")
	}
}
