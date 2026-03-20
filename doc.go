// Package ion provides production-grade logging, tracing, and metrics for Go services.
//
// Ion unifies structured logging (Zap), distributed tracing, and metrics
// (OpenTelemetry) behind a minimal, context-first API. It is designed for
// long-running services and distributed systems where trace correlation,
// failure isolation, and zero-downtime observability are critical.
//
// # Guarantees
//
// These invariants hold in all configurations, including under backend failure:
//
//   - Process Safety: Ion never terminates the process (no os.Exit, no panic).
//     Even [Logger.Critical] (mapped to FATAL severity) returns control to the caller.
//   - Concurrency: All [Logger], [Tracer], and Meter APIs are safe for concurrent use.
//   - Failure Isolation: Telemetry backend failures (collector down, disk full)
//     never crash application logic. Data may be lost, but the service continues.
//   - Lifecycle: [Ion.Shutdown] flushes all buffers on a best-effort basis within
//     the provided context deadline.
//
// # Initialization
//
// Create an Ion instance with [New]. Use [Default] for production defaults
// or [Development] for local development (pretty output, debug level):
//
//	app, warnings, err := ion.New(ion.Default().WithService("my-service"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, w := range warnings {
//	    log.Printf("ion warning: %v", w)
//	}
//	defer app.Shutdown(context.Background())
//
// [New] always returns a working instance when err is nil. Non-fatal issues
// (e.g., OTEL connection failure) are reported as [Warning] values — the service
// continues with degraded telemetry rather than failing to start.
//
// # Context-First Logging
//
// All log methods require [context.Context] as the first parameter. Ion automatically
// extracts trace_id and span_id from the active OTEL span in the context and injects
// them as structured fields. This ensures every log entry is correlated to its trace
// without any manual plumbing:
//
//	app.Info(ctx, "order processed", ion.String("order_id", "abc"), ion.Duration("latency", elapsed))
//
// Use typed field constructors ([String], [Int], [Int64], [Uint64], [Float64], [Bool],
// [Duration], [Err]) for zero-allocation structured logging. Use [F] for arbitrary types.
//
// # The Logger Interface
//
// The [Logger] interface defines the logging contract: [Logger.Debug], [Logger.Info],
// [Logger.Warn], [Logger.Error], and [Logger.Critical]. Accept [Logger] at package
// boundaries for dependency injection. Use *[Ion] internally when a component needs
// tracing or metrics.
//
// # Scoped Children
//
// Use [Ion.Child] to create scoped observability instances for application components.
// Each child preserves full access to logging, tracing, and metrics:
//
//	http := app.Child("http")
//	http.Info(ctx, "request received")
//	tracer := http.Tracer("http.handler")
//	meter := http.Meter("http.metrics")
//
// [Ion.Named] and [Ion.With] also preserve observability (the concrete type behind
// the [Logger] interface is *[Ion]), but [Ion.Child] returns *[Ion] directly — no
// type assertion required. Use [Ion.Child] when your component needs tracing or metrics.
//
// # Tracing and Metrics
//
// Access distributed tracing via [Ion.Tracer] and metrics via [Ion.Meter]:
//
//	tracer := app.Tracer("order.processor")
//	ctx, span := tracer.Start(ctx, "ProcessOrder")
//	defer span.End()
//
//	meter := app.Meter("order.metrics")
//	counter, _ := meter.Int64Counter("orders_processed_total")
//	counter.Add(ctx, 1)
//
// If tracing or metrics are not enabled in the configuration, these methods return
// no-op implementations that silently discard data — no nil checks needed.
//
// Ion provides status constants ([StatusOK], [StatusError], [StatusUnset]) so that
// callers do not need to import OpenTelemetry codes directly:
//
//	span.SetStatus(ion.StatusError, "validation failed")
//
// # Configuration
//
// Ion uses a comprehensive [Config] struct. Start with [Default] or [Development],
// then customize with builder methods:
//
//	cfg := ion.Default().
//	    WithService("payment-api").
//	    WithOTEL("otel-collector:4317").
//	    WithTracing("").   // inherits OTEL endpoint
//	    WithMetrics("")    // inherits OTEL endpoint
//
// Tracing, Metrics, and OTEL Logs inherit endpoint, protocol, and auth from the
// OTEL config when their own values are empty — configure once, reuse everywhere.
//
// # Context Helpers
//
// Inject custom identifiers into context for automatic inclusion in logs:
//
//	ctx = ion.WithRequestID(ctx, "req-123")
//	ctx = ion.WithUserID(ctx, "user-42")
//	app.Info(ctx, "processing")  // logs include request_id="req-123", user_id="user-42"
//
// Extract values with [RequestIDFromContext], [UserIDFromContext], [TraceIDFromContext].
//
// # Lifecycle
//
// [Ion.Shutdown] must be called before process exit to flush buffered traces, metrics,
// and OTEL logs. Always defer it in main with a timeout:
//
//	defer func() {
//	    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	    defer cancel()
//	    app.Shutdown(ctx)
//	}()
//
// Child instances share the parent's tracer and meter providers. Only shut down
// the root [Ion] instance — shutting down a child tears down shared providers.
//
// # Sub-Packages
//
// The fields sub-package (github.com/JupiterMetaLabs/ion/fields) provides
// domain-specific field constructors for blockchain applications: TxHash,
// BlockHeight, ShardID, Slot, Epoch, Validator, and more.
//
// The middleware sub-packages provide automatic context propagation for HTTP
// (github.com/JupiterMetaLabs/ion/middleware/ionhttp) and gRPC
// (github.com/JupiterMetaLabs/ion/middleware/iongrpc).
package ion
