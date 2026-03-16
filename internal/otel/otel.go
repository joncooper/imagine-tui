// Package otel provides opt-in OpenTelemetry tracing for imagine-tui.
// When OTEL_EXPORTER_OTLP_ENDPOINT is set, traces are exported via OTLP/HTTP.
// When unset, all operations are no-ops with zero overhead.
package otel

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var enabled bool

// Init initializes the OTEL trace pipeline. If OTEL_EXPORTER_OTLP_ENDPOINT is
// unset, it returns a no-op shutdown and Enabled() will return false.
func Init(ctx context.Context, serviceName, serviceVersion string) (shutdown func(context.Context) error, err error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracehttp.New(ctx) // reads OTEL_EXPORTER_* env vars
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	enabled = true

	return tp.Shutdown, nil
}

// Tracer returns the package-level tracer. Safe to call before Init (returns
// the global no-op tracer).
func Tracer() trace.Tracer {
	return otel.Tracer("imagine-tui")
}

// Enabled reports whether tracing was activated by Init.
func Enabled() bool {
	return enabled
}
