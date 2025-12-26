// Package ion provides production-grade logging and tracing for Go services.
//
// Ion unifies structured logging (Zap) and distributed tracing
// (OpenTelemetry) behind a minimal, context-first API.
//
// # Guarantees
//
//   - Process Safety: Ion never terminates the process (no os.Exit, no panic).
//   - Concurrency: All Logger and Tracer APIs are safe for concurrent use.
//   - Failure Isolation: Telemetry backend failures never crash application logic.
//   - Lifecycle: Shutdown(ctx) flushes all buffers on a best-effort basis.
//
// # Architecture
//
//   - Logs: Synchronous, structured, strongly typed.
//   - Traces: Asynchronous, sampled, batched.
//   - Correlation: Automatic injection of trace_id/span_id from context.Context.
//
// Ion is designed for long-running services and distributed systems.
// It is not a metrics SDK or a web framework.
package ion
