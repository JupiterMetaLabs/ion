# Ion Tracing Quickstart

A practical guide to distributed tracing with Ion. Learn how to create spans, propagate context, and debug production issues.

---

## What is a Span?

A **Span** represents a single unit of work in your application. Think of it as a "timer with context."

```
[Request]────────────────────────────────────────────────────────────>
          [ProcessOrder]────────────────────────>
                        [ValidatePayment]───────>
                                        [WriteDB]───>
```

Each box above is a span. Together, they form a **Trace** — the complete picture of a request's journey through your system.

---

## The Span Interface

Ion provides a clean `Span` interface that wraps OpenTelemetry:

```go
type Span interface {
    // End marks the span as complete. MUST be called.
    End()
    
    // SetStatus sets the span status (Ok, Error, or Unset).
    SetStatus(code codes.Code, description string)
    
    // RecordError records an error as an event on the span timeline.
    RecordError(err error)
    
    // SetAttributes adds key-value metadata for filtering/searching.
    SetAttributes(attrs ...attribute.KeyValue)
    
    // AddEvent adds a timestamped event to the span.
    AddEvent(name string, attrs ...attribute.KeyValue)
}
```

---

## Quick Start

### 1. Basic Span Creation

Every span follows this pattern: **Start → Work → End**

```go
import (
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
)

func ProcessOrder(ctx context.Context, orderID string) error {
    // 1. Get a named tracer
    tracer := app.Tracer("order.processor")
    
    // 2. Start a span (returns enriched context + span)
    ctx, span := tracer.Start(ctx, "ProcessOrder")
    
    // 3. ALWAYS defer End() immediately after Start()
    defer span.End()
    
    // 4. Add attributes for filtering in Jaeger/Tempo
    span.SetAttributes(attribute.String("order.id", orderID))
    
    // 5. Do work...
    if err := validateOrder(ctx, orderID); err != nil {
        // 6. Record errors properly
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        return err
    }
    
    // 7. Mark success (optional, default is Unset/Ok)
    span.SetStatus(codes.Ok, "order processed")
    return nil
}
```

---

## Span Method Reference

### `End()`

Marks the span as complete and records its duration.

```go
ctx, span := tracer.Start(ctx, "MyOperation")
defer span.End() // ← CRITICAL: Always defer immediately
```

> [!CAUTION]
> Forgetting to call `End()` causes memory leaks and broken traces. Always use `defer`.

---

### `SetStatus(code, description)`

Sets the span's final status. This affects error rate metrics and trace visualization.

| Code | When to Use | Visual |
|------|-------------|--------|
| `codes.Unset` | Default, operation outcome unknown | Gray |
| `codes.Ok` | Explicit success | Green |
| `codes.Error` | Operation failed | **Red** |

```go
// On success
span.SetStatus(codes.Ok, "completed")

// On failure
span.SetStatus(codes.Error, "database timeout")
```

> [!IMPORTANT]
> By default, spans finish as `Unset` (success). You **MUST** call `SetStatus(codes.Error, ...)` on failures, or your error rate dashboard will show 0%.

---

### `RecordError(err)`

Records an error as an event on the span timeline. This captures:
- Error message
- Error type
- Stack trace (if available)

```go
if err != nil {
    span.RecordError(err)  // Adds "exception" event to timeline
    span.SetStatus(codes.Error, "operation failed")  // Marks span as failed
    return err
}
```

> [!TIP]
> Always use **both** `RecordError()` AND `SetStatus()` together. `RecordError` adds detail; `SetStatus` triggers alerts.

---

### `SetAttributes(attrs...)`

Adds searchable metadata to the span. Use for **low-cardinality** data that helps you filter traces.

```go
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.Int("retry.count", retryCount),
    attribute.Bool("cache.hit", cacheHit),
)
```

**Good Attributes** (Low Cardinality):
- `http.status_code` (50 values)
- `region` (10 values)
- `customer.tier` (3 values)

**Bad Attributes** (High Cardinality):
- `error.message` (infinite)
- `request.body` (huge)
- `user.email` (PII risk)

---

### `AddEvent(name, attrs...)`

Adds a timestamped marker to the span timeline. Use for significant moments during execution.

```go
// Mark when cache was missed
span.AddEvent("cache_miss", attribute.String("key", cacheKey))

// Mark retry attempts
span.AddEvent("retry_attempt", attribute.Int("attempt", 3))

// Mark important state changes
span.AddEvent("payment_authorized")
```

Events appear as points on the span timeline in Jaeger/Tempo, helping you understand the sequence of operations.

---

## Decision Guide: Spans vs. Events

Understanding when to create a new span versus adding an event is key to clean traces.

| Feature | `tracer.Start(...)` (New Span) | `span.AddEvent(...)` |
| :--- | :--- | :--- |
| **Concept** | **Duration** (Start & End) | **Point in Time** (Snapshot) |
| **Visual** | A bar with length | A dot on the timeline |
| **Best For** | DB Queries, API Calls, Major Functions | Retries, Cache Misses, Valdiation Failures |
| **Overhead** | Higher (Context switching, allocs) | Very Low (Log entry) |

**Rule of Thumb:**
- **"How long did this take?"** → New Span
- **"When did this happen?"** → Add Event

### What about `RecordError`?

`RecordError(err)` is just a specialized **Event**.
*   It adds an event named `"exception"` with the error message and stack trace.
*   **Where to put it?** Record the error on the **active span** where the failure occurred.
*   **Bubble Up:** If a child span fails (e.g., DB call), record the error there. If that error causes the parent operation to fail, record it on the parent span as well.

## Span Creation Options

When starting a span, you can customize its behavior:

### `ion.WithAttributes(attrs...)`

Pre-populate attributes at span creation:

```go
ctx, span := tracer.Start(ctx, "ProcessPayment",
    ion.WithAttributes(
        attribute.String("payment.method", "card"),
        attribute.Float64("amount", 99.99),
    ),
)
```

### `ion.WithSpanKind(kind)`

Set the span's role in the trace:

```go
import "go.opentelemetry.io/otel/trace"

// For incoming requests (HTTP server, gRPC server)
tracer.Start(ctx, "HandleRequest", ion.WithSpanKind(trace.SpanKindServer))

// For outgoing requests (HTTP client, DB calls)
tracer.Start(ctx, "CallExternalAPI", ion.WithSpanKind(trace.SpanKindClient))

// For async message producers
tracer.Start(ctx, "PublishEvent", ion.WithSpanKind(trace.SpanKindProducer))

// For async message consumers
tracer.Start(ctx, "ConsumeEvent", ion.WithSpanKind(trace.SpanKindConsumer))

// Default: internal operations
tracer.Start(ctx, "Calculate", ion.WithSpanKind(trace.SpanKindInternal))
```

### `ion.WithLinks(links...)`

Connect spans that are causally related but not in a parent-child relationship:

```go
// Capture link from parent context
link := ion.LinkFromContext(parentCtx)

// Create new span with link to parent
ctx, span := tracer.Start(context.Background(), "BackgroundJob",
    ion.WithLinks(link),
)
```

---

## Common Patterns

### Pattern 1: The Standard Request Handler

```go
func HandleRequest(ctx context.Context, req *Request) (*Response, error) {
    tracer := app.Tracer("api.handler")
    ctx, span := tracer.Start(ctx, "HandleRequest")
    defer span.End()
    
    // Add request metadata
    span.SetAttributes(
        attribute.String("http.method", req.Method),
        attribute.String("http.path", req.Path),
    )
    
    // Process
    result, err := process(ctx, req)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }
    
    span.SetStatus(codes.Ok, "success")
    return result, nil
}
```

### Pattern 2: Database Operations

```go
func (r *Repository) GetUser(ctx context.Context, id string) (*User, error) {
    tracer := app.Tracer("repository.user")
    ctx, span := tracer.Start(ctx, "GetUser",
        ion.WithSpanKind(trace.SpanKindClient),
    )
    defer span.End()
    
    span.SetAttributes(attribute.String("db.operation", "SELECT"))
    
    user, err := r.db.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "query failed")
        return nil, err
    }
    
    return user, nil
}
```

### Pattern 3: Background Workers (with Links)

When spawning goroutines, use links to preserve trace causality:

```go
func (s *Service) ProcessAsync(ctx context.Context, job *Job) {
    // Capture causality before spawning
    link := ion.LinkFromContext(ctx)
    
    go func() {
        // Create FRESH context (won't be canceled when parent returns)
        newCtx := context.Background()
        tracer := app.Tracer("worker.background")
        
        // Link back to original request
        ctx, span := tracer.Start(newCtx, "ProcessJob",
            ion.WithLinks(link),
        )
        defer span.End()
        
        // Process...
        if err := s.doWork(ctx, job); err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, "job failed")
        }
    }()
}
```

### Pattern 4: Batch Processing

For loops, don't trace every item — trace the batch:

```go
func ProcessBatch(ctx context.Context, items []*Item) error {
    tracer := app.Tracer("batch.processor")
    ctx, span := tracer.Start(ctx, "ProcessBatch")
    defer span.End()
    
    span.SetAttributes(attribute.Int("batch.size", len(items)))
    
    var errors int
    for i, item := range items {
        if err := processItem(ctx, item); err != nil {
            errors++
            // Add event instead of child span
            span.AddEvent("item_failed", 
                attribute.Int("index", i),
                attribute.String("error", err.Error()),
            )
        }
    }
    
    span.SetAttributes(attribute.Int("batch.errors", errors))
    
    if errors > 0 {
        span.SetStatus(codes.Error, fmt.Sprintf("%d items failed", errors))
        return fmt.Errorf("%d items failed", errors)
    }
    
    return nil
}
```

---

## The Error Handling Contract

> [!WARNING]
> This is the #1 source of broken dashboards.

When an error occurs, you must do **three things**:

```go
if err != nil {
    // 1. Record the error details (adds event to timeline)
    span.RecordError(err)
    
    // 2. Mark the span as failed (turns red, affects error rate)
    span.SetStatus(codes.Error, "operation failed")
    
    // 3. Log for human debugging (correlated via trace_id)
    app.Error(ctx, "operation failed", err)
    
    return err
}
```

| Action | What It Does | Where It Shows |
|--------|--------------|----------------|
| `RecordError(err)` | Adds exception event with stack trace | Span timeline |
| `SetStatus(codes.Error, ...)` | Marks span as failed | Red in UI, Error Rate metrics |
| `app.Error(ctx, ...)` | Logs with trace correlation | Loki, searching by trace_id |

---

## Visualization

Once configured correctly, your traces will appear in Jaeger/Tempo like this:

```
ion-service                                                    12.5ms
├── HandleRequest [http.method=POST, http.path=/orders]        12.0ms
│   ├── ValidateOrder                                           2.1ms
│   ├── ProcessPayment [payment.method=card]                    5.2ms
│   │   └── CallPaymentGateway [SpanKindClient]                 4.8ms
│   └── SaveOrder                                               3.1ms
│       └── PostgreSQL.INSERT [SpanKindClient]                  2.9ms
```

Each span shows:
- Duration (right side)
- Attributes (in brackets)
- Span kind (for client/server relationships)
- Events (as points on the timeline)
- Errors (red highlighting)

---

## Configuration

Enable tracing in your Ion config:

```go
cfg := ion.Config{
    ServiceName: "my-service",
    
    Tracing: ion.TracingConfig{
        Enabled:  true,
        Endpoint: "localhost:4317",      // OTEL Collector address
        Protocol: "grpc",                // or "http"
        Sampler:  "always",              // or "ratio:0.1" for 10%
    },
}

app, _, err := ion.New(cfg)
```

---

## Best Practices Checklist

- [ ] **Always `defer span.End()`** immediately after `tracer.Start()`
- [ ] **Always call both** `RecordError()` and `SetStatus(codes.Error, ...)` on failures
- [ ] **Use low-cardinality attributes** — avoid putting error messages or dynamic IDs as attribute values
- [ ] **Pass context everywhere** — broken context = broken traces
- [ ] **Use Links for goroutines** — prevents ghost spans and preserves causality
- [ ] **Trace boundaries, not everything** — DB calls, HTTP requests, major logic blocks
- [ ] **Name spans descriptively** — `ProcessOrder` not `DoStuff`
- [ ] **Call `app.Shutdown(ctx)`** on exit — flushes all buffered traces

---

## Quick Reference

```go
// Get tracer
tracer := app.Tracer("component.name")

// Start span
ctx, span := tracer.Start(ctx, "OperationName")
defer span.End()

// Add attributes
span.SetAttributes(attribute.String("key", "value"))

// Add event
span.AddEvent("event_name", attribute.Int("count", 5))

// Record error
span.RecordError(err)
span.SetStatus(codes.Error, "failed")

// Mark success
span.SetStatus(codes.Ok, "success")

// Link spans (for goroutines)
link := ion.LinkFromContext(parentCtx)
ctx, span := tracer.Start(context.Background(), "AsyncOp", ion.WithLinks(link))
```
