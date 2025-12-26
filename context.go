// Package ion context helpers provide functions for propagating trace, request,
// and user IDs through context.Context. These values are automatically extracted
// and included in log entries.
//
// For OTEL tracing, trace_id and span_id are automatically extracted from the
// span context. For non-OTEL scenarios, use WithTraceID to set manually.
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
