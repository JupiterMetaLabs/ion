# Ion

**Ion** is an enterprise-grade, structured logging library designed for high-performance blockchain applications at [JupiterMeta Labs](https://github.com/JupiterMetaLabs).

It combines the raw speed of [Zap](https://github.com/uber-go/zap) with seamless [OpenTelemetry (OTEL)](https://opentelemetry.io/) integration, ensuring your logs are both fast and observable.

## Features

-   🚀 **High Performance**: Built on Uber's Zap with a custom **Zero-Allocation** core for hot paths.
-   🔭 **OpenTelemetry Native**: Seamless integration with OTEL for distributed tracing (Logs + Traces).
-   🛡️ **Enterprise Grade**: Reliable `Shutdown` hooks, internal pooling, and safe concurrency patterns.
-   🔗 **Blockchain Ready**: Specialized field helpers for `TxHash`, `Slot`, `ShardID`, and more.
-   📝 **Developer Friendly**: Pretty console output for local dev, JSON for production.
-   🔄 **Lumberjack**: Built-in file rotation and compression.

## Installation

```bash
go get github.com/JupiterMetaLabs/ion
```

## Usage Patterns

### 1. The Singleton Pattern (Recommended for Apps)
For most applications, setting a global logger instance is the most convenient approach. It allows you to log from any package without passing a logger variable through every function signature.

```go
func main() {
    // Automatically configure from env vars (LOG_LEVEL, SERVICE_NAME, OTEL_ENDPOINT)
    ion.SetGlobal(ion.InitFromEnv())
    defer ion.Sync()

    ion.Info("starting application")
    RunServer()
}

func RunServer() {
    // Use the package-level helpers
    ion.Info("server listening on :8080")
}
```

### 2. The Dependency Injection Pattern (Recommended for Libraries)
If you are building a library or prefer explicit dependencies, you can create and pass the `ion.Logger` interface.

```go
type Server struct {
    logger ion.Logger
}

func NewServer(l ion.Logger) *Server {
    return &Server{logger: l}
}

func (s *Server) Start() {
    s.logger.Info("server started")
}
```

## Advanced Features

### Child Loggers (`With` and `Named`)
You can create scoped "child" loggers that inherit configuration but add specific context.

*   **`With`**: Adds permanent fields to every log entry from that child.
*   **`Named`**: Adds a "logger" name (usually used to identify a component/module).

```go
// Create a sub-logger for the "database" component
dbLogger := logger.Named("db").With(ion.String("db_name", "orders"))

dbLogger.Info("connection established") 
// Output: {"level":"info", "logger":"db", "db_name":"orders", "msg":"..."}
```

### Context Integration (`WithContext`)
Ion integrates deeply with `context.Context` to extract OpenTelemetry `trace_id` and `span_id` automatically.

```go
func HandleRequest(ctx context.Context) {
    // Creates a child logger that automatically extracts trace information from ctx
    l := ion.WithContext(ctx)
    
    l.Info("processing request")
    // Output will include "trace_id" and "span_id" if they exist in ctx
}
```

### With Context (Tracing)

Ion automatically correlates logs with distributed traces if a context is provided.

```go
func HandleRequest(ctx context.Context) {
    // If ctx contains OTEL trace info, it will be added to the log
    logger.WithContext(ctx).Info("processing transaction",
        ion.F("user_id", "u_12345"),
    )
}
```

### Configuration

Ion uses a strongly typed `Config` struct. You can load it from code or environment variables.

```go
cfg := ion.Default()

// Override with env vars or manual settings
cfg.Level = "debug"
cfg.ServiceName = "payment-service"
cfg.OTEL.Enabled = true
cfg.OTEL.Endpoint = "otel-collector:4317"

logger := ion.New(cfg)
```

### Production Configuration

For enterprise deployments, you should utilize the full configuration power (Files, Rotation, OTEL):

```go
cfg := ion.Config{
    Level:       "info",
    Development: false,      // JSON format
    ServiceName: "payment-service",
    
    // File Logging with Rotation (Lumberjack)
    File: &ion.FileConfig{
        Enabled:    true,
        Path:       "/var/log/app/service.log",
        MaxSize:    100, // MB
        MaxBackups: 10,
        Compress:   true,
    },

    // OpenTelemetry Export
    OTEL: &ion.OTELConfig{
        Enabled:  true,
        Endpoint: "otel-collector:4317", // gRPC by default
        Protocol: "grpc",
        Username: "admin",        // Optional Basic Auth
        Password: "supersecret",  // Optional Basic Auth
        Headers: map[string]string{ // Optional custom headers
            "X-Custom-Token": "value",
        },
        BatchSize: 1000,
        Attributes: map[string]string{
            "env": "production",
        },
    },
}
logger, _ := ion.NewWithOTEL(cfg)
defer logger.Shutdown(ctx)
```

## 🏗️ Recommended Usage for Teams

### 1. Dependency Injection (Preferred)
Pass the `ion.Logger` explicitly to your components. This makes testing easier and dependencies clear.

```go
type Server struct {
    log ion.Logger
}

func NewServer(l ion.Logger) *Server {
    return &Server{
        log: l.With("component", "server"), // Attach context once
    }
}
```

### 2. Global Singleton (Refactoring / Scripts)
For legacy codebases or simple scripts where passing the logger is difficult, use the global singleton.

```go
// In main.go
ion.SetGlobal(logger)

// In package foo
func Bar() {
    // Uses the globally configured logger
    ion.L().Info("something happened")
}
```

## 🚀 Performance Tips

Ion is designed for high-performance zero-allocation logging. To maximize speed:

1.  **Use Typed Constructors**: Always use `ion.Int`, `ion.String`, `ion.Bool` instead of `ion.F`.
    *   `ion.String("key", "val")` -> **0 allocations**
    *   `ion.F("key", "val")` -> **1 allocation** (boxing to `any`)
### 3. Lifecycle & Runtime Configuration

#### Graceful Shutdown
Always ensure you flush logs before your application exits to prevent data loss (especially for OTEL traces).

```go
// In main()
logger, _ := ion.NewWithOTEL(cfg)
// Use a timeout context to ensure we don't hang if OTEL collector is down
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
defer logger.Shutdown(ctx)
```

#### Runtime Configuration (Dynamic Level)
Ion supports changing the log level at runtime without restarting the application. This is thread-safe.

```go
// Safe to call concurrently
logger.SetLevel("debug") 
// Later...
logger.SetLevel("info")
```

#### Hot Reloading (Initialization Pattern)
A common pattern for applications is to start with a basic logger, load configuration, and then upgrade to a production logger.

```go
func main() {
    // 1. Start with a safe default (Console, Info)
    ion.SetGlobal(ion.New(ion.Default()))

    // 2. Load Configuration (e.g., from YAML or Env)
    appConfig := LoadConfig()

    // 3. Initialize Production Logger (File, OTEL, etc.)
    prodLogger, err := ion.NewWithOTEL(appConfig.Log)
    if err != nil {
        ion.L().Fatal("failed to init logger", err)
    }

    // 4. Replace Global Logger
    ion.SetGlobal(prodLogger)
    
    // 5. Run App
    RunApp(ion.L())
}
```

## 🚀 Performance Tips

Ion is designed for high-performance zero-allocation logging. To maximize speed:

1.  **Use Typed Constructors**: Always use `ion.Int`, `ion.String`, `ion.Bool` instead of `ion.F`.
    *   `ion.String("key", "val")` -> **0 allocations**
    *   `ion.F("key", "val")` -> **1 allocation** (boxing to `any`)
2.  **Sync vs Shutdown**: `logger.Sync()` only flushes the local buffer. `logger.Shutdown(ctx)` flushes **everything** (including OTEL) and closes connections. Use `Shutdown` on exit.

The `Config` struct maps to YAML/JSON and Environment Variables.

| Field | Env Var | Default | Description |
|-------|---------|---------|-------------|
| `level` | `LOG_LEVEL` | `info` | Minimum log level (`debug`, `info`, `warn`, `error`). |
| `development` | `LOG_DEVELOPMENT` | `false` | Enables pretty printing, stack traces, and caller info. |
| `service_name` | `SERVICE_NAME` | `unknown` | Name of the service for OTEL traces. |
| `version` | `SERVICE_VERSION` | `""` | Service version for OTEL. |
| **Console** | | | |
| `console.enabled` | - | `true` | Enable stdout/stderr output. |
| `console.format` | - | `json` | `json` or `pretty`. |
| `console.color` | - | `true` | Enable ANSI colors in `pretty` mode. |
| `console.errors_to_stderr` | - | `true` | Send warn/error/fatal to stderr. |
| **File** | | | |
| `file.enabled` | - | `false` | Enable writing to a log file. |
| `file.path` | - | `""` | Absolute path to the log file. |
| `file.max_size_mb` | - | `100` | Rotate after N megabytes. |
| `file.max_backups` | - | `5` | Keep N old files. |
| `file.max_age_days` | - | `7` | Keep files for N days. |
| `file.compress` | - | `true` | Gzip rotated files. |
| **OTEL** | | | |
| `otel.enabled` | - | `false` | Enable OpenTelemetry export. |
| `otel.endpoint` | `OTEL_ENDPOINT`* | `""` | Collector address (e.g., `localhost:4317`). |
| `otel.protocol` | - | `grpc` | `grpc` or `http`. |
| `otel.insecure` | - | `false` | Disable TLS (use for local collectors). |
| `otel.username` | `OTEL_USERNAME` | `""` | Basic Auth username (optional). |
| `otel.password` | `OTEL_PASSWORD` | `""` | Basic Auth password (optional). |
| `otel.headers` | - | `{}` | Custom headers (map, for Bearer tokens, etc.). |
| `otel.timeout` | - | `10s` | Export timeout. |
| `otel.batch_size` | - | `512` | Max logs per batch. |
| `otel.export_interval`| - | `5s` | Flush interval. |
| `otel.attributes` | - | `{}` | Extra resource attributes (map). |

> *`OTEL_ENDPOINT` is only read by `InitFromEnv()`, not automatically by the struct.




## Architecture

Ion is designed as a wrapper around `zap.Logger`. It injects a custom Core for OTEL integration that does not impede the performance of local logging.

-   **Console/File**: Handled directly by Zap.
-   **OTEL**: Handled by an asynchronous batch processor to avoid blocking the application.

## License

[MIT](LICENSE) © 2025 JupiterMeta Labs
