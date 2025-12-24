# Ion

**Ion** is an enterprise-grade observability library for Go, providing unified **structured logging** and **distributed tracing** with seamless [OpenTelemetry](https://opentelemetry.io/) integration.

## Features

- 🚀 **High Performance** - Pool-optimized, low-allocation logging (~10ns)
- 🔭 **OpenTelemetry Native** - Logs AND traces with automatic correlation
- 🔗 **Trace Correlation** - `trace_id`/`span_id` automatically extracted from context
- 🛡️ **Enterprise Grade** - Graceful shutdown, file rotation, runtime level changes
- ⛓️ **Blockchain Ready** - Field helpers for `TxHash`, `BlockHeight`, `ShardID`

## Installation

```bash
go get github.com/JupiterMetaLabs/ion@latest
```

---

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/JupiterMetaLabs/ion"
)

func main() {
    ctx := context.Background()

    // Create Ion instance
    app, err := ion.New(ion.Default().WithService("myapp"))
    if err != nil {
        log.Fatal(err)
    }
    defer app.Shutdown(ctx) // CRITICAL: Always shutdown!

    // Log messages
    app.Info(ctx, "application started", ion.String("version", "1.0.0"))
    app.Debug(ctx, "debug info")
    app.Warn(ctx, "something concerning")
    app.Error(ctx, "operation failed", err, ion.String("op", "db_connect"))
}
```

---

## Table of Contents

1. [Initialization](#initialization)
2. [Shutdown](#shutdown)
3. [Logging](#logging)
4. [Child Loggers](#child-loggers)
5. [Tracing](#tracing)
6. [Trace-Log Correlation](#trace-log-correlation)
7. [Global Usage](#global-usage)
8. [Configuration Reference](#configuration-reference)
9. [Blockchain Fields](#blockchain-fields)
10. [Production Example](#production-example)

---

## Initialization

### Basic Logger (Console Only)

```go
app, err := ion.New(ion.Default().WithService("myapp"))
if err != nil {
    log.Fatal(err)
}
defer app.Shutdown(ctx)
```

### Development Mode (Pretty Console)

```go
app, err := ion.New(ion.Development())
if err != nil {
    log.Fatal(err)
}
```

### With OTEL Logging

```go
app, err := ion.New(ion.Default().
    WithService("myapp").
    WithOTEL("localhost:4317"))  // Collector endpoint
```

### With Tracing + OTEL Logging

```go
app, err := ion.New(ion.Default().
    WithService("myapp").
    WithOTEL("localhost:4317").
    WithTracing("localhost:4317"))
```

### Full Configuration

```go
cfg := ion.Config{
    Level:       "info",
    Development: false,
    ServiceName: "order-service",
    Version:     "v2.1.0",
    
    Console: ion.ConsoleConfig{
        Enabled:        true,
        Format:         "json",     // "json" or "pretty"
        Color:          true,
        ErrorsToStderr: true,       // warn/error → stderr
    },
    
    File: ion.FileConfig{
        Enabled:    true,
        Path:       "/var/log/app/app.log",
        MaxSizeMB:  100,
        MaxBackups: 5,
        Compress:   true,
    },
    
    OTEL: ion.OTELConfig{
        Enabled:   true,
        Endpoint:  "collector:4317",
        Protocol:  "grpc",          // "grpc" or "http"
        Insecure:  false,
        Username:  "user",          // Basic Auth (optional)
        Password:  "pass",
        Attributes: map[string]string{
            "env": "production",
        },
    },
    
    Tracing: ion.TracingConfig{
        Enabled:  true,
        Sampler:  "ratio:0.1",      // Sample 10%
        // Endpoint defaults to OTEL.Endpoint if not set
    },
}

app, err := ion.New(cfg)
```

---

## Shutdown

**Critical**: Always call `Shutdown()` before exit to flush logs and traces.

```go
func main() {
    app, _ := ion.New(cfg)
    
    // Use defer for graceful shutdown
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := app.Shutdown(ctx); err != nil {
            log.Printf("shutdown error: %v", err)
        }
    }()
    
    // ... application code
}
```

**What Shutdown does:**
1. Flushes pending log entries
2. Shuts down OTEL log exporter (if configured)
3. Shuts down tracer provider (flushes pending spans)

---

## Logging

### Log Levels

```go
app.Debug(ctx, "detailed debugging info")
app.Info(ctx, "normal operation")
app.Warn(ctx, "something might be wrong")
app.Error(ctx, "operation failed", err)  // err can be nil
app.Fatal(ctx, "unrecoverable error", err) // calls os.Exit(1)
```

### Structured Fields

```go
// Type-safe constructors (preferred)
app.Info(ctx, "user action",
    ion.String("user_id", "u123"),
    ion.Int("action_code", 42),
    ion.Float64("score", 0.95),
    ion.Bool("premium", true),
)

// Generic constructor (auto-detects type)
app.Info(ctx, "event", ion.F("key", value))
```

### Runtime Level Changes

```go
// Change level at runtime (thread-safe)
app.SetLevel("debug")

// Get current level
level := app.GetLevel() // "debug"
```

---

## Child Loggers

Child loggers add context that appears in all their log entries.

### Named (Component Scoping)

```go
// Create component-scoped loggers
httpLog := app.Named("http")
dbLog := app.Named("database")
cacheLog := app.Named("cache")

httpLog.Info(ctx, "request received")
// Output: {"logger":"http", "msg":"request received", ...}

dbLog.Info(ctx, "query executed")
// Output: {"logger":"database", "msg":"query executed", ...}
```

### With (Permanent Fields)

```go
// Add fields that persist across all logs from this child
userLog := app.With(
    ion.String("user_id", "u123"),
    ion.String("tenant", "acme"),
)

userLog.Info(ctx, "action performed")
// Output: {"user_id":"u123", "tenant":"acme", "msg":"action performed", ...}

// Children can be nested
sessionLog := userLog.With(ion.String("session_id", "s456"))
sessionLog.Info(ctx, "session event")
// Output: {"user_id":"u123", "tenant":"acme", "session_id":"s456", ...}
```

### Combining Named and With

```go
// Best practice: scope by component, then add instance fields
orderSvc := app.Named("orders").With(ion.String("region", "us-east"))

orderSvc.Info(ctx, "order processed")
// Output: {"logger":"orders", "region":"us-east", "msg":"order processed", ...}
```

---

## Tracing

Ion provides a clean tracing API that wraps OpenTelemetry.

### Creating a Tracer

```go
// Get a tracer for your component
tracer := app.Tracer("myapp.orders")  // Instrumentation scope name
```

### Creating Spans

```go
func ProcessOrder(ctx context.Context, orderID string) error {
    tracer := app.Tracer("myapp.orders")
    
    // Start a span (automatically linked to parent if in context)
    ctx, span := tracer.Start(ctx, "ProcessOrder")
    defer span.End()  // Always end spans!
    
    // Add attributes
    span.SetAttributes(attribute.String("order_id", orderID))
    
    // Log with trace correlation
    app.Info(ctx, "processing order", ion.String("order_id", orderID))
    
    if err := validateOrder(ctx, orderID); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        return err
    }
    
    span.SetStatus(codes.Ok, "")
    return nil
}
```

### Nested Spans

```go
func HandleRequest(ctx context.Context) {
    tracer := app.Tracer("myapp.api")
    
    ctx, parentSpan := tracer.Start(ctx, "HandleRequest")
    defer parentSpan.End()
    
    // Child spans automatically link to parent via context
    ctx, dbSpan := tracer.Start(ctx, "QueryDatabase")
    // ... db work
    dbSpan.End()
    
    ctx, cacheSpan := tracer.Start(ctx, "CheckCache")
    // ... cache work
    cacheSpan.End()
}
```

### Span Options

```go
// Set span kind
ctx, span := tracer.Start(ctx, "CallExternalAPI",
    ion.WithSpanKind(trace.SpanKindClient),
)

// Add attributes at creation
ctx, span := tracer.Start(ctx, "ProcessBatch",
    ion.WithAttributes(
        attribute.Int("batch_size", 100),
        attribute.String("source", "kafka"),
    ),
)
```

### Span Methods

```go
span.End()                           // Mark complete
span.SetStatus(codes.Ok, "")         // Set success
span.SetStatus(codes.Error, "msg")   // Set failure
span.RecordError(err)                // Record error event
span.SetAttributes(attrs...)         // Add attributes
span.AddEvent("checkpoint", attrs...)// Add event
```

---

## Trace-Log Correlation

Ion automatically extracts `trace_id` and `span_id` from context and adds them to logs.

```go
func HandleRequest(ctx context.Context) {
    tracer := app.Tracer("myapp")
    ctx, span := tracer.Start(ctx, "HandleRequest")
    defer span.End()
    
    // trace_id and span_id are automatically included!
    app.Info(ctx, "handling request")
}
```

**Log output:**
```json
{
  "level": "info",
  "msg": "handling request",
  "trace_id": "abc123...",
  "span_id": "def456...",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Manual Context Values

For non-OTEL scenarios, you can add IDs manually:

```go
ctx = ion.WithRequestID(ctx, "req-123")
ctx = ion.WithUserID(ctx, "user-456")
ctx = ion.WithTraceID(ctx, "trace-789")

app.Info(ctx, "event")
// Output includes: request_id, user_id, trace_id
```

---

## Global Usage

For scripts or legacy code where dependency injection is impractical.

### Setup

```go
func main() {
    app, _ := ion.New(ion.Default().WithService("script"))
    ion.SetGlobal(app)
    defer app.Shutdown(ctx)
    
    DoWork(ctx)
}
```

### Usage

```go
func DoWork(ctx context.Context) {
    // Package-level functions use global instance
    ion.Info(ctx, "working")
    ion.Debug(ctx, "debug info")
    
    // Get tracer from global
    tracer := ion.GetTracer("script.worker")
    ctx, span := tracer.Start(ctx, "Work")
    defer span.End()
    
    // Child loggers from global
    dbLog := ion.Named("database")
    dbLog.Info(ctx, "query executed")
}
```

### Accessing Global

```go
// Get global instance (panics if not set)
app := ion.L()

// Sync global logger
ion.Sync()
```

---

## Configuration Reference

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Level` | string | `"info"` | Log level: debug/info/warn/error/fatal |
| `Development` | bool | `false` | Enable dev mode (pretty output, caller info) |
| `ServiceName` | string | `"unknown"` | Service identifier |
| `Version` | string | `""` | Application version |

### ConsoleConfig

| Field | Default | Description |
|-------|---------|-------------|
| `Enabled` | `true` | Enable console output |
| `Format` | `"json"` | `"json"` or `"pretty"` |
| `Color` | `true` | ANSI colors in pretty mode |
| `ErrorsToStderr` | `true` | Send warn/error to stderr |

### FileConfig

| Field | Default | Description |
|-------|---------|-------------|
| `Enabled` | `false` | Enable file output |
| `Path` | `""` | Log file path |
| `MaxSizeMB` | `100` | Max size before rotation |
| `MaxBackups` | `5` | Old files to keep |
| `Compress` | `true` | Gzip old files |

### OTELConfig

| Field | Default | Description |
|-------|---------|-------------|
| `Enabled` | `false` | Enable OTEL export |
| `Endpoint` | `""` | Collector address |
| `Protocol` | `"grpc"` | `"grpc"` or `"http"` |
| `Insecure` | `false` | Disable TLS |
| `Username` | `""` | Basic Auth username |
| `Password` | `""` | Basic Auth password |
| `BatchSize` | `512` | Logs per batch |
| `ExportInterval` | `5s` | Batch export interval |

### TracingConfig

| Field | Default | Description |
|-------|---------|-------------|
| `Enabled` | `false` | Enable tracing |
| `Endpoint` | OTEL endpoint | Collector address (falls back to OTEL) |
| `Sampler` | `"always"` | `"always"`, `"never"`, `"ratio:0.5"` |
| `Protocol` | OTEL protocol | `"grpc"` or `"http"` |

---

## Blockchain Fields

Import the fields package for domain-specific helpers:

```go
import "github.com/JupiterMetaLabs/ion/fields"

app.Info(ctx, "transaction routed",
    fields.TxHash("0xabc123..."),
    fields.BlockHeight(1000000),
    fields.ShardID(3),
    fields.LatencyMs(12.5),
    fields.NodeID("validator-01"),
)
```

**Available fields:**
- **Transaction:** `TxHash`, `TxSignature`, `TxStatus`, `TxType`, `Nonce`, `GasLimit`, `GasPrice`, `GasUsed`, `Value`, `FromAddress`, `ToAddress`
- **Block:** `BlockHeight`, `BlockHash`, `Slot`, `Epoch`
- **Chain:** `ChainID`, `Network`, `ShardID`, `NodeID`, `Address`
- **Timing:** `LatencyMs`, `DurationMs`, `DurationSec`
- **Counts:** `Count`, `Size`, `Pending`, `Total`
- **Component:** `Component`, `Operation`, `Method`
- **Status:** `Success`, `Enabled`, `Reason`

---

## Production Example

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/JupiterMetaLabs/ion"
    "github.com/JupiterMetaLabs/ion/fields"
)

func main() {
    ctx := context.Background()
    
    // Production configuration
    cfg := ion.Config{
        Level:       "info",
        ServiceName: "order-service",
        Version:     os.Getenv("APP_VERSION"),
        
        Console: ion.ConsoleConfig{
            Enabled:        true,
            Format:         "json",
            ErrorsToStderr: true,
        },
        
        OTEL: ion.OTELConfig{
            Enabled:  true,
            Endpoint: os.Getenv("OTEL_ENDPOINT"),
            Insecure: os.Getenv("OTEL_INSECURE") == "true",
        },
        
        Tracing: ion.TracingConfig{
            Enabled: true,
            Sampler: "ratio:0.1",  // Sample 10% in production
        },
    }
    
    // Initialize
    app, err := ion.New(cfg)
    if err != nil {
        panic(err)
    }
    ion.SetGlobal(app)  // Optional: for global access
    
    // Graceful shutdown
    defer func() {
        shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
        defer cancel()
        if err := app.Shutdown(shutdownCtx); err != nil {
            os.Stderr.WriteString("shutdown error: " + err.Error() + "\n")
        }
    }()
    
    // Create component loggers
    log := app.Named("main")
    tracer := app.Tracer("order-service.main")
    
    // Start main span
    ctx, span := tracer.Start(ctx, "ApplicationStart")
    log.Info(ctx, "service starting")
    
    // Run your app
    orderSvc := NewOrderService(app)
    go orderSvc.Start(ctx)
    
    span.End()
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    sig := <-sigChan
    log.Info(ctx, "received shutdown signal", ion.String("signal", sig.String()))
}

type OrderService struct {
    log    ion.Logger
    tracer ion.Tracer
}

func NewOrderService(app *ion.Ion) *OrderService {
    return &OrderService{
        log:    app.Named("orders"),
        tracer: app.Tracer("order-service.orders"),
    }
}

func (s *OrderService) Start(ctx context.Context) {
    s.log.Info(ctx, "order service starting")
}

func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
    ctx, span := s.tracer.Start(ctx, "ProcessOrder")
    defer span.End()
    
    s.log.Info(ctx, "processing order", fields.TxHash(orderID))
    
    // ... business logic
    
    return nil
}
```

---

## Performance

| Scenario | Time | Allocations |
|----------|------|-------------|
| Info (no fields) | ~10ns | 0 |
| Info (3 fields) | ~90ns | 1 |
| Debug (filtered) | ~3ns | 0 |
| With trace context | ~150ns | 1-2 |

---

## License

[MIT](LICENSE) © 2025 JupiterMeta Labs
