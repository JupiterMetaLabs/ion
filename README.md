# Ion

**Ion** is an enterprise-grade, structured logging library designed for high-performance blockchain applications at [JupiterMeta Labs](https://github.com/JupiterMetaLabs).

It combines the raw speed of [Zap](https://github.com/uber-go/zap) with seamless [OpenTelemetry (OTEL)](https://opentelemetry.io/) integration, ensuring your logs are both fast and observable.

## Features

-   🚀 **High Performance**: Built on Uber's Zap with pool-optimized, low-allocation hot paths.
-   🔭 **OpenTelemetry Native**: Seamless integration with OTEL for distributed tracing (Logs + Traces).
-   🛡️ **Enterprise Grade**: Reliable `Shutdown` hooks, internal pooling, and safe concurrency patterns.
-   🔗 **Blockchain Ready**: Specialized field helpers for `TxHash`, `Slot`, `ShardID`, and more.
-   📝 **Developer Friendly**: Pretty console output for local dev, JSON for production.
-   🔄 **Lumberjack**: Built-in file rotation and compression.

## Installation

```bash
go get github.com/JupiterMetaLabs/ion@latest
```

---

## Quick Start

### 1. Global Logger (Recommended for Apps)

```go
package main

import (
    "context"
    
    "github.com/JupiterMetaLabs/ion"
)

func main() {
    ctx := context.Background()

    // Configure from environment variables
    ion.SetGlobal(ion.InitFromEnv())
    defer ion.Sync()

    ion.Info(ctx, "application started")
    RunServer(ctx)
}

func RunServer(ctx context.Context) {
    ion.Info(ctx, "server listening", ion.Int("port", 8080))
}
```

### 2. Dependency Injection (Recommended for Libraries)

```go
type Server struct {
    logger ion.Logger
}

func NewServer(l ion.Logger) *Server {
    return &Server{logger: l.Named("server")}
}

func (s *Server) Start(ctx context.Context) {
    s.logger.Info(ctx, "server started")
}
```

---

## Core Concepts

### Child Loggers (`With` and `Named`)

Create scoped loggers that inherit configuration but add specific context:

-   **`With`**: Adds permanent fields to every log entry.
-   **`Named`**: Adds a "logger" name (component identifier).

```go
dbLogger := logger.Named("db").With(ion.String("db_name", "orders"))

dbLogger.Info(ctx, "connection established") 
// Output: {"level":"info", "logger":"db", "db_name":"orders", "msg":"..."}
```

### Context Integration

Context is **always the first parameter**. Trace IDs are extracted automatically:

```go
func HandleRequest(ctx context.Context) {
    // trace_id and span_id are extracted from ctx automatically
    logger.Info(ctx, "processing request", ion.String("endpoint", "/api/orders"))
}

// For startup/shutdown (no trace):
ion.Info(context.Background(), "service starting")
```

---

## Configuration

Ion uses a strongly typed `Config` struct:

```go
cfg := ion.Default()
cfg.Level = "debug"
cfg.ServiceName = "payment-service"
cfg.OTEL.Enabled = true
cfg.OTEL.Endpoint = "otel-collector:4317"

logger := ion.New(cfg)
```

### Production Configuration

```go
cfg := ion.Config{
    Level:       "info",
    Development: false,
    ServiceName: "payment-service",
    Version:     "1.2.0",
    
    // File Logging with Rotation
    File: ion.FileConfig{
        Enabled:    true,
        Path:       "/var/log/app/service.log",
        MaxSizeMB:  100,
        MaxBackups: 10,
        MaxAgeDays: 7,
        Compress:   true,
    },

    // OpenTelemetry Export
    OTEL: ion.OTELConfig{
        Enabled:  true,
        Endpoint: "otel-collector:4317",
        Protocol: "grpc",
        Insecure: false,
        Username: "admin",        // Basic Auth (optional)
        Password: "supersecret",  // Basic Auth (optional)
        Attributes: map[string]string{
            "env": "production",
        },
    },
}
logger, _ := ion.NewWithOTEL(cfg)
defer logger.Shutdown(ctx)
```

---

## Environment Variables

`InitFromEnv()` reads the following environment variables for zero-code configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Minimum log level: `debug`, `info`, `warn`, `error` |
| `LOG_DEVELOPMENT` | `false` | Set to `true` for pretty console output |
| `SERVICE_NAME` | `unknown` | Service name for logs and OTEL |
| `SERVICE_VERSION` | `""` | Service version for OTEL resources |
| `OTEL_ENDPOINT` | `""` | OTEL collector address (enables OTEL if set) |
| `OTEL_INSECURE` | `false` | Set to `true` to disable TLS |
| `OTEL_USERNAME` | `""` | Basic Auth username |
| `OTEL_PASSWORD` | `""` | Basic Auth password |

Example deployment:

```bash
export LOG_LEVEL=info
export SERVICE_NAME=payment-service
export OTEL_ENDPOINT=otel-collector:4317
export OTEL_USERNAME=admin
export OTEL_PASSWORD=secret

./my-service
```

---

## Configuration Reference

### Core Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | `string` | `info` | Minimum log level |
| `development` | `bool` | `false` | Pretty printing + caller info |
| `service_name` | `string` | `unknown` | Service identifier |
| `version` | `string` | `""` | Service version |

### Console Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `console.enabled` | `bool` | `true` | Enable stdout/stderr output |
| `console.format` | `string` | `json` | `json` or `pretty` |
| `console.color` | `bool` | `true` | ANSI colors in pretty mode |
| `console.errors_to_stderr` | `bool` | `true` | Send warn/error to stderr |

### File Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `file.enabled` | `bool` | `false` | Enable file logging |
| `file.path` | `string` | `""` | Log file path |
| `file.max_size_mb` | `int` | `100` | Max size before rotation (MB) |
| `file.max_age_days` | `int` | `7` | Max days to keep old logs |
| `file.max_backups` | `int` | `5` | Max rotated files to keep |
| `file.compress` | `bool` | `true` | Gzip rotated files |

### OTEL Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `otel.enabled` | `bool` | `false` | Enable OTEL export |
| `otel.endpoint` | `string` | `""` | Collector address |
| `otel.protocol` | `string` | `grpc` | `grpc` or `http` |
| `otel.insecure` | `bool` | `false` | Disable TLS |
| `otel.username` | `string` | `""` | Basic Auth username |
| `otel.password` | `string` | `""` | Basic Auth password |
| `otel.headers` | `map` | `{}` | Additional headers |
| `otel.timeout` | `duration` | `10s` | Export timeout |
| `otel.batch_size` | `int` | `512` | Logs per batch |
| `otel.export_interval` | `duration` | `5s` | Batch flush interval |
| `otel.attributes` | `map` | `{}` | Resource attributes |

---

## Usage Patterns for Teams

### 1. Dependency Injection (Preferred)

Pass `ion.Logger` explicitly to components. This makes testing easier and dependencies clear.

```go
type Server struct {
    log ion.Logger
}

func NewServer(l ion.Logger) *Server {
    return &Server{
        log: l.Named("server").With(ion.String("component", "http")),
    }
}
```

### 2. Global Singleton (Legacy/Scripts)

For existing codebases or simple scripts where DI is impractical:

```go
// In main.go
ion.SetGlobal(logger)

// In any package
func DoWork(ctx context.Context) {
    ion.L().Info(ctx, "doing work")
}
```

### 3. Hot Reloading Pattern

Start with a basic logger, then upgrade after loading configuration:

```go
func main() {
    ctx := context.Background()

    // 1. Start with safe defaults
    ion.SetGlobal(ion.New(ion.Default()))

    // 2. Load configuration
    appConfig := LoadConfig()

    // 3. Initialize production logger
    prodLogger, err := ion.NewWithOTEL(appConfig.Log)
    if err != nil {
        ion.Fatal(ctx, "failed to init logger", err)
    }

    // 4. Replace global logger
    ion.SetGlobal(prodLogger)
    defer prodLogger.Shutdown(ctx)
    
    // 5. Run application
    RunApp(ctx)
}
```

---

## Lifecycle Management

### Graceful Shutdown

Always flush before exit to prevent data loss:

```go
logger, _ := ion.NewWithOTEL(cfg)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
defer logger.Shutdown(ctx) // Flushes OTEL + local buffers
```

**Notes:**
- `Sync()` flushes only local buffers
- `Shutdown(ctx)` flushes OTEL exporters AND local buffers
- Always use `Shutdown` for OTEL-enabled loggers

### Runtime Level Changes

Change log level without restart (thread-safe):

```go
logger.SetLevel("debug")
// Later...
logger.SetLevel("info")
```

---

## Performance

Ion is optimized for minimal allocations:

| Scenario | Allocations | Latency |
|----------|-------------|---------|
| Background context | 1 alloc | ~50ns |
| With trace context | 2 allocs | ~170ns |
| Field: `ion.String()` | 0 allocs | — |
| Field: `ion.F()` | 1 alloc | — |

**Best Practices:**
1. Use typed field constructors (`ion.String`, `ion.Int`) instead of `ion.F`
2. Use `context.Background()` for startup logs (skips trace extraction)

---

## Blockchain Field Helpers

Import specialized helpers for consistent field naming:

```go
import "github.com/JupiterMetaLabs/ion/fields"

logger.Info(ctx, "transaction routed",
    fields.TxHash("abc123"),
    fields.ShardID(5),
    fields.LatencyMs(12.5),
    fields.Slot(12345678),
)
```

---

## Architecture

Ion wraps `zap.Logger` with a custom Core for OTEL integration:

-   **Console/File**: Handled directly by Zap (minimal overhead).
-   **OTEL**: Asynchronous batch processor (non-blocking).

---

## License

[MIT](LICENSE) © 2025 JupiterMeta Labs
