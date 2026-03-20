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

	// Critical logs a message at the highest severity level but does NOT exit.
	//
	// Critical logs are emitted at FATAL level to ensure visibility in backends,
	// but the process is GUARANTEED not to exit.
	//
	// Usage pattern:
	//   logger.Critical(ctx, "unrecoverable error", err)
	//   return err  // Let caller decide how to handle
	Critical(ctx context.Context, msg string, err error, fields ...Field)

	// With returns a child logger with additional fields attached.
	// Fields are included in all subsequent log entries.
	//
	// When called on an [Ion] instance, the returned Logger preserves access to
	// tracing and metrics (the concrete type is *Ion). For direct *Ion access
	// without a type assertion, use [Ion.Child] instead.
	With(fields ...Field) Logger

	// Named returns a named sub-logger.
	// The name appears in logs as the "logger" field.
	//
	// When called on an [Ion] instance, the returned Logger preserves access to
	// tracing and metrics (the concrete type is *Ion). For direct *Ion access
	// without a type assertion, use [Ion.Child] instead.
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
