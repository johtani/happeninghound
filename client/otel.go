package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitTracer OpenTelemetry の Tracer を初期化します
func InitTracer(ctx context.Context, w io.Writer) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	otelExporter := os.Getenv("OTEL_EXPORTER")
	switch otelExporter {
	case "otlp":
		proto := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
		if proto == "http/protobuf" {
			exporter, err = otlptracehttp.New(ctx)
		} else {
			// デフォルトは gRPC
			exporter, err = otlptracegrpc.New(ctx)
		}
	default:
		exporter, err = stdouttrace.New(
			stdouttrace.WithWriter(w),
			stdouttrace.WithPrettyPrint(),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("happeninghound"),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}

func ShutdownTracer(tp *sdktrace.TracerProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down tracer provider: %v\n", err)
	}
}
