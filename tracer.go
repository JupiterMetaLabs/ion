package ion

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Link is an alias for trace.Link to avoid importing otel/trace.
type Link = trace.Link

// StatusCode is an alias for codes.Code, used with [Span.SetStatus].
// Ion exports status constants so callers do not need to import
// "go.opentelemetry.io/otel/codes" directly.
type StatusCode = codes.Code

const (
	// StatusUnset is the default span status. Most spans end with this.
	StatusUnset StatusCode = codes.Unset
	// StatusError indicates the operation contained an error.
	// Use with [Span.RecordError] for full error reporting.
	StatusError StatusCode = codes.Error
	// StatusOK explicitly marks the operation as successful.
	// Only set this when you need to override a default or parent status.
	StatusOK StatusCode = codes.Ok
)

// LinkFromContext extracts a link from the current context to connect spans.
func LinkFromContext(ctx context.Context) Link {
	return trace.LinkFromContext(ctx)
}

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
	// Use [StatusOK], [StatusError], or [StatusUnset].
	SetStatus(code StatusCode, description string)
	// RecordError records an error as an event.
	RecordError(err error)
	// SetAttributes sets attributes on the span.
	// Use attribute.String(), attribute.Int64(), etc. to create Attr values.
	SetAttributes(attrs ...Attr)
	// AddEvent adds an event to the span.
	// Use attribute.String(), attribute.Int64(), etc. to create Attr values.
	AddEvent(name string, attrs ...Attr)
}

// SpanOption configures span creation.
type SpanOption interface {
	apply(*spanOptions)
}

type spanOptions struct {
	kind       trace.SpanKind
	attributes []Attr
	links      []trace.Link
	otelOpts   []trace.SpanStartOption
}

type kindOption trace.SpanKind

func (k kindOption) apply(o *spanOptions) { o.kind = trace.SpanKind(k) }

// WithSpanKind sets the span kind (client, server, etc).
func WithSpanKind(kind trace.SpanKind) SpanOption { return kindOption(kind) }

type attrOption []Attr

func (a attrOption) apply(o *spanOptions) { o.attributes = append(o.attributes, a...) }

// WithAttributes adds attributes to the span.
// Use attribute.String(), attribute.Int64(), etc. to create Attr values.
func WithAttributes(attrs ...Attr) SpanOption { return attrOption(attrs) }

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

// otelTracer wraps an OpenTelemetry trace.Tracer to satisfy the Ion [Tracer] interface.
// It translates Ion's [SpanOption] types into native OTel span start options.
type otelTracer struct {
	tracer trace.Tracer
}

// newOTELTracer creates a Tracer backed by the global OTel tracer provider.
// The name identifies the instrumentation scope (e.g., "http.handler", "db.client").
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

func (s *otelSpan) End()                                   { s.span.End() }
func (s *otelSpan) SetStatus(code StatusCode, desc string) { s.span.SetStatus(code, desc) }
func (s *otelSpan) RecordError(err error)                  { s.span.RecordError(err) }
func (s *otelSpan) SetAttributes(attrs ...Attr)            { s.span.SetAttributes(attrs...) }
func (s *otelSpan) AddEvent(name string, attrs ...Attr) {
	s.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// --- No-op implementations ---

// noopTracer is a Tracer that creates no-op spans. Used when tracing is disabled.
type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _ string, _ ...SpanOption) (context.Context, Span) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) End()                         {}
func (noopSpan) SetStatus(StatusCode, string) {}
func (noopSpan) RecordError(error)            {}
func (noopSpan) SetAttributes(...Attr)        {}
func (noopSpan) AddEvent(string, ...Attr)     {}
