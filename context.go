package ion

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// contextKey is an unexported type for context keys defined in this package.
// This prevents collisions with keys defined in other packages.
type contextKey string

// Context keys for storing log-relevant values in context.Context.
// These values are automatically extracted and added to log entries.
const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
	traceIDKey   contextKey = "trace_id"
	spanIDKey    contextKey = "span_id"
)

// WithRequestID adds a request ID to the context.
// This ID will be automatically included in logs.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithUserID adds a user ID to the context.
// This ID will be automatically included in logs as the "user_id" field.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithTraceID adds a trace ID to the context for non-OTEL scenarios.
// When OTEL tracing is active, trace IDs are extracted automatically from the span context;
// use this only when you need manual trace correlation without an OTEL span.
// This ID will be automatically included in logs as the "trace_id" field.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// UserIDFromContext extracts the user ID from context.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// TraceIDFromContext extracts the trace ID from context.
// It first checks for an active OTEL span; if none, falls back to a manually set trace ID.
// Returns an empty string if no trace ID is available.
func TraceIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// extractContextZapFields pulls trace/span IDs and custom values from context.
// Returns zap.Field slice directly for use in log methods (avoids Field conversion).
// Lazily allocates the slice only when fields are found.
func extractContextZapFields(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}

	var fields []zap.Field

	// Extract OTEL trace context (if available)
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		fields = make([]zap.Field, 0, 4)
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	} else {
		// Fallback to manual trace ID if set
		if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
			fields = make([]zap.Field, 0, 4)
			fields = append(fields, zap.String("trace_id", traceID))
		}
		if spanID, ok := ctx.Value(spanIDKey).(string); ok && spanID != "" {
			if fields == nil {
				fields = make([]zap.Field, 0, 4)
			}
			fields = append(fields, zap.String("span_id", spanID))
		}
	}

	// Extract request ID
	if reqID, ok := ctx.Value(requestIDKey).(string); ok && reqID != "" {
		if fields == nil {
			fields = make([]zap.Field, 0, 4)
		}
		fields = append(fields, zap.String("request_id", reqID))
	}

	// Extract user ID
	if userID, ok := ctx.Value(userIDKey).(string); ok && userID != "" {
		if fields == nil {
			fields = make([]zap.Field, 0, 4)
		}
		fields = append(fields, zap.String("user_id", userID))
	}

	return fields
}
