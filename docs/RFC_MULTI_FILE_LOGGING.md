# RFC: Multi-File Child Logger Support

**Status**: Proposal  
**Author**: Engineering  
**Date**: 2025-12-22  

## Summary

This document proposes adding support for child loggers (`Named()`) to automatically route logs to different files based on component name. Currently, all child loggers share the same output destinations as their parent.

---

## Problem Statement

Today, if you want different components to log to different files, you must create **completely separate logger instances**:

```go
// Current: Two separate loggers, two separate configs
dbLog := ion.New(ion.Default().WithFile("/var/log/db.log"))
httpLog := ion.New(ion.Default().WithFile("/var/log/http.log"))
```

**Pain Points:**
1. Must manage multiple logger instances manually.
2. Cannot use `logger.Named("db")` and have it "just work" with a different file.
3. Requires explicit dependency injection of the correct logger to each component.

---

## Current Architecture

Ion uses Zap's `zapcore.Core` abstraction. When you call `Named()` or `With()`, Zap creates a **shallow clone** of the logger that shares the same underlying Core.

```
                     ┌──────────────────┐
                     │   zapcore.Core   │
                     │  (File + OTEL)   │
                     └────────┬─────────┘
                              │ shared
           ┌──────────────────┼──────────────────┐
           │                  │                  │
    ┌──────▼──────┐    ┌──────▼──────┐    ┌──────▼──────┐
    │ logger.Named│    │ logger.Named│    │ logger.With │
    │   ("db")    │    │  ("http")   │    │ (fields...) │
    └─────────────┘    └─────────────┘    └─────────────┘
                              │
                      All write to the
                        SAME file
```

---

## Proposed Solutions

### Option A: Routing Core (Complex)

Create a custom `zapcore.Core` that inspects log fields at write-time and routes to different file writers based on the `logger` field.

```go
type routingCore struct {
    defaultWriter io.Writer
    routes        map[string]io.Writer // "db" -> /var/log/db.log
}

func (c *routingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
    // Check logger name, route to appropriate file
}
```

#### Pros
| Benefit | Description |
|---------|-------------|
| Seamless DX | `Named("db")` automatically writes to `/var/log/db.log` |
| Single Instance | One logger, one config, one lifecycle |
| Flexible | Can add routes at runtime |

#### Cons
| Drawback | Description |
|----------|-------------|
| Performance | Field inspection on every log call (~10-20ns overhead) |
| Complexity | Custom Core is harder to debug and maintain |
| Magic | Less obvious "where does this log go?" |

---

### Option B: Explicit Multi-Logger (Simple) ✅ **Recommended**

Keep the current architecture. Users create separate logger instances for separate files.

```go
// Create separate loggers for each component
dbLog := ion.New(ion.Default().WithFile("/var/log/db.log").WithService("db"))
httpLog := ion.New(ion.Default().WithFile("/var/log/http.log").WithService("http"))

// Pass to components explicitly
db := NewDatabase(dbLog)
server := NewHTTPServer(httpLog)
```

#### Pros
| Benefit | Description |
|---------|-------------|
| Zero Overhead | No runtime inspection or routing |
| Explicit | Clear where each log goes |
| Already Works | No library changes needed |
| Testable | Easy to inject mock loggers |

#### Cons
| Drawback | Description |
|----------|-------------|
| More Setup | Must configure each logger separately |
| Not Automatic | `Named()` doesn't route to different files |
| Multiple Shutdowns | Must call `Shutdown()` on each logger |

---

### Option C: Logger Registry (Middle Ground)

Add a `LoggerRegistry` that creates and caches loggers by component name.

```go
// Configure once at startup
reg := ion.NewRegistry(ion.Default())
reg.RegisterFile("db", "/var/log/db.log")
reg.RegisterFile("http", "/var/log/http.log")

// Use anywhere
dbLog := reg.Get("db")     // Creates or returns cached
httpLog := reg.Get("http")

// Single shutdown
reg.Shutdown(ctx)
```

#### Pros
| Benefit | Description |
|---------|-------------|
| Centralized Config | One place to define all component loggers |
| Cached Instances | Avoids creating duplicate loggers |
| Unified Shutdown | Single `Shutdown()` call for all |

#### Cons
| Drawback | Description |
|----------|-------------|
| New Concept | Users must learn "registry" pattern |
| Global State | Registry is typically a singleton |
| Not Automatic | Still doesn't make `Named()` route automatically |

---

## Recommendation

**Option B (Explicit Multi-Logger)** for the following reasons:

1. **Zero overhead** - No runtime cost for routing.
2. **Explicit is better** - No magic; engineers know exactly where logs go.
3. **Already works** - No library changes needed.
4. **Matches Zap philosophy** - Zap prioritizes performance over convenience.

If there is strong demand, we can add **Option C (Registry)** as a DX improvement without changing the core architecture.

---

## Open Questions for Team Feedback

1. **Is multi-file logging a real need?**  
   - How many components need separate log files?
   - Is this for compliance, debugging, or operational convenience?

2. **Would Option C (Registry) be valuable?**  
   - Does centralized config + unified shutdown justify the new concept?

3. **Alternative: Log Level Filtering?**  
   - Would routing by log *level* (errors to one file, debug to another) be more useful than by *component*?

---

## Next Steps

Pending team feedback:
- [ ] Decide on approach (A, B, or C)
- [ ] If B, document best practices in README
- [ ] If C, implement `LoggerRegistry`
- [ ] If A, prototype `routingCore` and benchmark

---

*Please share your feedback in the thread or during the next engineering sync.*
