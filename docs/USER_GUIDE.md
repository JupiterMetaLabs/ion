# Ion Enterprise Usage Guide

This guide details how to use Ion to build observable, high-reliability systems. It moves beyond "how to call the API" into "how to operate the telemetry."

---

## 1. Philosophy: The Two pillars

Ion is opinionated. It treats Logs and Traces as distinct but interlocked signals.

| Signal | Purpose | Semantics | Target Audience |
|--------|---------|-----------|-----------------|
| **Logs** | "What happened?" | Discrete Events. High detail. 100% Reliability. | Humans (Debugging), Security Audits. |
| **Traces** | "Where & How Long?" | Causal Chains. Sampled data. 99% Reliability. | SREs (Latency Analysis), Capacity Planners. |

**The Golden Rule**: *Logs give you the error details; Traces tell you which upstream service caused it.*

---

## 2. The Logger In-Depth

### 2.1 The Severity Contract
Ion's log levels have strict semantic meanings. Misusing them trips alerts.

*   **`Info` (Default)**: "System is working."
    *   *Use for*: Startup config, periodic jobs finishing, significant state changes (e.g., "Order Placed").
    *   *Frequency*: 1-100 per request is fine.
*   **`Warn`**: "System is degraded but functioning."
    *   *Use for*: Retries, deprecated API usage, fallback defaults triggered, slow queries (soft timeout).
    *   *Action*: No immediate paging, but shows up on "Health" dashboards.
*   **`Error`**: "A request failed."
    *   *Use for*: DB unavailable, validation failure, panic recovery.
    *   *Action*: **Pages the on-call** if error rate spikes > 1%.
*   **`Critical`**: "Data integrity at risk."
    *   *Use for*: WAL corruption, split-brain detected, invalid configuration that prevents logic.
    *   *Action*: **Immediate Page**. (Note: Ion guarantees `Critical` does NOT crash the process, unlike `std/log.Fatal`).

### 2.2 Performance & Allocations
Ion is optimized for "hot paths." To maintain zero-allocation:

*   ✅ **DO**: Use strongly typed fields.
    ```go
    // Zero allocation (uses stack or pool)
    app.Info(ctx, "processed", ion.String("id", id), ion.Int("count", 5))
    ```
*   ❌ **DON'T**: Use `fmt.Sprintf` or `Sugar`.
    ```go
    // Allocates string, allocates interface{}, escape analysis fails
    app.Info(ctx, fmt.Sprintf("processed %s", id)) 
    ```

### 2.3 Contextual Logging (Scopes)
In large apps, passing 20 fields manually is unreadable. Use **Derived Loggers**.

```go
// In your HTTP Middleware
func RequestIDMiddleware(next http.Handler, root ion.Logger) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Create a logger that ALWAYS has request_id
        reqID := uuid.New()
        reqLog := root.With(ion.String("req_id", reqID))
        
        // Pass it down via Context (optional, if using context extraction)
        // Or inject it into your service struct for this request scope.
        next.ServeHTTP(w, r)
    })
}
```

---

## 3. The Tracer In-Depth

### 3.1 The "Active Span" Mentality
Tracing is about **Scope**. When you `Start` a span, you are defining a scope of work.

*   **Granularity**: Do not trace every function call. Trace **Network Boundaries** (DB, Cache, External API) and **Major Logic Blocks** (Calculation, Serialization).
    *   *Too much*: `span "AddInts"` (Overhead > Work)
    *   *Just right*: `span "CalculateRiskScore"` (Complex logic)

### 3.2 Cardinality & Attributes
Spans are indexed. High cardinality kills backend performance.

*   ✅ **Safe Attributes**: `http.status_code` (50 values), `region` (10 values), `customer_tier` (3 values).
*   ❌ **Dangerous Attributes**: `error_message` (infinite), `stack_trace` (huge), `user_input` (security risk).

**Tip**: Put the *Category* of error in the Attribute, and the *Detail* in the Log.

### 3.3 Status Codes vs Recording Errors
In OpenTelemetry, "Recording an Error" and "Failing the Span" are separate.

1.  `span.RecordError(err)`: Adds an event `"exception"` with the error stack.
    *   *Result*: You can see the error in the trace UI timeline.
    *   *Status*: Span is still "Unset" (Gray).
2.  `span.SetStatus(codes.Error, msg)`: Marks the span as failed.
    *   *Result*: Error Rate metrics increment. Span turns RED.

**Enterprise Policy**: You **MUST** do both for actual failures.

---

## 4. Together: The Power of Correlation

Ion automates the hardest part: **linking logs to traces**.

### 4.1 How it works
When you pass `context.Context` to `app.Info(ctx, ...)`:
1.  Ion checks if a Trace is active in `ctx`.
2.  If yes, it extracts `TraceID` and `SpanID`.
3.  It injects them as fields `trace_id` and `span_id` into the structured log.

### 4.2 The Debugging Workflow
1.  **Alert fires**: "High Error Rate on PaymentService".
2.  **Dashboard**: You see a spike in 500s.
3.  **Logs**: You query `service="payment" level="error"`.
4.  **Correlation**: You find a periodic error: "Connection Timeout".
5.  **Trace Lookup**: You copy the `trace_id` from that log into Jaeger.
6.  **Root Cause**: The trace shows the `payment-db` was responding in 2.1s, but your timeout is 2.0s.

### 4.3 Async Context Propagation
This is the #1 source of broken traces.

**Scenario**: You spawn a goroutine to send an email after response.
*   *Wrong*: passing `ctx`. The handler cancels it -> Trace breaks.
*   *Wrong*: passing `context.Background()`. Trace ID is lost -> New detached trace.
*   *Right*: **Linking**.

```go
func (s *Service) AsyncEmail(reqCtx context.Context) {
    // 1. Capture the "Parent" trace link
    link := trace.LinkFromContext(reqCtx)
    
    go func() {
        // 2. Start a FRESH root trace
        // This ensures the email trace is complete even if request finishes.
        ctx, span := s.tracer.Start(context.Background(), "SendEmail", trace.WithLinks(link))
        defer span.End()
        
        // 3. Work...
        s.mailer.Send(ctx)
    }()
}
```

---


---

## 5. Real World Patterns: Blockchain & Distributed Systems

This section demonstrates how `ion` flows across high-throughput nodes, P2P layers, and consensus engines.

### 5.1 The "Node Architecture" Pattern (DI + Child Loggers)
In a blockchain node, distinct subsystems (P2P, Consensus, Mempool) need distinct log identities.

```go
// main.go (Node Entrypoint)
root := ion.New(cfg) // name="ion-node"

// Inject scoped loggers into major components
p2pServer := p2p.NewServer(root.Named("p2p"))        // name="ion-node.p2p"
consensus := engine.New(root.Named("consensus"))     // name="ion-node.consensus"
mempool   := pool.New(root.Named("mempool"))         // name="ion-node.mempool"
```

```go
// p2p/peer.go
type Peer struct {
    log ion.Logger // Will be "ion-node.p2p"
    id  string
}

func (p *Peer) HandleHandshake(ctx context.Context) {
    // Log inherits "ion-node.p2p", adds "peer_id" to THIS log only
    // This allows filtering logs by specific peer across the entire session
    p.log.Info(ctx, "handshake accepted", ion.String("peer_id", p.id))
}
```

### 5.2 Hot-Loop Tracing (Block Validation)
Avoid tracing every transaction in a loop. Trace the **Batch** or the **Block** level.

```go
// consensus/engine.go
func (e *Engine) ProcessProposal(ctx context.Context, block *Block) {
    // 1. Start Scope for the Block (High Level)
    ctx, span := e.tracer.Start(ctx, "ProcessProposal")
    defer span.End() // This span measures total validation time
    
    span.SetAttributes(attribute.Int64("block.height", block.Height))

    // 2. Validate Signatures (Expensive CPU op)
    // We trace this because if it's slow, we miss the slot.
    if err := e.verifySignatures(ctx, block); err != nil {
        span.RecordError(err)
        return
    }
    
    // 3. DB Write (I/O op)
    // Pass CTX so the DB layer (using ion/otel) links its spans to "ProcessProposal"
    if err := e.stateDB.Commit(ctx, block); err != nil {
        span.RecordError(err)
        return
    }
}
```

### 5.3 Gossip & Network Boundaries (gRPC/P2P)
When gossiping blocks between nodes, `TraceID` propagation allows you to visualize propagation delay across the network.

**Scenario**: Node A sends a block to Node B via gRPC.

```go
// Node A (Sender)
func (n *Node) GossipBlock(ctx context.Context, peer Client, block *Block) {
    // 1. Start "Gossip" Span
    ctx, span := n.tracer.Start(ctx, "GossipBlock")
    defer span.End()
    
    // 2. gRPC Interceptor (iongrpc) AUTOMATICALLY injects headers
    // You just pass the context.
    peer.SendBlock(ctx, block)
}
```

```go
// Node B (Receiver)
func (s *GRPCServer) SendBlock(ctx context.Context, req *SendBlockReq) (*Ack, error) {
    // 1. gRPC Interceptor (iongrpc) AUTOMATICALLY extracts headers
    // and starts a child span.
    
    // 2. We can see the "Network Lag" by comparing Parent Start Time (Node A)
    // vs Child Start Time (Node B) in Jaeger.
    
    s.log.Info(ctx, "block received", ion.Int64("height", req.Block.Height))
    return &Ack{}, nil
}
```

