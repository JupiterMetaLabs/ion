// Package ion provides enterprise-grade structured logging for JupiterMeta blockchain applications.
//
// Features:
//   - High-performance Zap core with zero-allocation hot paths
//   - Multi-destination output (Console, File, OTEL)
//   - Blockchain-specific field helpers (TxHash, ShardID, Slot, etc.)
//   - Automatic trace context propagation
//   - Pretty console output for development
//   - File rotation via lumberjack
//   - OpenTelemetry integration for observability
//
// Basic usage:
//
//	logger := ion.New(ion.Default())
//	defer logger.Sync()
//
//	logger.Info("server started", ion.F("port", 8080))
//
// With blockchain fields:
//
//	import "MRE/pkg/ion/fields"
//
//	logger.Info("transaction routed",
//	    fields.TxHash("abc123"),
//	    fields.ShardID(5),
//	    fields.LatencyMs(12.5),
//	)
//
// With context (auto-extracts trace_id):
//
//	logger.WithContext(ctx).Info("handling request")
package ion

import (
	"context"
)

// Logger is the primary logging interface.
// All methods are safe for concurrent use.
type Logger interface {
	// Debug logs a message at debug level.
	Debug(msg string, fields ...Field)

	// Info logs a message at info level.
	Info(msg string, fields ...Field)

	// Warn logs a message at warn level.
	Warn(msg string, fields ...Field)

	// Error logs a message at error level with an error.
	Error(msg string, err error, fields ...Field)

	// Fatal logs a message at fatal level and calls os.Exit(1).
	Fatal(msg string, err error, fields ...Field)

	// With returns a child logger with additional fields attached.
	// Fields are included in all subsequent log entries.
	With(fields ...Field) Logger

	// WithContext returns a child logger that extracts trace_id, span_id,
	// and other context values (request_id, user_id) from the context.
	WithContext(ctx context.Context) Logger

	// Named returns a named sub-logger.
	// The name appears in logs as the "component" field.
	Named(name string) Logger

	// Sync flushes any buffered log entries.
	// Applications should call Sync before exiting.
	Sync() error

	// SetLevel changes the log level at runtime.
	// Valid levels: debug, info, warn, error, fatal.
	SetLevel(level string)

	// GetLevel returns the current log level as a string.
	GetLevel() string
}

// Field represents a structured logging field (key-value pair).
type Field struct {
	Key   string
	Value any
}

// F is a convenience constructor for Field.
//
//	logger.Info("connected", ion.F("host", "localhost"), ion.F("port", 8080))
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field with the standard key "error".
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: nil}
	}
	return Field{Key: "error", Value: err.Error()}
}
