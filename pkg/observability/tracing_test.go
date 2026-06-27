package observability

import (
	"context"
	"testing"
)

func TestInitTracingDisabledIsNoop(t *testing.T) {
	tp, err := InitTracing(context.Background(), TracingConfig{Enabled: false})
	if err != nil {
		t.Fatalf("disabled init should not error: %v", err)
	}
	// Shutdown on a no-op handle must be safe.
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown: %v", err)
	}
}

func TestInitTracingStdoutExporter(t *testing.T) {
	tp, err := InitTracing(context.Background(), TracingConfig{
		Enabled:     true,
		Exporter:    "stdout",
		SampleRatio: 1.0,
		ServiceName: "test-svc",
	})
	if err != nil {
		t.Fatalf("stdout init: %v", err)
	}
	defer tp.Shutdown(context.Background())

	// A span can be started and ended without panic, and carries attributes.
	ctx, span := StartPhaseSpan(context.Background(), "merge", "target-a")
	if !span.SpanContext().IsValid() {
		t.Fatal("expected a valid (sampled) span context")
	}
	span.End()

	_, bspan := StartBackendSpan(ctx, "vault", "fetch")
	bspan.End()
}

func TestInitTracingUnknownExporter(t *testing.T) {
	if _, err := InitTracing(context.Background(), TracingConfig{Enabled: true, Exporter: "bogus"}); err == nil {
		t.Fatal("expected error for unknown exporter")
	}
}

func TestSpanHelpersSafeWhenDisabled(t *testing.T) {
	// With no provider configured, Tracer() is the global no-op; helpers must
	// still return usable (no-op) spans.
	ctx, span := StartPhaseSpan(context.Background(), "sync", "t")
	span.End()
	_, b := StartBackendSpan(ctx, "aws", "write")
	b.End()
}
