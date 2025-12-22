package ion

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// Context keys for custom values.
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
	traceIDKey   contextKey = "trace_id"
	spanIDKey    contextKey = "span_id"
)

// WithRequestID adds a request ID to the context.
// This ID will be automatically included in logs via WithContext().
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithUserID adds a user ID to the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithTraceID adds a trace ID to the context (for non-OTEL scenarios).
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

// extractContextFields pulls trace/span IDs and custom values from context.
// Called by WithContext() to automatically add context fields to logs.
func extractContextFields(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}

	fields := make([]Field, 0, 4)

	// Extract OTEL trace context (if available)
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		fields = append(fields,
			String("trace_id", spanCtx.TraceID().String()),
			String("span_id", spanCtx.SpanID().String()),
		)
	} else {
		// Fallback to manual trace ID if set
		if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
			fields = append(fields, String("trace_id", traceID))
		}
		if spanID, ok := ctx.Value(spanIDKey).(string); ok && spanID != "" {
			fields = append(fields, String("span_id", spanID))
		}
	}

	// Extract request ID
	if reqID, ok := ctx.Value(requestIDKey).(string); ok && reqID != "" {
		fields = append(fields, String("request_id", reqID))
	}

	// Extract user ID
	if userID, ok := ctx.Value(userIDKey).(string); ok && userID != "" {
		fields = append(fields, String("user_id", userID))
	}

	return fields
}
