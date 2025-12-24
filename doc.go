// Package ion provides enterprise-grade structured logging for JupiterMeta blockchain services.
//
// Ion wraps the high-performance [Zap] logger with [OpenTelemetry] integration,
// offering a simple yet powerful API for consistent, observable logging across microservices.
//
// # Key Features
//
//   - Pool-optimized hot paths with minimal allocations
//   - Built-in OpenTelemetry (OTEL) integration for log export
//   - Automatic context propagation (trace_id, span_id)
//   - Specialized field helpers for blockchain primitives (TxHash, Slot, ShardID)
//   - Configurable output formats (JSON for production, Pretty for development)
//   - Log rotation and compression via lumberjack
//
// # Quick Start
//
// Global logger pattern (recommended for applications):
//
//	package main
//
//	import (
//	    "context"
//
//	    "github.com/JupiterMetaLabs/ion"
//	)
//
//	func main() {
//	    ctx := context.Background()
//
//	    ion.SetGlobal(ion.InitFromEnv())
//	    defer ion.Sync()
//
//	    ion.Info(ctx, "application started", ion.String("version", "1.0.0"))
//	}
//
// # Dependency Injection
//
// For libraries or explicit dependencies, pass [Logger] directly:
//
//	func NewServer(logger ion.Logger) *Server {
//	    return &Server{log: logger.Named("server")}
//	}
//
//	func (s *Server) Start(ctx context.Context) {
//	    s.log.Info(ctx, "server started", ion.Int("port", 8080))
//	}
//
// # Context-First Logging
//
// Context is always the first parameter. Trace IDs are extracted automatically:
//
//	func HandleRequest(ctx context.Context) {
//	    // trace_id and span_id are added to logs if present in ctx
//	    logger.Info(ctx, "processing request")
//	}
//
// For startup and shutdown logs where no trace context exists:
//
//	ion.Info(context.Background(), "service starting")
//
// # Configuration
//
// Ion supports configuration via code or environment variables:
//
//	cfg := ion.Default()
//	cfg.Level = "debug"
//	cfg.OTEL.Enabled = true
//	cfg.OTEL.Endpoint = "otel-collector:4317"
//
//	logger, warnings, err := ion.New(cfg)
//
// Environment variables supported by [InitFromEnv]:
//
//	LOG_LEVEL        - debug, info, warn, error (default: info)
//	LOG_DEVELOPMENT  - "true" for pretty console output
//	SERVICE_NAME     - service name for logs and OTEL
//	SERVICE_VERSION  - service version for OTEL resources
//	OTEL_ENDPOINT    - collector address, enables OTEL if set
//	OTEL_INSECURE    - "true" to disable TLS
//	OTEL_USERNAME    - Basic Auth username
//	OTEL_PASSWORD    - Basic Auth password
//
// [Zap]: https://github.com/uber-go/zap
// [OpenTelemetry]: https://opentelemetry.io/
package ion
