// Package ion provides enterprise-grade structured logging for JupiterMeta blockchain applications.
//
// Ion is designed for distributed systems where trace correlation is critical. All log methods
// require a context.Context as the first parameter to ensure trace information is never forgotten.
//
// Features:
//   - High-performance Zap core
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
