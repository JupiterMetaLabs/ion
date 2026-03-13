# Ion Observability Reference Specification

This document defines the "Reference Backend" architectural requirements for an Ion-standard observability stack. It focuses on maintaining the **Semantic Fidelity** of telemetry produced by the Ion library.

## 1. The Ion Contract

Ion is not a collection of wrappers; it is an **instrumentation contract**. When a service uses Ion, it guarantees:
1.  **Strict Context Propagation**: Every log and span carries the same `trace_id` and `span_id`.
2.  **Resource Identity**: Every record is tagged with immutable resource attributes (`service.name`, `service.version`).
3.  **Attribute Richness**: Business context (high-cardinality data) is stored in structured attributes, not as string-formatted log bodies.

A backend that "greps" text or siloed data fails this contract.

---

## 2. Architectural Invariants

A "Reference Backend" must satisfy these three technical invariants:

### I. Unified Event Model (Log-Trace Entanglement)
The backend must treat logs and traces as two facets of a single event stream. 
- **Requirement**: The storage layer must index `trace_id` globally.
- **Validation**: A request for `trace_id=X` must return all spans AND all log entries associated with that trace in a single search operation.

### II. High-Cardinality Performance
Ion emits high-cardinality metadata (`user_id`, `node_id`, `order_id`).
- **Requirement**: The backend must use a **Columnar Storage** engine (e.g., ClickHouse) or a **Wide-Event Store** (e.g., Honeycomb).
- **Non-Goal**: Avoid "Label-heavy" databases (like legacy Prometheus/Loki) that suffer from index explosions when dealing with unique IDs.

### III. Contextual Navigation (One-Click Handover)
The UI must provide a bi-directional "Context Bridge."
- **Log → Trace**: Clicking a `trace_id` in a log must render the trace waterfall.
- **Trace → Log**: Clicking a span in the waterfall must filter logs to that specific `span_id`.

---

## 3. Recommended Backends

| Tier | Backend Stack | Rationale |
| :--- | :--- | :--- |
| **Tier 1 (SaaS)** | **Honeycomb** | **The Reference Reference.** Honeycomb's event-centric model perfectly matches Ion's Wide-Event philosophy. Zero-config correlation. |
| **Tier 2 (OSS)** | **SigNoz / HyperDX** | **Performance Choice.** Built on ClickHouse, these solve the high-cardinality "wiring" pain of self-hosted Loki/Jaeger. |
| **Tier 3 (Legacy)**| **Loki + Tempo** | **The Maintenance Choice.** Requires manual field bridging (Correlations/Derived Fields) but fits into existing Grafana ecosystems. |

---

## 4. Reference Collector Configuration

The **OpenTelemetry Collector** is the "Enforcement Point" for the Ion contract. It ensures that Ion’s OTLP gRPC stream is correctly batched and routed.

```yaml
# /etc/otel-collector/config.yaml
receivers:
  otlp/ion:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317 # Primary entry for Ion services

processors:
  batch:
    send_batch_size: 1000
    timeout: 10s
    
  # Standardize labels and resource attributes
  resource:
    attributes:
      - key: ion.managed
        value: "true"
        action: insert

exporters:
  # Example: Direct OTLP export to a Tier 2 backend
  otlp/backend:
    endpoint: "signoz-otel-collector:4317"
    tls: { insecure: true }

service:
  pipelines:
    traces:
      receivers: [otlp/ion]
      processors: [batch, resource]
      exporters: [otlp/backend]
    logs:
      receivers: [otlp/ion]
      processors: [batch, resource]
      exporters: [otlp/backend]
```

---

## 5. Certification Checklist

A deployment is "Ion-Certified" only if it satisfies this developer workflow:

1.  **The "No-Copies" Rule**: A developer never has to copy a Trace ID from one tab and paste it into another.
2.  **The "Drill-Down" Rule**: Can you filter for a specific `user_id` across 1 billion logs in less than 5 seconds?
3.  **The "Consistency" Rule**: Does every log show the `service.version` automatically, preventing "ghost debugging" of old code?

---
