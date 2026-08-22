// Package observability cung cấp logging có cấu trúc (slog) và tracing
// (OpenTelemetry) cho agent-go.
//
// SetupTracer dựng một TracerProvider THẬT (go.opentelemetry.io/otel/sdk/trace)
// gắn resource service.name. Exporter được chọn theo env:
//   - OTEL_EXPORTER_OTLP_ENDPOINT rỗng (mặc định)  → stdouttrace, in span dạng
//     JSON ra stdout — không cần hạ tầng ngoài, phù hợp dev/local.
//   - OTEL_EXPORTER_OTLP_ENDPOINT có set             → OTLP HTTP exporter, gửi
//     span tới endpoint đó (otlptracehttp tự đọc biến env chuẩn OTel).
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
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

// SetupTracer dựng một sdktrace.TracerProvider THẬT, đăng ký làm global
// TracerProvider (otel.SetTracerProvider) và trả về hàm shutdown để caller
// defer khi thoát (flush span còn lại trong buffer).
//
// Exporter: OTEL_EXPORTER_OTLP_ENDPOINT rỗng → stdouttrace (in span ra
// stdout); có set → OTLP HTTP exporter trỏ tới endpoint đó.
func SetupTracer(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: build resource: %w", err)
	}

	var (
		sp         sdktrace.SpanProcessor
		exporterID string
	)
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		// otlptracehttp tự đọc OTEL_EXPORTER_OTLP_ENDPOINT (và các biến env
		// chuẩn OTel khác như headers/insecure) — không cần truyền lại thủ công.
		exp, expErr := otlptracehttp.New(ctx)
		if expErr != nil {
			return nil, fmt.Errorf("observability: create otlp exporter: %w", expErr)
		}
		sp = sdktrace.NewBatchSpanProcessor(exp)
		exporterID = "otlp"
		slog.Default().Info("tracer initialized", "exporter", exporterID, "endpoint", endpoint, "service.name", serviceName)
	} else {
		exp, expErr := stdouttrace.New(stdouttrace.WithoutTimestamps())
		if expErr != nil {
			return nil, fmt.Errorf("observability: create stdout exporter: %w", expErr)
		}
		sp = sdktrace.NewBatchSpanProcessor(exp)
		exporterID = "stdout"
		slog.Default().Info("tracer initialized", "exporter", exporterID, "service.name", serviceName)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// Tracer trả về một trace.Tracer có tên name từ global TracerProvider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
