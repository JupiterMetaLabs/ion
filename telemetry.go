package ion

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap/zapcore"
)

// TelemetryLog provides a fluent API for trace-coupled logging.
// It creates both a log entry AND a trace span in a single call.
//
// Usage:
//
//	NewTelemetryLog(logger).
//	    Instrument("mre.routing").
//	    Component("pool").
//	    Trace("ROUTE_TX").
//	    Info("transaction routed", fields.TxHash("abc123"))
//
// The log will automatically include trace_id and span_id.
// Errors automatically mark the span as failed.
type TelemetryLog struct {
	logger     Logger
	ctx        context.Context
	instrument string // OTEL scope name (e.g., "mre.routing")
	component  string // Component name (e.g., "pool")
	module     string // Module name (e.g., "grpc")
	traceName  string // Span name (e.g., "ROUTE_TX")
	level      zapcore.Level
	tracer     trace.Tracer
}

// NewTelemetryLog creates a new fluent telemetry logger.
// Pass the base logger to use for output.
func NewTelemetryLog(logger Logger) *TelemetryLog {
	return &TelemetryLog{
		logger: logger,
		ctx:    context.Background(),
		level:  zapcore.InfoLevel,
	}
}

// NewTelemetryLogFromGlobal creates a TelemetryLog using the global logger.
func NewTelemetryLogFromGlobal() *TelemetryLog {
	return NewTelemetryLog(GetGlobal())
}

// --- Fluent Configuration Methods ---

// WithContext sets the context for trace propagation.
// If the context already has a span, the new span will be a child.
func (t *TelemetryLog) WithContext(ctx context.Context) *TelemetryLog {
	t.ctx = ctx
	return t
}

// Instrument sets the OTEL instrumentation scope name.
// Example: "mre.routing", "gossipnode.consensus"
func (t *TelemetryLog) Instrument(name string) *TelemetryLog {
	t.instrument = name
	t.tracer = otel.Tracer(name)
	return t
}

// Component sets the component name field.
func (t *TelemetryLog) Component(name string) *TelemetryLog {
	t.component = name
	return t
}

// Module sets the module name field.
func (t *TelemetryLog) Module(name string) *TelemetryLog {
	t.module = name
	return t
}

// Trace sets the span name to create.
// Example: "ROUTE_TX", "VALIDATE_BLOCK", "CONNECT_NODE"
func (t *TelemetryLog) Trace(spanName string) *TelemetryLog {
	t.traceName = spanName
	return t
}

// Level sets the log level.
func (t *TelemetryLog) Level(level zapcore.Level) *TelemetryLog {
	t.level = level
	return t
}

// --- Log Methods ---

// Debug logs at debug level with optional trace span.
func (t *TelemetryLog) Debug(msg string, fields ...Field) {
	t.log(zapcore.DebugLevel, msg, nil, fields)
}

// Info logs at info level with optional trace span.
func (t *TelemetryLog) Info(msg string, fields ...Field) {
	t.log(zapcore.InfoLevel, msg, nil, fields)
}

// Warn logs at warn level with optional trace span.
func (t *TelemetryLog) Warn(msg string, fields ...Field) {
	t.log(zapcore.WarnLevel, msg, nil, fields)
}

// Error logs at error level and marks span as error.
func (t *TelemetryLog) Error(msg string, err error, fields ...Field) {
	t.log(zapcore.ErrorLevel, msg, err, fields)
}

// Printf is a convenience method for formatted logging.
func (t *TelemetryLog) Printf(format string, args ...any) {
	msg := formatMessage(format, args...)
	t.log(t.level, msg, nil, nil)
}

// --- Internal ---

func (t *TelemetryLog) log(level zapcore.Level, msg string, err error, fields []Field) {
	ctx := t.ctx

	// Create span if trace name is set
	var span trace.Span
	if t.traceName != "" && t.tracer != nil {
		ctx, span = t.tracer.Start(ctx, t.traceName)
		defer span.End()

		// Add span attributes
		if t.component != "" {
			span.SetAttributes(attribute.String("component", t.component))
		}
		if t.module != "" {
			span.SetAttributes(attribute.String("module", t.module))
		}

		// Mark error on span
		if err != nil || level >= zapcore.ErrorLevel {
			span.SetStatus(codes.Error, msg)
			if err != nil {
				span.RecordError(err)
			}
		}
	}

	// Build fields with component/module
	allFields := make([]Field, 0, len(fields)+4)
	if t.component != "" {
		allFields = append(allFields, String("component", t.component))
	}
	if t.module != "" {
		allFields = append(allFields, String("module", t.module))
	}
	if t.instrument != "" {
		allFields = append(allFields, String("instrument", t.instrument))
	}
	allFields = append(allFields, fields...)

	// Log with context (injects trace_id, span_id)
	ctxLogger := t.logger.WithContext(ctx)

	switch level {
	case zapcore.DebugLevel:
		ctxLogger.Debug(msg, allFields...)
	case zapcore.InfoLevel:
		ctxLogger.Info(msg, allFields...)
	case zapcore.WarnLevel:
		ctxLogger.Warn(msg, allFields...)
	case zapcore.ErrorLevel:
		ctxLogger.Error(msg, err, allFields...)
	case zapcore.FatalLevel:
		ctxLogger.Fatal(msg, err, allFields...)
	default:
		ctxLogger.Info(msg, allFields...)
	}
}

func formatMessage(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// --- Global Logger Singleton ---

var (
	globalLogger Logger
	globalMu     sync.RWMutex
)

// SetGlobal sets the global logger instance.
// Call this early in application startup.
func SetGlobal(l Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

// GetGlobal returns the global logger instance.
// Returns a no-op logger if SetGlobal was never called.
func GetGlobal() Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger == nil {
		// Return a minimal default logger
		return New(Default())
	}
	return globalLogger
}

// L is a shorthand for GetGlobal().
func L() Logger {
	return GetGlobal()
}

// T is a shorthand for creating a TelemetryLog from the global logger.
func T() *TelemetryLog {
	return NewTelemetryLogFromGlobal()
}
