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

func TestSetupTracer(t *testing.T) {
	ctx := context.Background()

	shutdown, err := SetupTracer(ctx, "agent-go-test")
	if err != nil {
		t.Fatalf("SetupTracer() lỗi: %v", err)
	}
	if shutdown == nil {
		t.Fatal("SetupTracer() trả về shutdown nil")
	}

	// Tracer từ global provider phải dùng được, không panic.
	if tr := Tracer("test"); tr == nil {
		t.Fatal("Tracer() trả về nil")
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() lỗi: %v", err)
	}
}
