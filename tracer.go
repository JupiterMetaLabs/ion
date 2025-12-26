package ion

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Tracer creates spans for distributed tracing.
type Tracer interface {
	// Start creates a new span.
	Start(ctx context.Context, spanName string, opts ...SpanOption) (context.Context, Span)
}

// Span represents a unit of work in a trace.
type Span interface {
	// End marks the span as complete.
	End()
	// SetStatus sets the span status.
	SetStatus(code codes.Code, description string)
	// RecordError records an error as an event.
	RecordError(err error)
	// SetAttributes sets attributes on the span.
	SetAttributes(attrs ...attribute.KeyValue)
	// AddEvent adds an event to the span.
	AddEvent(name string, attrs ...attribute.KeyValue)
}

// SpanOption configures span creation.
type SpanOption interface {
	apply(*spanOptions)
}

type spanOptions struct {
	kind       trace.SpanKind
	attributes []attribute.KeyValue
	links      []trace.Link
	otelOpts   []trace.SpanStartOption
}

type kindOption trace.SpanKind

func (k kindOption) apply(o *spanOptions) { o.kind = trace.SpanKind(k) }

// WithSpanKind sets the span kind (client, server, etc).
func WithSpanKind(kind trace.SpanKind) SpanOption { return kindOption(kind) }

type attrOption []attribute.KeyValue

func (a attrOption) apply(o *spanOptions) { o.attributes = append(o.attributes, a...) }

// WithAttributes adds attributes to the span.
func WithAttributes(attrs ...attribute.KeyValue) SpanOption { return attrOption(attrs) }

type linkOption []trace.Link

func (l linkOption) apply(o *spanOptions) { o.links = append(o.links, l...) }

// WithLinks adds links to the span.
func WithLinks(links ...trace.Link) SpanOption { return linkOption(links) }

type otelOption []trace.SpanStartOption

func (t otelOption) apply(o *spanOptions) { o.otelOpts = append(o.otelOpts, t...) }

// WithOTELOptions allows passing raw OpenTelemetry options directly.
// This is an escape hatch for advanced features not yet wrapped by Ion.
func WithOTELOptions(opts ...trace.SpanStartOption) SpanOption { return otelOption(opts) }

// --- OTEL Tracer Implementation ---

type otelTracer struct {
	tracer trace.Tracer
}

func newOTELTracer(name string) Tracer {
	return &otelTracer{tracer: otel.Tracer(name)}
}

func (t *otelTracer) Start(ctx context.Context, spanName string, opts ...SpanOption) (context.Context, Span) {
	o := &spanOptions{kind: trace.SpanKindInternal}
	for _, opt := range opts {
		opt.apply(o)
	}

	traceOpts := []trace.SpanStartOption{trace.WithSpanKind(o.kind)}
	if len(o.attributes) > 0 {
		traceOpts = append(traceOpts, trace.WithAttributes(o.attributes...))
	}
	if len(o.links) > 0 {
		traceOpts = append(traceOpts, trace.WithLinks(o.links...))
	}
	if len(o.otelOpts) > 0 {
		traceOpts = append(traceOpts, o.otelOpts...)
	}

	ctx, span := t.tracer.Start(ctx, spanName, traceOpts...)
	return ctx, &otelSpan{span: span}
}

type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) End()                                      { s.span.End() }
func (s *otelSpan) SetStatus(code codes.Code, desc string)    { s.span.SetStatus(code, desc) }
func (s *otelSpan) RecordError(err error)                     { s.span.RecordError(err) }
func (s *otelSpan) SetAttributes(attrs ...attribute.KeyValue) { s.span.SetAttributes(attrs...) }
func (s *otelSpan) AddEvent(name string, attrs ...attribute.KeyValue) {
	s.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// --- No-op implementations ---

type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _ string, _ ...SpanOption) (context.Context, Span) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) End()                                   {}
func (noopSpan) SetStatus(codes.Code, string)           {}
func (noopSpan) RecordError(error)                      {}
func (noopSpan) SetAttributes(...attribute.KeyValue)    {}
func (noopSpan) AddEvent(string, ...attribute.KeyValue) {}
