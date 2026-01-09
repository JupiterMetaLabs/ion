package ion

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/JupiterMetaLabs/ion/internal/core"
	"go.uber.org/zap"
)

// Ion is the unified observability instance providing logging and tracing.
// It implements the Logger interface directly, so you can use it for logging.
// It also provides access to Tracer for distributed tracing.
//
// Example:
//
//	app, err := ion.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Shutdown(context.Background())
//
//	// Logging (Ion implements Logger)
//	app.Info(ctx, "message", ion.F("key", "value"))
//
//	// Tracing
//	tracer := app.Tracer("myapp.component")
//	ctx, span := tracer.Start(ctx, "Operation")
//	defer span.End()
//
//	// Global usage (DEPRECATED: Prefer Dependency Injection)
//	ion.SetGlobal(app)
//	ion.Info(ctx, "works from anywhere (but try to avoid this)")
type Ion struct {
	logger         Logger
	serviceName    string
	version        string
	tracerProvider *core.TracerProvider
	tracingEnabled bool
}

// Warning represents a non-fatal initialization issue.
// Ion returns warnings instead of failing when optional components
// (like OTEL or tracing) cannot be initialized.
type Warning struct {
	Component string // "otel", "tracing"
	Err       error
}

func (w Warning) Error() string {
	return fmt.Sprintf("%s: %v", w.Component, w.Err)
}

// New creates a new Ion instance with the given configuration.
// This is the single entry point for creating ion observability.
//
// Returns:
//   - *Ion: Always returns a working Ion instance (may use fallbacks)
//   - []Warning: Non-fatal issues (e.g., OTEL connection failed, tracing disabled)
//   - error: Fatal configuration errors
func New(cfg Config) (*Ion, []Warning, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %w", err)
	}

	var warnings []Warning

	ion := &Ion{
		serviceName: cfg.ServiceName,
		version:     cfg.Version,
	}

	// 1. Setup Logger (Zap + OTEL Logs)
	zapRes, err := core.NewZapLogger(cfg)
	if err != nil {
		// Fatal error if we can't even init Zap (e.g. file error)
		return nil, nil, fmt.Errorf("failed to init logger: %w", err)
	}

	// Construct the logger wrapper
	ion.logger = &zapLogger{
		zap:          zapRes.Logger,
		config:       cfg,
		atomicLvl:    zapRes.AtomicLevel,
		otelProvider: zapRes.OTELProvider,
	}

	// 2. Setup Tracing (OTEL Traces)
	if cfg.Tracing.Enabled {
		// Use Tracing endpoint or fallback to OTEL endpoint
		if cfg.Tracing.Endpoint == "" {
			cfg.Tracing.Endpoint = cfg.OTEL.Endpoint
		}
		if cfg.Tracing.Protocol == "" {
			cfg.Tracing.Protocol = cfg.OTEL.Protocol
		}
		if !cfg.Tracing.Insecure && cfg.OTEL.Insecure {
			cfg.Tracing.Insecure = true // Inherit insecure if not explicitly set
		}

		tp, err := core.SetupTracerProvider(cfg.Tracing, cfg.ServiceName, cfg.Version)
		if err != nil {
			warnings = append(warnings, Warning{
				Component: "tracing",
				Err:       fmt.Errorf("failed to init tracing: %w (tracing disabled)", err),
			})
		} else if tp != nil {
			ion.tracerProvider = tp
			ion.tracingEnabled = true
		}
	}

	return ion, warnings, nil
}

// --- Global Accessor (Deprecated) ---

var (
	globalLogger Logger
	globalMu     sync.RWMutex
	globalOnce   sync.Once
)

// SetGlobal sets the global logger instance.
//
// Deprecated: Use Dependency Injection instead. This method exists for migration
// purposes only and will be removed in a future version.
func SetGlobal(logger Logger) {
	globalOnce.Do(func() {
		// Log warning on first usage
		log.Println("[ion] WARNING: ion.SetGlobal is deprecated. Please inject ion.Logger explicitly.")
	})
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = logger
}

// GetGlobal returns the global logger.
// If SetGlobal has not been called, it returns a safe no-op logger.
func GetGlobal() Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger != nil {
		return globalLogger
	}
	return nopLogger
}

var nopLogger Logger = &zapLogger{
	zap:       zap.NewNop(),
	atomicLvl: zap.NewAtomicLevel(),
}

// Debug logs at debug level using global logger.
func Debug(ctx context.Context, msg string, fields ...Field) {
	GetGlobal().Debug(ctx, msg, fields...)
}

// Info logs at info level using global logger.
func Info(ctx context.Context, msg string, fields ...Field) {
	GetGlobal().Info(ctx, msg, fields...)
}

// Warn logs at warn level using global logger.
func Warn(ctx context.Context, msg string, fields ...Field) {
	GetGlobal().Warn(ctx, msg, fields...)
}

// Error logs at error level using global logger.
func Error(ctx context.Context, msg string, err error, fields ...Field) {
	GetGlobal().Error(ctx, msg, err, fields...)
}

// Critical logs at critical level using global logger.
// Does NOT exit the process - caller decides what to do.
func Critical(ctx context.Context, msg string, err error, fields ...Field) {
	GetGlobal().Critical(ctx, msg, err, fields...)
}

// GetTracer returns a named tracer from global Ion.
// Note: This relies on SetGlobal being called with an *Ion instance (or implementation that supports Tracer())
// If the global logger does not support Tracer, this might fail or return no-op.
// Since Logger interface doesn't have Tracer(), we can only check if global is *Ion.
// BUT, legacy GetTracer probably used `otel.Tracer`.
// We should probably just call `otel.Tracer` directly here or use `GetGlobal`?
// The previous implementation called `getGlobal().Tracer()`.
// Since `Logger` interface does NOT have `Tracer`, `ion.Tracer` works on `*Ion`.
// So `GetGlobal()` returning `Logger` logic is tricky for `Tracer`.
// Fix: Check type assertion.
func GetTracer(name string) Tracer {
	if ion, ok := GetGlobal().(*Ion); ok {
		return ion.Tracer(name)
	}
	// Fallback to OTEL directly if global logger isn't *Ion?
	// or return no-op.
	// We'll return a no-op tracer with a warning log if possible.
	return newOTELTracer(name) // This uses global OTEL provider set by core.
}

// Sync flushes the global logger.
func Sync() error {
	return GetGlobal().Sync()
}

// Named returns a child logger from global.
func Named(name string) Logger {
	return GetGlobal().Named(name)
}

// --- Logger interface implementation ---

func (i *Ion) Debug(ctx context.Context, msg string, fields ...Field) {
	i.logger.Debug(ctx, msg, fields...)
}

func (i *Ion) Info(ctx context.Context, msg string, fields ...Field) {
	i.logger.Info(ctx, msg, fields...)
}

func (i *Ion) Warn(ctx context.Context, msg string, fields ...Field) {
	i.logger.Warn(ctx, msg, fields...)
}

func (i *Ion) Error(ctx context.Context, msg string, err error, fields ...Field) {
	i.logger.Error(ctx, msg, err, fields...)
}

func (i *Ion) Critical(ctx context.Context, msg string, err error, fields ...Field) {
	i.logger.Critical(ctx, msg, err, fields...)
}

func (i *Ion) With(fields ...Field) Logger {
	return i.logger.With(fields...)
}

func (i *Ion) Named(name string) Logger {
	return i.logger.Named(name)
}

func (i *Ion) Sync() error {
	return i.logger.Sync()
}

func (i *Ion) SetLevel(level string) {
	i.logger.SetLevel(level)
}

func (i *Ion) GetLevel() string {
	return i.logger.GetLevel()
}

// --- Tracer access ---

var tracingDisabledLogged bool

// Tracer returns a named tracer for creating spans.
// If tracing is not enabled, returns a no-op tracer (logs warning once).
func (i *Ion) Tracer(name string) Tracer {
	if !i.tracingEnabled || i.tracerProvider == nil {
		if !tracingDisabledLogged {
			tracingDisabledLogged = true
			log.Println("[ion] Tracing disabled: Tracer() returning no-op. Enable via Config.Tracing.Enabled")
		}
		return noopTracer{}
	}
	// core.SetupTracerProvider sets the global OTEL provider,
	// so newOTELTracer(name) which calls otel.Tracer(name) works correctly.
	return newOTELTracer(name)
}

// --- Lifecycle ---

// Shutdown gracefully shuts down logging and tracing.
func (i *Ion) Shutdown(ctx context.Context) error {
	var firstErr error

	if i.tracerProvider != nil {
		if err := i.tracerProvider.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}

	if i.logger != nil {
		if err := i.logger.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
