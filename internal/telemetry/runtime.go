package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

// Config controls process-level telemetry setup for imagine-tui.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Command        string
	LogPath        string
}

// Runtime owns the process-wide logger and tracer setup for imagine-tui.
type Runtime struct {
	Logger         *slog.Logger
	tracerProvider trace.TracerProvider
	shutdown       func(context.Context) error
}

// New configures file logging plus optional OTLP trace and log exporters.
func New(ctx context.Context, cfg Config) (*Runtime, error) {
	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	traceProvider := trace.TracerProvider(nooptrace.NewTracerProvider())
	traceShutdown := func(context.Context) error { return nil }
	if tracesEnabled() {
		traceExporter, err := newTraceExporter(ctx)
		if err != nil {
			return nil, err
		}
		tp := oteltrace.NewTracerProvider(
			oteltrace.WithBatcher(traceExporter),
			oteltrace.WithResource(res),
			oteltrace.WithSampler(oteltrace.AlwaysSample()),
		)
		traceProvider = tp
		traceShutdown = tp.Shutdown
	}

	var handlers []slog.Handler
	var closeLogFile func() error
	if cfg.LogPath != "" {
		fileHandler, closeFn, err := newFileHandler(cfg.LogPath)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, fileHandler)
		closeLogFile = closeFn
	}

	logProviderShutdown := func(context.Context) error { return nil }
	if logsEnabled() {
		logExporter, err := newLogExporter(ctx)
		if err != nil {
			return nil, err
		}
		lp := log.NewLoggerProvider(
			log.WithProcessor(log.NewBatchProcessor(logExporter)),
			log.WithResource(res),
		)
		logHandler := otelslog.NewHandler(
			cfg.ServiceName,
			otelslog.WithLoggerProvider(lp),
			otelslog.WithSource(true),
			otelslog.WithVersion(cfg.ServiceVersion),
			otelslog.WithAttributes(
				attribute.String(AttrCommand, cfg.Command),
			),
		)
		handlers = append(handlers, logHandler)
		logProviderShutdown = lp.Shutdown
	}

	if len(handlers) == 0 {
		handlers = append(handlers, slog.NewTextHandler(discardWriter{}, nil))
	}

	logger := slog.New(&fanoutHandler{handlers: handlers})
	logger = logger.With(
		slog.String(AttrCommand, cfg.Command),
		slog.String(string(semconv.ServiceNameKey), cfg.ServiceName),
	)

	return &Runtime{
		Logger:         logger,
		tracerProvider: traceProvider,
		shutdown: func(ctx context.Context) error {
			var err error
			err = errors.Join(err, logProviderShutdown(ctx))
			err = errors.Join(err, traceShutdown(ctx))
			if closeLogFile != nil {
				err = errors.Join(err, closeLogFile())
			}
			return err
		},
	}, nil
}

// Shutdown flushes and stops any active OTel providers.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil || r.shutdown == nil {
		return nil
	}
	return r.shutdown(ctx)
}

// Tracer returns a named tracer from the runtime tracer provider.
func (r *Runtime) Tracer(name string) trace.Tracer {
	if r == nil || r.tracerProvider == nil {
		return nooptrace.NewTracerProvider().Tracer(name)
	}
	return r.tracerProvider.Tracer(name)
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	return resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String(AttrCommand, cfg.Command),
		),
	)
}

func newFileHandler(path string) (slog.Handler, func() error, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	return slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}), f.Close, nil
}

func newTraceExporter(ctx context.Context) (oteltrace.SpanExporter, error) {
	switch exporterProtocol("traces") {
	case "grpc":
		return otlptracegrpc.New(ctx)
	default:
		return otlptracehttp.New(ctx)
	}
}

func newLogExporter(ctx context.Context) (log.Exporter, error) {
	switch exporterProtocol("logs") {
	case "grpc":
		return otlploggrpc.New(ctx)
	default:
		return otlploghttp.New(ctx)
	}
}

func tracesEnabled() bool {
	return otlpEnabled() && hasAnyEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
}

func logsEnabled() bool {
	return otlpEnabled() && hasAnyEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT")
}

func otlpEnabled() bool {
	if disabled := strings.TrimSpace(strings.ToLower(os.Getenv("OTEL_SDK_DISABLED"))); disabled == "true" || disabled == "1" {
		return false
	}
	return hasAnyEnv(
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
	)
}

func exporterProtocol(signal string) string {
	var keys []string
	switch signal {
	case "traces":
		keys = []string{"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "OTEL_EXPORTER_OTLP_PROTOCOL"}
	case "logs":
		keys = []string{"OTEL_EXPORTER_OTLP_LOGS_PROTOCOL", "OTEL_EXPORTER_OTLP_PROTOCOL"}
	default:
		return "http"
	}

	for _, key := range keys {
		switch strings.TrimSpace(strings.ToLower(os.Getenv(key))) {
		case "grpc":
			return "grpc"
		case "http", "http/protobuf", "http/json":
			return "http"
		}
	}
	return "http"
}

func hasAnyEnv(keys ...string) bool {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

type fanoutHandler struct {
	handlers []slog.Handler
}

func (h *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		err = errors.Join(err, handler.Handle(ctx, record.Clone()))
	}
	return err
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: handlers}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &fanoutHandler{handlers: handlers}
}

func init() {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		fmt.Fprintf(os.Stderr, "otel error: %v\n", err)
	}))
}
