package render

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestDOMChangedUpdateEmitsRenderSpans(t *testing.T) {
	m, _ := newTestModelWithTree(t)

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)
	m.SetTracer(tp.Tracer("test/render"))

	parentTracer := tp.Tracer("test/parent")
	ctx, parent := parentTracer.Start(context.Background(), "parent")

	newM, _ := m.Update(DOMChangedMsg{Ctx: ctx})
	_ = newM.(Model)
	parent.End()

	var updateSpan, refreshSpan, syncSpan sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		switch span.Name() {
		case "render.update":
			updateSpan = span
		case "render.refresh_scripts_and_widgets":
			refreshSpan = span
		case "render.sync_state":
			syncSpan = span
		}
	}

	if updateSpan == nil {
		t.Fatal("expected render.update span")
	}
	if refreshSpan == nil {
		t.Fatal("expected render.refresh_scripts_and_widgets span")
	}
	if syncSpan == nil {
		t.Fatal("expected render.sync_state span")
	}
	if updateSpan.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatalf("render.update parent = %s, want %s", updateSpan.Parent().SpanID(), parent.SpanContext().SpanID())
	}
	if refreshSpan.Parent().SpanID() != updateSpan.SpanContext().SpanID() {
		t.Fatalf("refresh span parent = %s, want %s", refreshSpan.Parent().SpanID(), updateSpan.SpanContext().SpanID())
	}
	if syncSpan.Parent().SpanID() != refreshSpan.SpanContext().SpanID() {
		t.Fatalf("sync span parent = %s, want %s", syncSpan.Parent().SpanID(), refreshSpan.SpanContext().SpanID())
	}
}
