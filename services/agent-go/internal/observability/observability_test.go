package observability

import (
	"context"
	"testing"
)

func TestSetupLogger(t *testing.T) {
	logger := SetupLogger()
	if logger == nil {
		t.Fatal("SetupLogger() trả về nil, mong đợi *slog.Logger non-nil")
	}
}

// TestSetupTracer verifies the real TracerProvider (stdout exporter path,
// khi KHÔNG có OTEL_EXPORTER_OTLP_ENDPOINT) khởi tạo không panic, trả về
// provider non-nil dùng được, và shutdown sạch — không test integration với
// OTLP thật (không có hạ tầng ngoài trong CI).
func TestSetupTracer(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	ctx := context.Background()

	shutdown, err := SetupTracer(ctx, "agent-go-test")
	if err != nil {
		t.Fatalf("SetupTracer() lỗi: %v", err)
	}
	if shutdown == nil {
		t.Fatal("SetupTracer() trả về shutdown nil")
	}

	// Tracer từ global provider phải dùng được, không panic.
	tr := Tracer("test")
	if tr == nil {
		t.Fatal("Tracer() trả về nil")
	}
	_, span := tr.Start(ctx, "test-span")
	span.End()

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() lỗi: %v", err)
	}
}
