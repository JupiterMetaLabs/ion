package ion

import (
	"context"
	"fmt"
	"log"

	internalotel "github.com/JupiterMetaLabs/ion/internal/otel"
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
//	// Global usage (same API)
//	ion.SetGlobal(app)
//	ion.Info(ctx, "works from anywhere")
type Ion struct {
	logger         Logger
	serviceName    string
	version        string
	tracerProvider *internalotel.TracerProvider
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
//   - error: Fatal configuration errors (currently always nil, reserved for future use)
//
// Example:
//
//	app, warnings, err := ion.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, w := range warnings {
//	    log.Printf("ion warning: %v", w)
//	}
func New(cfg Config) (*Ion, []Warning, error) {
	var warnings []Warning

	ion := &Ion{
		serviceName: cfg.ServiceName,
		version:     cfg.Version,
	}

	// Create logger
	if cfg.OTEL.Enabled && cfg.OTEL.Endpoint != "" {
		logger, err := newZapLoggerWithOTEL(cfg)
		if err != nil {
			warnings = append(warnings, Warning{
				Component: "otel",
				Err:       fmt.Errorf("failed to init OTEL logger: %w (using basic logger)", err),
			})
			ion.logger = newZapLogger(cfg)
		} else {
			ion.logger = logger
		}
	} else {
		ion.logger = newZapLogger(cfg)
	}

	// Setup tracing
	if cfg.Tracing.Enabled {
		endpoint := cfg.Tracing.Endpoint
		if endpoint == "" {
			endpoint = cfg.OTEL.Endpoint
		}

		protocol := cfg.Tracing.Protocol
		if protocol == "" {
			protocol = cfg.OTEL.Protocol
		}

		insecure := cfg.Tracing.Insecure
		if !insecure && cfg.OTEL.Insecure {
			insecure = true
		}

		tracerCfg := internalotel.TracerConfig{
			Enabled:        true,
			Endpoint:       endpoint,
			Protocol:       protocol,
			Insecure:       insecure,
			Sampler:        cfg.Tracing.Sampler,
			Propagators:    cfg.Tracing.Propagators,
			BatchSize:      cfg.Tracing.BatchSize,
			ExportInterval: cfg.Tracing.ExportInterval,
			Timeout:        cfg.Tracing.Timeout,
			Headers:        cfg.Tracing.Headers,
			Attributes:     cfg.Tracing.Attributes,
		}

		tp, err := internalotel.SetupTracer(tracerCfg, cfg.ServiceName, cfg.Version)
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
