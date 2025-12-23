package ion

import (
	"context"
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

// New creates a new Ion instance with the given configuration.
// This is the single entry point for creating ion observability.
//
// If OTEL is configured (OTEL.Endpoint set), logs are exported to OTEL collector.
// If Tracing is enabled, traces are exported to OTEL collector.
func New(cfg Config) (*Ion, error) {
	ion := &Ion{
		serviceName: cfg.ServiceName,
		version:     cfg.Version,
	}

	// Create logger
	if cfg.OTEL.Enabled && cfg.OTEL.Endpoint != "" {
		logger, err := newZapLoggerWithOTEL(cfg)
		if err != nil {
			log.Printf("[ion] Warning: Failed to init OTEL logger: %v (using basic)", err)
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
			log.Printf("[ion] Warning: Failed to init tracing: %v (tracing disabled)", err)
		} else if tp != nil {
			ion.tracerProvider = tp
			ion.tracingEnabled = true
		}
	}

	return ion, nil
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

func (i *Ion) Fatal(ctx context.Context, msg string, err error, fields ...Field) {
	i.logger.Fatal(ctx, msg, err, fields...)
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

// Tracer returns a named tracer for creating spans.
func (i *Ion) Tracer(name string) Tracer {
	if !i.tracingEnabled || i.tracerProvider == nil {
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
