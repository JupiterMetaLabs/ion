// Package ion provides an enterprise-grade structured logger tailored for JupiterMeta blockchain services.
//
// It wraps the high-performance Zap logger and OpenTelemetry (OTEL) for observability, offering a simple
// yet powerful API for consistent logging across microservices.
//
// # Key Features
//
//   - Zero-allocation hot paths using a custom Zap core.
//   - Built-in OpenTelemetry (OTEL) integration for log export.
//   - Automatic context propagation (Trace ID, Span ID).
//   - Specialized field helpers for blockchain primitives (TxHash, Slot, ShardID).
//   - Configurable output formats (JSON for production, Pretty for development).
//   - Log rotation and compression via lumberjack.
//
// # Basic Usage
//
// Initialize the logger with a configuration:
//
//	import "github.com/JupiterMetaLabs/ion"
//
//	func main() {
//	    logger := ion.New(ion.Default())
//	    defer logger.Sync()
//
//	    logger.Info("application started", ion.F("version", "1.0.0"))
//	}
//
// # Context Support
//
// Use FromContext or WithContext to automatically attach trace identifiers:
//
//	func HandleRequest(ctx context.Context) {
//	    // Extracts trace_id and span_id if present in ctx
//	    logger.WithContext(ctx).Info("processing request", ion.F("user_id", 123))
//	}
package ion
