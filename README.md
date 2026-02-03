# Ion

**Ion** is an enterprise-grade observability client for Go services. It unifies **structured logging** (Zap) and **distributed tracing** (OpenTelemetry) into a single, cohesive API designed for high-throughput, long-running infrastructure.

> **Status:** v0.2 Release Candidate  
> **Target:** Microservices, Blockchain Nodes, Distributed Systems

---

## Guarantees & Design Invariants

Ion is built on strict operational guarantees. Operators can rely on these invariants in production:

1.  **No Process Termination**: Ion will **never** call `os.Exit`, `panic`, or `log.Fatal`. Even `Critical` level logs are strictly informational (mapped to FATAL severity) and guarantee control flow returns to the caller.
2.  **Thread Safety**: All public APIs on `Logger` and `Tracer` are safe for concurrent use by multiple goroutines.
3.  **Non-Blocking Telemetry**: Trace export is asynchronous and decoupled from application logic. A slow OTEL collector will never block your business logic (logs are synchronous to properly handle crash reporting, but rely on high-performance buffered writes).
4.  **Failure Isolation**: Telemetry backend failures (e.g., Collector down) are isolated. They may result in data loss (dropped spans) but will **never** crash the service.

## Non-Goals

To maintain focus and stability, Ion explicitly avoids:
*   **Alerting**: Ion emits signals; it does not manage thresholds or paging.
*   **Framework Magic**: Ion does not auto-inject into HTTP handlers without explicit middleware usage.

---

## Operational Model

### How Ion Works
*   **Logs**: Emitted **synchronously** to the configured cores (Console/File/Memory). This ensures that if your application crashes immediately after a log statement, the log is persisted (up to OS buffering).
*   **Traces**: Buffered and exported **asynchronously**. Spans are batched in memory and sent to the configured OTEL, endpoint on a timer or size threshold.
*   **Correlation**: `trace_id` and `span_id` are extracted from `context.Context` at the moment of logging and injected as fields.

### When to Use Logs vs Traces
*   **Logs**: Use for **state changes**, **errors**, and **high-cardinality events** (e.g., specific transaction failure reasons). Logs must be reliable and available immediately.
*   **Traces**: Use for **latency analysis**, **causality** (who called whom), and **request flows**. Traces are sampled and statistically significant, but individual traces may be dropped under load.

---

## Installation

```bash
go get github.com/JupiterMetaLabs/ion
```
Requires Go 1.21+.

---

## Quick Start

A minimal, correct example for a production service.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/JupiterMetaLabs/ion"
)

func main() {
    ctx := context.Background()

    // 1. Initialize with Service Identity
    // Returns warning slice for non-fatal config issues (e.g. invalid OTEL url)
    app, warnings, err := ion.New(ion.Default().WithService("payment-node"))
    if err != nil {
        log.Fatalf("Fatal: failed to init observability: %v", err)
    }
    for _, w := range warnings {
        log.Printf("Ion Startup Warning: %v", w)
    }

    // 2. Establishing the Lifecycle Contract
    // Ensure logs/traces flush before exit.
    defer func() {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        // Errors here mean data loss, not application failure.
        if err := app.Shutdown(shutdownCtx); err != nil {
            log.Printf("Shutdown data loss: %v", err)
        }
    }()

    // 3. Application Logic
    app.Info(ctx, "node started", ion.String("version", "1.0.0"))
    
    // Simulate work
    doWork(ctx, app)
}

func doWork(ctx context.Context, logger ion.Logger) {
    // Context is mandatory for correlation
    logger.Info(ctx, "processing block", ion.Uint64("height", 100))
}
```

---

## Configuration Reference

Ion uses a comprehensive configuration struct for behavior control. This maps 1:1 with `ion.Config`.

### Root Configuration (`ion.Config`)
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Level` | `string` | `"info"` | Minimum log level (`debug`, `info`, `warn`, `error`, `fatal`). |
| `Development` | `bool` | `false` | Enables development mode (pretty output, caller location, stack traces). |
| `ServiceName` | `string` | `"unknown"` | Identity of the service (vital for trace attribution). |
| `Version` | `string` | `""` | Service version (e.g., commit hash or semver). |
| `Console` | `ConsoleConfig` | `Enabled: true` | configuration for stdout/stderr. |
| `File` | `FileConfig` | `Enabled: false` | configuration for file logging (with rotation). |
| `OTEL` | `OTELConfig` | `Enabled: false` | configuration for remote OpenTelemetry logging. |
| `Tracing` | `TracingConfig` | `Enabled: false` | configuration for Distributed Tracing. |
| `Metrics` | `MetricsConfig` | `Enabled: false` | configuration for OpenTelemetry Metrics. |

### Console Configuration (`ion.ConsoleConfig`)
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Enabled` | `bool` | `true` | If false, stdout/stderr is silenced. |
| `Format` | `string` | `"json"` | `"json"` (production) or `"pretty"` (human-readable). |
| `Color` | `bool` | `true` | Enables ANSI colors (only references `pretty` format). |
| `ErrorsToStderr` | `bool` | `true` | Writes `warn`/`error`/`fatal` to stderr, others to stdout. |
| `Level` | `string` | `""` | Optional override for console log level. Inherits global level if empty. |

### File Configuration (`ion.FileConfig`)
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Enabled` | `bool` | `false` | Enables file writing. |
| `Path` | `string` | `""` | Absolute path to the log file (e.g., `/var/log/app.log`). |
| `MaxSizeMB` | `int` | `100` | Max size per file before rotation. |
| `MaxBackups` | `int` | `5` | Number of old files to keep. |
| `MaxAgeDays` | `int` | `7` | Max age of files to keep. |
| `Compress` | `bool` | `true` | Gzip old log files. |
| `Level` | `string` | `""` | Optional override for file log level. Inherits global level if empty. |

### OTEL Configuration (`ion.OTELConfig`)
Controls the OpenTelemetry **Logs** Exporter.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Enabled` | `bool` | `false` | Enables log export to Collector. |
| `Level` | `string` | `""` | Optional override for OTEL log level. Inherits global level if empty. |
| `Endpoint` | `string` | `""` | `host:port` or URL (e.g. `https://otel.io`). URL schemes override `Insecure` setting. |
| `Protocol` | `string` | `"grpc"` | `"grpc"` (recommended) or `"http"`. |
| `Insecure` | `bool` | `false` | If true, disables TLS (dev only). Ignored if Endpoint starts with `https://`. |
| `Username` | `string` | `""` | Basic Auth Username. |
| `Password` | `string` | `""` | Basic Auth Password. |
| `BatchSize` | `int` | `512` | Max logs per export batch. |
| `ExportInterval` | `Duration` | `5s` | flush interval. |

### Tracing Configuration (`ion.TracingConfig`)
Controls the OpenTelemetry **Trace** Provider.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Enabled` | `bool` | `false` | Enables trace generation and export. |
| `Endpoint` | `string` | `""` | `host:port`. **Inherits** `OTEL.Endpoint` if empty. |
| `Sampler` | `string` | `"ratio:0.1"` | `"always"`, `"never"`, or `"ratio:0.X"` (e.g., `ratio:0.1` for 10%). Development mode uses `"always"`. |
| `Protocol` | `string` | `"grpc"` | `"grpc"` or `"http"`. **Inherits** `OTEL.Protocol` if empty. |
| `Username` | `string` | `""` | **Inherits** `OTEL.Username` if empty. |
| `Password` | `string` | `""` | **Inherits** `OTEL.Password` if empty. |

### Metrics Configuration (`ion.MetricsConfig`)
Controls the OpenTelemetry **Metrics** Provider (OTLP Push).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Enabled` | `bool` | `false` | Enables metrics export. |
| `Endpoint` | `string` | `""` | `host:port`. **Inherits** `OTEL.Endpoint` if empty. |
| `Interval` | `Duration` | `15s` | Push interval. Development mode uses `5s`. |
| `Temporality` | `string` | `"cumulative"` | `"cumulative"` (Prometheus-compatible) or `"delta"`. |
| `Protocol` | `string` | `"grpc"` | **Inherits** `OTEL.Protocol` if empty. |
| `Username` | `string` | `""` | **Inherits** `OTEL.Username` if empty. |
| `Password` | `string` | `""` | **Inherits** `OTEL.Password` if empty. |

---

## Detailed Initialization

For full control, initialize the struct directly rather than using builders.

```go
cfg := ion.Config{
    Level:       "info",
    ServiceName: "payment-service",
    Version:     "v1.2.3",
    
    Console: ion.ConsoleConfig{
        Enabled:        true,
        Format:         "json",
        ErrorsToStderr: true,
    },
    
    // File rotation
    File: ion.FileConfig{
        Enabled:    true,
        Path:       "/var/log/payment-service.log",
        MaxSizeMB:  500,
        MaxBackups: 3,
        Compress:   true,
    },
    
    // Remote Telemetry (OTEL)
    OTEL: ion.OTELConfig{
        Enabled:  true,
        Endpoint: "otel-collector.prod:4317",
        Protocol: "grpc",
        // Attributes added to every log
        Attributes: map[string]string{
            "cluster": "us-east-1",
            "env":     "production",
        },
    },
    
    Tracing: ion.TracingConfig{
        Enabled: true,
        // Fallback: If Endpoint/Auth/Protocol are empty, they are inherited from OTEL config above.
        // This is safe to leave empty if using the same collector for logs and traces.
        Sampler: "ratio:0.05", // Sample 5% of traces
    },
}

// Initialize
app, warnings, err := ion.New(cfg)
if err != nil {
    panic(err)
}
// Handle warnings (e.g., invalid sampler string fallback)
for _, w := range warnings {
    log.Println("Ion config warning:", w)
}
defer app.Shutdown(context.Background())
```

---


> 📘 **Deep Dive**: For enterprise patterns, advanced tracing, and blockchain-specific examples, see the [Enterprise User Guide](docs/USER_GUIDE.md).

## Proper Usage Guide

### 1. The Logger
Use the Logger for human-readable events, state changes, and errors.

*   **Always** pass `context.Context` (even if generic).
*   **Prefer** typed fields (`ion.String`) over generic `ion.F`.
*   **Do not** log sensitive data (PII/Secrets).

```go
// INFO: Operational state changes
app.Info(ctx, "transaction processed", 
    ion.String("tx_id", "0x123"),
    ion.Duration("latency", 50 * time.Millisecond),
)

// ERROR: Actionable failures. Does not interrupt flow.
if err != nil {
    // Automatically adds "error": err.Error() field
    app.Error(ctx, "database connection failed", err, ion.String("db_host", "primary"))
}

// CRITICAL: Invariant violations (e.g. data corruption).
// Use this for "wake up the on-call" events.
// GUARANTEE: Does NOT call os.Exit(). Safe to use in libraries.
app.Critical(ctx, "memory corruption detected", nil)
```

```

### 3. Child Loggers (Scopes)
Use `With` and `Named` to create context-aware sub-loggers. This is often better than passing raw `ion` fields everywhere.

*   **`Named(name)`**: Appends to the logger name. Good for components.
    *   `app` -> `app.http` -> `app.http.client`
*   **`With(fields...)`**: Permanently attaches fields to all logs from this logger.

```go
// In your constructor
func NewPaymentService(root ion.Logger) *PaymentService {
    // All logs from this service will have "service": "payment" and "component": "core"
    // AND be named "main.payment"
    log := root.Named("payment").With(ion.String("component", "core"))
    
    return &PaymentService{log: log}
}

func (s *PaymentService) Process(ctx context.Context) {
    // Log comes out as:
    // logger="main.payment" component="core" msg="processing"
    s.log.Info(ctx, "processing") 
}
```

### 4. The Tracer
Use the Tracer for latency measurement and causal chains.

*   **Start/End**: Every `Start` **MUST** have a corresponding `End()`.
*   **Defer**: Use `defer span.End()` immediately after checking `err` isn't nil (or just immediately if function is simple).
*   **Attributes**: Add attributes to spans *only* if they are valuable for querying latency/filtering (e.g., "http.status_code"). High cardinality data belongs in Logs, not Spans attributes (usually).

```go
func ProcessOrder(ctx context.Context, orderID string) error {
    // 1. Get Named Tracer
    tracer := app.Tracer("order.processor")
    
    // 2. Start Span
    ctx, span := tracer.Start(ctx, "ProcessOrder")
    // 3. Ensure End
    defer span.End()
    
    // 4. Enrich Span
    span.SetAttributes(attribute.String("order.id", orderID))
    
    // ... work ...
    
    if err := validate(ctx); err != nil {
        // 5. Record Errors in Span
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        return err
    }
    
    return nil
}
```

> **📘 OTel Types in Ion**
> 
> Ion uses OpenTelemetry for tracing and metrics. The `ion.Attr` type (visible in Ion's API signatures) is an alias for `attribute.KeyValue`.
> 
> To create attributes, import the OTel package directly:
> ```go
> import "go.opentelemetry.io/otel/attribute"
> 
> span.SetAttributes(
>     attribute.String("order.id", orderID),
>     attribute.Int64("retry.count", 3),
> )
> ```
> 
> This is intentional: **Ion abstracts provider lifecycle, not the instrumentation API.** Users benefit from learning the standard OTel API.

---

## Common Configurations

Recipes for standard deployment scenarios.

---

## Initialization Scenarios

Ion is flexible. Here are the core patterns for different environments.

### 1. The "Standard" (Console Only)
Best for: CLI tools, scripts, local testing.
```go
cfg := ion.Default()
cfg.Level = "info" 
// cfg.Development = true // Optional: enables callers / stacktraces
```

### 2. The "Boxed Service" (Console + File)
Best for: Systemd services, VM-based deployments, legacy nodes.
```go
cfg := ion.Default()
// Console for live tailing (kubectl logs)
cfg.Console.Enabled = true 
// File for long-term retention
cfg.File.Enabled = true
cfg.File.Path = "/var/log/app/app.log"
cfg.File.MaxSizeMB = 500
cfg.File.MaxBackups = 5
```

### 3. The "Silent Agent" (OTEL Only)
Best for: High-traffic sidecars where local IO is expensive.
```go
cfg := ion.Default()
cfg.Console.Enabled = false // Disable local IO
cfg.OTEL.Enabled = true
cfg.OTEL.Endpoint = "localhost:4317"
cfg.OTEL.Protocol = "grpc"
```

### 4. The "Full Stack" (Console + OTEL + Tracing)
Best for: Kubernetes microservices.
```go
cfg := ion.Default()
cfg.Console.Enabled = true       // For pod logs
cfg.OTEL.Enabled = true          // For log aggregation (Loki/Elastic)
cfg.OTEL.Endpoint = "otu-col:4317"

cfg.Tracing.Enabled = true       // For distributed traces (Tempo/Jaeger)
cfg.Tracing.Sampler = "ratio:0.1" // 10% sampling
```

---

## 🔍 Tracing Guide for Developers

Distributed tracing is powerful but requires discipline. Follow these rules to get useful traces.

### 1. Span Lifecycle
*   **Root Spans**: Created by middleware (HTTP/gRPC). You rarely start these manually unless writing a background worker.
*   **Child Spans**: Created by `Start(ctx, name)`. Always inherit parent ID from `ctx`.

```go
// 1. Start (creates child if ctx has parent)
ctx, span := tracer.Start(ctx, "CalculateHash")
// 2. Defer End (Critical!)
defer span.End() 
```

### 2. Attributes vs Events vs Logs
*   **Attributes**: "Search Tags". Low cardinality. Use for filtering.
    *   *Good*: `user_id`, `http.status`, `region`, `retry_count`
    *   *Bad*: `error_message` (too variable), `payload_dump` (too big)
*   **Events**: "Timestamped Markers". Significant moments inside a span.
    *   *Example*: `span.AddEvent("cache_miss")`
*   **Logs**: "Detailed Context". High cardinality. Use `app.Info(ctx, ...)` instead.
    *   *Why*: Logs are cheaper and searchable by full text.

### 3. Handling Errors in Spans
**The Problem**: By default, a span finishes as `OK` even if your function returns an error. This results in **0% Error Rate** on your dashboard despite complete failure.

**The Fix**: You must explicitly "taint" the span.

```go
if err != nil {
    // 1. Record stacktrace and error type in the span
    span.RecordError(err)
    // 2. Flip the span status to Error (turns red in Jaeger/Tempo)
    span.SetStatus(codes.Error, "failed to insert order")
    
    // 3. Log it for humans (Logs = Detail, Traces = Signals)
    app.Error(ctx, "failed to insert order", err)
    return err
}
// Optional: Explicitly mark success if needed, though default is Unset/OK
span.SetStatus(codes.Ok, "success")
```

### 4. Spawning Goroutines
**The Problem**: Spans are bound to `context`. If the parent request finishes, `ctx` is canceled and the parent span Ends. A background goroutine using that same `ctx` becomes a "Ghost Span" — it might log errors, but it has no parent in the trace visualization (disconnected).

**The Fix**: Create a **Link**. A Link connects a new Root Span to the old Parent Trace, saying "This background job was caused by that request", without being killed by it.

```go
// Fire-and-Forget Background Task
go func(parentCtx context.Context) {
    // 1. Create a FRESH context (so we don't die when request finishes)
    newCtx := context.Background()
    tracer := app.Tracer("background_worker")
    
    // 2. Create a Link from the parent (Preserves causality)
    link := trace.LinkFromContext(parentCtx) 
    
    // 3. Start a new Root Span with the link
    ctx, span := tracer.Start(newCtx, "AsyncJob", ion.WithLinks(link))
    defer span.End()
    
    // Now you have a safe, independent span correlated to the original request
    app.Info(ctx, "processing in background", ion.String("job", "email_send"))
}(ctx)
```

---

## HTTP & gRPC Integration

Ion provides specialized middleware/interceptors to automate context propagation.

### HTTP Middleware (`middleware/ionhttp`)

```go
import "github.com/JupiterMetaLabs/ion/middleware/ionhttp"

mux := http.NewServeMux()
handler := ionhttp.Handler(mux, "payment-api") 
http.ListenAndServe(":8080", handler)
```

### gRPC Interceptors (`middleware/iongrpc`)

```go
import "github.com/JupiterMetaLabs/ion/middleware/iongrpc"

// Server
s := grpc.NewServer(
    grpc.StatsHandler(iongrpc.ServerHandler()),
)

// Client
conn, err := grpc.Dial(addr, 
    grpc.WithStatsHandler(iongrpc.ClientHandler()),
)
```

---

## Production Failure Modes

Operators must understand how Ion behaves under stress:

*   **OTEL Collector Down**: The internal exporter will retry with exponential backoff. If buffers fill, **new traces will be dropped**. Application performance is preserved (failure is isolated).
*   **Disk Full (File Logging)**: `lumberjack` rotation will attempt to write. If the write syscall fails, Zap internal error handling catches it. The application continues, but logs are lost (written to stderr fallback if possible).
*   **High Load**: Tracing uses a batch processor. Under extreme load, if the export rate lags generation, spans are dropped to prevent memory leaks (bounded buffer).

---

---

## Best Practices

1.  **Pass Context Everywhere**: `context.Background()` breaks the trace chain. Only use it in `main` or background worker roots.
2.  **Shutdown is Mandatory**: Failing to call `Shutdown` guarantees data loss (buffered traces/logs) on deployment.
3.  **Structured Keys**: Use consistent key names (e.g., `user_id`, not `userID` or `uid`) to make logs queryable.
4.  **No Dynamic Keys**: `ion.String(userInput, "value")` is a security risk and breaks indexing. Keys must be static constants.

---

## Versioning

*   **Public API**: `ion.go`, `logger.go`, `config.go`. Stable v0.2.
*   **Internal**: `internal/*`. No stability guarantees.
*   **Behavior**: Log format changes or configuration defaults are considered breaking changes.

---

## License

[MIT](LICENSE) © 2025 JupiterMeta Labs
