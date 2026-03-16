package mcp

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestToolCallEmitsSpanAndMutationContext(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)
	s.SetTracer(tp.Tracer("test/mcp"))

	var mutationCtx context.Context
	s.SetOnMutation(func(ctx context.Context) {
		mutationCtx = ctx
	})

	parentTracer := tp.Tracer("test/parent")
	ctx, parent := parentTracer.Start(context.Background(), "parent")

	result, err := s.CallTool(ctx, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	parent.End()

	if result.IsError {
		t.Fatalf("replace returned MCP error result: %+v", result.Content)
	}
	if mutationCtx == nil {
		t.Fatal("expected mutation callback context")
	}

	mutationSpan := trace.SpanContextFromContext(mutationCtx)
	if !mutationSpan.IsValid() {
		t.Fatal("expected mutation callback to carry a valid span context")
	}

	var toolSpan sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.Name() == "mcp.tool.replace" {
			toolSpan = span
			break
		}
	}
	if toolSpan == nil {
		t.Fatal("expected mcp.tool.replace span")
	}
	if toolSpan.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatalf("replace span parent = %s, want %s", toolSpan.Parent().SpanID(), parent.SpanContext().SpanID())
	}
	if mutationSpan.SpanID() != toolSpan.SpanContext().SpanID() {
		t.Fatalf("mutation span = %s, want %s", mutationSpan.SpanID(), toolSpan.SpanContext().SpanID())
	}
}
