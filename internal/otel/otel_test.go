package otel

import (
	"context"
	"testing"
)

func TestInit_NoEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Init(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if Enabled() {
		t.Fatal("expected Enabled() == false when endpoint is unset")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned error: %v", err)
	}
}

func TestInit_WithEndpoint(t *testing.T) {
	// Point at a dummy endpoint; the SDK won't actually connect during init.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	// Reset package state so the test is hermetic.
	enabled = false

	shutdown, err := Init(context.Background(), "test-svc", "0.0.1")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if !Enabled() {
		t.Fatal("expected Enabled() == true when endpoint is set")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	// Reset for other tests in the same process.
	enabled = false
}

func TestTracer_SafeBeforeInit(t *testing.T) {
	// Tracer() must not panic even if Init was never called.
	tr := Tracer()
	if tr == nil {
		t.Fatal("Tracer() returned nil")
	}
	// Start a span to confirm it's functional (no-op).
	_, span := tr.Start(context.Background(), "test-span")
	span.End()
}
