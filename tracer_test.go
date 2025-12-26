package ion

import (
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTracer_WithLinks(t *testing.T) {
	// calling WithLinks should not panic
	opts := WithLinks(trace.Link{
		SpanContext: trace.SpanContext{},
	})

	so := &spanOptions{}
	opts.apply(so)

	if len(so.links) != 1 {
		t.Errorf("Expected 1 link, got %d", len(so.links))
	}
}

func TestTracer_WithOTELOptions(t *testing.T) {
	// calling WithOTELOptions should not panic
	// We can't easily inspect the internal otel options without reflection or a mock,
	// but we can ensure it appends to our internal slice.

	// Create a dummy option
	dummyOpt := trace.WithAttributes()

	opts := WithOTELOptions(dummyOpt)

	so := &spanOptions{}
	opts.apply(so)

	if len(so.otelOpts) != 1 {
		t.Errorf("Expected 1 otel option, got %d", len(so.otelOpts))
	}
}
