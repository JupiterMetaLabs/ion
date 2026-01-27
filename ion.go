package ion

import (
	"context"
	"fmt"
	"log"

	"github.com/JupiterMetaLabs/ion/internal/core"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
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
//	// Metrics
//	meter := app.Meter("myapp.component")
//	counter, _ := meter.Int64Counter("my.counter")
//	counter.Add(ctx, 1)
type Ion struct {
	logger         Logger
	serviceName    string
	version        string
	tracerProvider *core.TracerProvider
	tracingEnabled bool
	meterProvider  *core.MeterProvider
	metricsEnabled bool
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
		if cfg.Tracing.Username == "" {
			cfg.Tracing.Username = cfg.OTEL.Username
		}
		if cfg.Tracing.Password == "" {
			cfg.Tracing.Password = cfg.OTEL.Password
		}
		if cfg.Tracing.Timeout == 0 {
			cfg.Tracing.Timeout = cfg.OTEL.Timeout
		}
		if cfg.Tracing.BatchSize == 0 {
			cfg.Tracing.BatchSize = cfg.OTEL.BatchSize
		}
		if cfg.Tracing.ExportInterval == 0 {
			cfg.Tracing.ExportInterval = cfg.OTEL.ExportInterval
		}
		if cfg.Tracing.Headers == nil && len(cfg.OTEL.Headers) > 0 {
			// Deep copy headers to avoid map reference issues
			cfg.Tracing.Headers = make(map[string]string, len(cfg.OTEL.Headers))
			for k, v := range cfg.OTEL.Headers {
				cfg.Tracing.Headers[k] = v
			}
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

	// 3. Setup Metrics (OTEL Metrics)
	if cfg.Metrics.Enabled {
		// Use Metrics endpoint or fallback to OTEL endpoint
		if cfg.Metrics.Endpoint == "" {
			cfg.Metrics.Endpoint = cfg.OTEL.Endpoint
		}
		if cfg.Metrics.Protocol == "" {
			cfg.Metrics.Protocol = cfg.OTEL.Protocol
		}
		if !cfg.Metrics.Insecure && cfg.OTEL.Insecure {
			cfg.Metrics.Insecure = true
		}
		// Auth inheritance
		if cfg.Metrics.Username == "" {
			cfg.Metrics.Username = cfg.OTEL.Username
		}
		if cfg.Metrics.Password == "" {
			cfg.Metrics.Password = cfg.OTEL.Password
		}
		// Metadata inheritance
		if cfg.Metrics.Headers == nil && len(cfg.OTEL.Headers) > 0 {
			cfg.Metrics.Headers = make(map[string]string, len(cfg.OTEL.Headers))
			for k, v := range cfg.OTEL.Headers {
				cfg.Metrics.Headers[k] = v
			}
		}

		mp, err := core.SetupMeterProvider(cfg.Metrics, cfg.ServiceName, cfg.Version)
		if err != nil {
			warnings = append(warnings, Warning{
				Component: "metrics",
				Err:       fmt.Errorf("failed to init metrics: %w (metrics disabled)", err),
			})
		} else if mp != nil {
			ion.meterProvider = mp
			ion.metricsEnabled = true
		}
	}

	return ion, warnings, nil
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

// --- Metrics access ---

// Meter returns a named meter for creating instruments.
func (i *Ion) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	if !i.metricsEnabled || i.meterProvider == nil {
		return newNoopMeter()
	}
	return i.meterProvider.Meter(name, opts...)
}

// Ensure noopMeter is initialized with a working noop implementation
func newNoopMeter() metric.Meter {
	return noop.NewMeterProvider().Meter("noop")
}

// --- Lifecycle ---

// Shutdown gracefully shuts down logging, tracing, and metrics.
func (i *Ion) Shutdown(ctx context.Context) error {
	var firstErr error

	if i.tracerProvider != nil {
		if err := i.tracerProvider.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}

	if i.meterProvider != nil {
		if err := i.meterProvider.Shutdown(ctx); err != nil && firstErr == nil {
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
