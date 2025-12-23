# OTEL Setup Guide for ION

This guide shows how to properly set up OpenTelemetry tracing with ION so that `trace_id` and `span_id` are automatically included in all logs.

## Architecture Overview

```
┌────────────────────────────────────────────────────────────────┐
│                        Your Service                            │
│                                                                │
│  ┌──────────────┐    ┌─────────────┐    ┌──────────────────┐  │
│  │ HTTP Handler │───►│   Context   │───►│   ION Logger     │  │
│  │ (otelhttp)   │    │ (trace_id)  │    │ (auto-extracts)  │  │
│  └──────────────┘    └─────────────┘    └──────────────────┘  │
│                                                                │
│  ┌──────────────┐    ┌─────────────┐    ┌──────────────────┐  │
│  │ gRPC Server  │───►│   Context   │───►│   ION Logger     │  │
│  │ (otelgrpc)   │    │ (trace_id)  │    │ (auto-extracts)  │  │
│  └──────────────┘    └─────────────┘    └──────────────────┘  │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  OTEL Collector │
                    │  (or Jaeger)    │
                    └─────────────────┘
```

## Quick Start

### 1. Dependencies

```bash
go get github.com/JupiterMetaLabs/ion@latest
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
```

### 2. Main Setup

```go
package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/JupiterMetaLabs/ion"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
    ctx := context.Background()

    // ─────────────────────────────────────────────────────────────
    // 1. Initialize ION with OTEL
    // ─────────────────────────────────────────────────────────────
    cfg := ion.Config{
        Level:       "info",
        ServiceName: "orchestrator",
        Version:     "1.0.0",
        Console: ion.ConsoleConfig{
            Enabled: true,
            Format:  "json",
        },
        OTEL: ion.OTELConfig{
            Enabled:  true,
            Endpoint: os.Getenv("OTEL_ENDPOINT"), // e.g., "jaeger:4317"
            Protocol: "grpc",
            Insecure: true, // Set false in production with TLS
        },
    }

    logger, err := ion.NewWithOTEL(cfg)
    if err != nil {
        panic("failed to init logger: " + err.Error())
    }
    ion.SetGlobal(logger)
    defer logger.Shutdown(ctx)

    // Startup log - no trace (context.Background)
    ion.Info(ctx, "orchestrator starting",
        ion.String("version", cfg.Version),
    )

    // ─────────────────────────────────────────────────────────────
    // 2. Setup HTTP Server with OTEL instrumentation
    // ─────────────────────────────────────────────────────────────
    mux := http.NewServeMux()
    
    // Each handler gets auto-traced
    mux.HandleFunc("/health", healthHandler)
    mux.HandleFunc("/api/v1/route", routeHandler)
    mux.HandleFunc("/api/v1/submit", submitHandler)

    // Wrap entire mux with OTEL - this creates/propagates trace IDs
    handler := otelhttp.NewHandler(mux, "orchestrator-http",
        otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
    )

    server := &http.Server{
        Addr:    ":8080",
        Handler: handler,
    }

    // ─────────────────────────────────────────────────────────────
    // 3. Graceful shutdown
    // ─────────────────────────────────────────────────────────────
    go func() {
        ion.Info(ctx, "HTTP server listening", ion.Int("port", 8080))
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            ion.Error(ctx, "server error", err)
        }
    }()

    // Wait for interrupt
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    ion.Info(ctx, "shutting down...")
    shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    server.Shutdown(shutdownCtx)
}
```

### 3. HTTP Handlers (trace_id automatically included)

```go
package main

import (
    "net/http"

    "github.com/JupiterMetaLabs/ion"
    "github.com/JupiterMetaLabs/ion/fields"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    // trace_id is AUTOMATICALLY in r.Context() from otelhttp middleware
    ctx := r.Context()
    
    ion.Debug(ctx, "health check") // Will include trace_id!
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
}

func routeHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    txHash := r.URL.Query().Get("tx")
    
    // All logs automatically include trace_id + span_id
    ion.Info(ctx, "routing transaction",
        fields.TxHash(txHash),
    )
    
    // Call other services - trace propagates
    result, err := routeToShard(ctx, txHash)
    if err != nil {
        ion.Error(ctx, "routing failed", err, fields.TxHash(txHash))
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    ion.Info(ctx, "transaction routed",
        fields.TxHash(txHash),
        fields.ShardID(result.ShardID),
        fields.LatencyMs(result.LatencyMs),
    )
    
    w.WriteHeader(http.StatusOK)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Start a child span for specific operations
    tracer := otel.Tracer("orchestrator.submit")
    ctx, span := tracer.Start(ctx, "validate-transaction")
    defer span.End()
    
    ion.Info(ctx, "validating submission") // Has trace_id + new span_id
    
    // ... validation logic
}
```

### 4. gRPC Server Setup

```go
package main

import (
    "net"

    "github.com/JupiterMetaLabs/ion"
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
    "google.golang.org/grpc"
)

func setupGRPCServer() *grpc.Server {
    // Create server with OTEL interceptors
    server := grpc.NewServer(
        grpc.StatsHandler(otelgrpc.NewServerHandler()),
    )
    
    // Register your services
    // pb.RegisterOrchestratorServer(server, &orchestratorServer{})
    
    return server
}

// In your gRPC handler implementation:
type orchestratorServer struct {
    logger ion.Logger
}

func (s *orchestratorServer) RouteTransaction(ctx context.Context, req *pb.RouteRequest) (*pb.RouteResponse, error) {
    // ctx automatically has trace_id from otelgrpc interceptor!
    s.logger.Info(ctx, "received route request",
        fields.TxHash(req.TxHash),
    )
    
    // ... your logic
    
    return &pb.RouteResponse{ShardId: 5}, nil
}
```

### 5. Outgoing HTTP Calls (Propagate Trace)

```go
import (
    "net/http"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Create a traced HTTP client
var httpClient = &http.Client{
    Transport: otelhttp.NewTransport(http.DefaultTransport),
}

func callExternalService(ctx context.Context, url string) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    
    // trace_id is automatically added to headers (W3C traceparent)
    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### 6. Background Jobs (Manual Trace Creation)

```go
import "go.opentelemetry.io/otel"

func ProcessBackgroundJob(jobID string) {
    // No incoming request = no trace yet, create one
    tracer := otel.Tracer("orchestrator.jobs")
    ctx, span := tracer.Start(context.Background(), "process-job",
        trace.WithAttributes(
            attribute.String("job.id", jobID),
        ),
    )
    defer span.End()
    
    // Now all logs have trace_id
    ion.Info(ctx, "processing background job",
        ion.String("job_id", jobID),
    )
    
    // Child operations get child spans
    processStep1(ctx)
    processStep2(ctx)
}

func processStep1(ctx context.Context) {
    tracer := otel.Tracer("orchestrator.jobs")
    ctx, span := tracer.Start(ctx, "step-1")
    defer span.End()
    
    ion.Debug(ctx, "executing step 1") // Same trace_id, new span_id
}
```

## Environment Variables

For production, use environment-based configuration:

```bash
# Required
export SERVICE_NAME=orchestrator
export OTEL_ENDPOINT=jaeger:4317

# Optional
export LOG_LEVEL=info
export SERVICE_VERSION=1.0.0
export OTEL_INSECURE=true  # false for production
export OTEL_USERNAME=admin
export OTEL_PASSWORD=secret
```

Then use:
```go
logger := ion.InitFromEnv()
ion.SetGlobal(logger)
```

## Jaeger Configuration

If you're using Jaeger on GCP, your OTEL endpoint is the Jaeger collector:

```yaml
# docker-compose.yml example
services:
  jaeger:
    image: jaegertracing/all-in-one:1.51
    ports:
      - "16686:16686"  # UI
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    environment:
      - COLLECTOR_OTLP_ENABLED=true
```

```go
cfg.OTEL.Endpoint = "jaeger:4317"
cfg.OTEL.Protocol = "grpc"
cfg.OTEL.Insecure = true
```

## Verifying It Works

1. **Check logs for trace_id:**
```json
{
  "level": "info",
  "msg": "routing transaction",
  "service": "orchestrator",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "tx_hash": "abc123"
}
```

2. **View in Jaeger UI:**
   - Open `http://jaeger:16686`
   - Search traces by service name "orchestrator"
   - Click a trace to see the full span tree

3. **Correlate logs and traces:**
   - Copy `trace_id` from Jaeger
   - Search logs: `{service_name="orchestrator"} | json | trace_id="4bf92f3577b34da6"`
