// Package ion provides enterprise-grade structured logging for JupiterMeta blockchain applications.
//
// Ion is designed for distributed systems where trace correlation is critical. All log methods
// require a context.Context as the first parameter to ensure trace information is never forgotten.
//
// Features:
//   - High-performance Zap core with pool-optimized hot paths
//   - Multi-destination output (Console, File, OTEL)
//   - Blockchain-specific field helpers (TxHash, ShardID, Slot, etc.)
//   - Automatic trace context propagation from context.Context
//   - Pretty console output for development
//   - File rotation via lumberjack
//   - OpenTelemetry integration for observability
//
// Basic usage:
//
//	ctx := context.Background()
//	logger, warnings, _ := ion.New(ion.Default())
//	defer logger.Sync()
//
//	logger.Info(ctx, "server started", ion.Int("port", 8080))
//
// With blockchain fields:
//
//	import "github.com/JupiterMetaLabs/ion/fields"
//
//	logger.Info(ctx, "transaction routed",
//	    fields.TxHash("abc123"),
//	    fields.ShardID(5),
//	    fields.LatencyMs(12.5),
//	)
//
// Context-first design ensures trace_id and span_id are automatically extracted:
//
//	func HandleRequest(ctx context.Context) {
//	    // trace_id and span_id from ctx are added to logs automatically
//	    logger.Info(ctx, "processing request")
//	}
package ion

import (
	"context"
)

// Logger is the primary logging interface.
// All methods are safe for concurrent use.
// All log methods require a context.Context as the first parameter for trace correlation.
type Logger interface {
	// Debug logs a message at debug level.
	Debug(ctx context.Context, msg string, fields ...Field)

	// Info logs a message at info level.
	Info(ctx context.Context, msg string, fields ...Field)

	// Warn logs a message at warn level.
	Warn(ctx context.Context, msg string, fields ...Field)

	// Error logs a message at error level with an error.
	Error(ctx context.Context, msg string, err error, fields ...Field)

	// Fatal logs a message at fatal level and calls os.Exit(1).
	//
	// IMPORTANT: Fatal attempts to flush logs and shutdown OTEL before exiting,
	// but some logs may be lost if buffers are full. For graceful shutdown,
	// prefer returning errors and calling Shutdown() explicitly.
	Fatal(ctx context.Context, msg string, err error, fields ...Field)

	// With returns a child logger with additional fields attached.
	// Fields are included in all subsequent log entries.
	With(fields ...Field) Logger

	// Named returns a named sub-logger.
	// The name appears in logs as the "component" field.
	Named(name string) Logger

	// Sync flushes any buffered log entries.
	// Applications should call Sync before exiting.
	Sync() error

	// Shutdown gracefully shuts down the logger, flushing any buffered logs
	// and closing background resources (like OTEL exporters).
	Shutdown(ctx context.Context) error

	// SetLevel changes the log level at runtime.
	// Valid levels: debug, info, warn, error, fatal.
	SetLevel(level string)

	// GetLevel returns the current log level as a string.
	GetLevel() string
}

// FieldType roughly mirrors zapcore.FieldType
type FieldType uint8

const (
	UnknownType FieldType = iota
	StringType
	Int64Type
	Uint64Type
	Float64Type
	BoolType
	ErrorType
	AnyType
)

// Field represents a structured logging field (key-value pair).
// Field construction is zero-allocation for primitive types (String, Int, etc).
type Field struct {
	Key       string
	Type      FieldType
	Integer   int64
	StringVal string
	Float     float64
	Interface any
}

// F is a convenience constructor for Field.
// It detects the type and creates the appropriate Field.
func F(key string, value any) Field {
	switch v := value.(type) {
	case string:
		return String(key, v)
	case int:
		return Int(key, v)
	case int64:
		return Int64(key, v)
	case float64:
		return Float64(key, v)
	case bool:
		return Bool(key, v)
	case error:
		return Err(v)
	default:
		return Field{Key: key, Type: AnyType, Interface: value}
	}
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Type: StringType, StringVal: value}
}

// Int creates an integer field.
func Int(key string, value int) Field {
	return Field{Key: key, Type: Int64Type, Integer: int64(value)}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Type: Int64Type, Integer: value}
}

// Uint64 creates a uint64 field without truncation.
// Use this for large unsigned values (e.g., block heights, slots).
func Uint64(key string, value uint64) Field {
	return Field{Key: key, Type: Uint64Type, Interface: value}
}

// Float64 creates a float64 field.
func Float64(key string, value float64) Field {
	return Field{Key: key, Type: Float64Type, Float: value}
}

// Bool creates a boolean field.
func Bool(key string, value bool) Field {
	var i int64
	if value {
		i = 1
	}
	return Field{Key: key, Type: BoolType, Integer: i}
}

// Err creates an error field with the standard key "error".
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Type: AnyType, Interface: nil}
	}
	return Field{Key: "error", Type: ErrorType, Interface: err}
}
