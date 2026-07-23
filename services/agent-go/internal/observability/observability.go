// Package observability cung cấp logging có cấu trúc (slog) và tracing
// (OpenTelemetry) cho agent-go.
//
// Phase này exporter còn tối giản: SetupTracer chỉ đăng ký một TracerProvider
// "rỗng" (no-op, không phát telemetry) làm global provider. Mục tiêu là compile
// được và không panic; việc cắm sdktrace.TracerProvider + resource(serviceName)
// + exporter thật (stdouttrace/OTLP) để dành cho phase sau.
//
// Ghi chú kỹ thuật: hiện chưa dùng go.opentelemetry.io/otel/sdk/trace vì package
// đó kéo theo go.opentelemetry.io/otel/sdk/resource -> github.com/google/uuid,
// mà go.sum của service chưa có checksum cho uuid v1.6.0. Khi go.sum được bổ
// sung (phase sau), chỉ cần thay noop.NewTracerProvider() bằng
// sdktrace.NewTracerProvider(sdktrace.WithResource(res...)).
package observability

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// SetupLogger dựng một *slog.Logger ghi JSON ra stdout và đặt nó làm logger
// mặc định của slog (slog.SetDefault). Trả về logger để caller dùng trực tiếp.
func SetupLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// SetupTracer đăng ký một global TracerProvider và trả về hàm shutdown.
//
// Phase này dùng noop.TracerProvider (provider rỗng, không phát telemetry) nên
// serviceName chỉ được ghi log để chuẩn bị cho phase gắn resource thật. Hàm
// trả về không lỗi và shutdown là no-op an toàn để gọi khi thoát.
func SetupTracer(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	_ = ctx // dành cho phase sau khi dựng resource.New(ctx, ...).

	tp := noop.NewTracerProvider()
	otel.SetTracerProvider(tp)

	slog.Default().Info("tracer initialized (noop provider, exporter phase sau)",
		slog.String("service.name", serviceName),
	)

	// noop.TracerProvider không có Shutdown; trả về no-op để caller dùng thống
	// nhất với sdktrace.TracerProvider.Shutdown ở phase sau.
	shutdown = func(context.Context) error { return nil }
	return shutdown, nil
}

// Tracer trả về một trace.Tracer có tên name từ global TracerProvider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
