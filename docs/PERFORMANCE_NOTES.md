# Ion Performance Optimization Notes

This document captures performance analysis and trade-off decisions for future reference.

---

## Context Field Slice Pooling (Considered, Not Implemented)

**Date**: 2025-12-22  
**Status**: Deferred  

### Problem

The `extractContextZapFields` function allocates a new slice on every log call:

```go
func extractContextZapFields(ctx context.Context) []zap.Field {
    fields := make([]zap.Field, 0, 4)  // 96 bytes per call
    // ...
}
```

### Proposed Solution

Add a `sync.Pool` for the context field slice:

```go
var contextFieldPool = sync.Pool{
    New: func() any {
        s := make([]zap.Field, 0, 4)
        return &s
    },
}

func extractContextZapFieldsPooled(ctx context.Context) (*[]zap.Field, bool) {
    ptr := contextFieldPool.Get().(*[]zap.Field)
    *ptr = (*ptr)[:0]
    // ... populate
    return ptr, true  // Caller must return to pool
}
```

### Cost/Benefit Analysis

| Factor | Cost | Benefit |
|--------|------|---------|
| **Latency** | ~5ns pool overhead | Saves ~50ns allocation |
| **Memory** | Pool keeps slices alive | Less GC pressure under load |
| **Complexity** | Medium - caller must return to pool | Zero-alloc hot path |
| **Risk** | Pool misuse → memory leaks | — |
| **LOC** | +20 lines | — |

### Decision: Defer

**Rationale**:
1. The slice is only 96 bytes (4 fields × 24 bytes/field)
2. Pool Get/Put overhead may negate savings for low-volume logging
3. Added complexity and leak risk outweigh marginal gains
4. Alternative optimization (skip for `context.Background()`) provides better ROI

### Future Reconsideration

Revisit if:
- Benchmarks show >10M logs/sec sustained load
- Memory profiling shows GC pressure from context slices
- Users report allocation-related performance issues

---

## Background Context Short-Circuit (Implemented)

**Date**: 2025-12-22  
**Status**: ✅ Implemented  

### Problem

Every log call runs context extraction, even for `context.Background()` which can never have trace info.

### Solution

Short-circuit the extraction:

```go
var contextZapFields []zap.Field
if ctx != nil && ctx != context.Background() && ctx != context.TODO() {
    contextZapFields = extractContextZapFields(ctx)
}
```

### Cost/Benefit

| Factor | Value |
|--------|-------|
| **Cost** | 2 pointer comparisons (~1ns) |
| **Benefit** | Saves 1 allocation for startup/background logs |
| **Risk** | None - pointer comparison is safe |

**Decision: Implement** — Clear win with minimal cost.
