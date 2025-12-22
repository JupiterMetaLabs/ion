package ion

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/JupiterMetaLabs/ion/internal/otel"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger implements Logger using Uber's Zap.
type zapLogger struct {
	zap          *zap.Logger
	config       Config
	atomicLvl    zap.AtomicLevel
	otelProvider *otel.Provider
}

// New creates a new Logger from the provided configuration.
// This is the main constructor for ion.
func New(cfg Config) Logger {
	return buildLogger(cfg, nil, nil)
}

// NewWithOTEL creates a logger with OTEL export enabled.
// This wires Zap logs to the OpenTelemetry log pipeline.
func NewWithOTEL(cfg Config) (Logger, error) {
	// Map config to internal OTEL config
	otelCfg := otel.Config{
		Enabled:        cfg.OTEL.Enabled,
		Endpoint:       cfg.OTEL.Endpoint,
		Protocol:       cfg.OTEL.Protocol,
		Insecure:       cfg.OTEL.Insecure,
		Timeout:        cfg.OTEL.Timeout,
		Headers:        cfg.OTEL.Headers,
		Attributes:     cfg.OTEL.Attributes,
		BatchSize:      cfg.OTEL.BatchSize,
		ExportInterval: cfg.OTEL.ExportInterval,
	}

	// First set up OTEL provider
	provider, err := otel.Setup(otelCfg, cfg.ServiceName, cfg.Version)
	if err != nil {
		return nil, err
	}

	var otelCore zapcore.Core
	if provider != nil && provider.LoggerProvider() != nil {
		otelCore = otelzap.NewCore(
			cfg.ServiceName,
			otelzap.WithLoggerProvider(provider.LoggerProvider()),
		)
	}

	return buildLogger(cfg, otelCore, provider), nil
}

// buildLogger constructs the zapLogger with all configured cores.
// If otelCore is non-nil, it's added to the core tee.
func buildLogger(cfg Config, otelCore zapcore.Core, otelProvider *otel.Provider) Logger {
	atomicLevel := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	cores := make([]zapcore.Core, 0, 4)

	// Console output
	if cfg.Console.Enabled {
		consoleCores := buildConsoleCores(cfg, atomicLevel)
		cores = append(cores, consoleCores...)
	}

	// File output (with rotation)
	if cfg.File.Enabled && cfg.File.Path != "" {
		fileCore := buildFileCore(cfg, atomicLevel)
		if fileCore != nil {
			cores = append(cores, fileCore)
		}
	}

	// OTEL core (if provided)
	if otelCore != nil {
		cores = append(cores, otelCore)
	}

	// Combine all cores
	var core zapcore.Core
	switch len(cores) {
	case 0:
		core = zapcore.NewNopCore()
	case 1:
		core = cores[0]
	default:
		core = zapcore.NewTee(cores...)
	}

	// Build options
	opts := buildZapOptions(cfg)
	logger := zap.New(core, opts...)

	return &zapLogger{
		zap:          logger,
		config:       cfg,
		atomicLvl:    atomicLevel,
		otelProvider: otelProvider,
	}
}

// buildZapOptions creates common zap options from config.
func buildZapOptions(cfg Config) []zap.Option {
	opts := []zap.Option{
		zap.AddCallerSkip(1), // Skip the wrapper methods
	}

	if cfg.Development {
		opts = append(opts, zap.Development())
		opts = append(opts, zap.AddCaller())
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	if cfg.ServiceName != "" {
		opts = append(opts, zap.Fields(zap.String("service", cfg.ServiceName)))
	}
	if cfg.Version != "" {
		opts = append(opts, zap.Fields(zap.String("version", cfg.Version)))
	}

	return opts
}

// buildConsoleCores creates console output cores.
// Returns multiple cores if ErrorsToStderr is enabled (stdout for info, stderr for errors).
func buildConsoleCores(cfg Config, level zap.AtomicLevel) []zapcore.Core {
	encoder := buildConsoleEncoder(cfg)

	if cfg.Console.ErrorsToStderr {
		// Split: debug/info → stdout, warn/error/fatal → stderr
		stdoutLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= level.Level() && lvl < zapcore.WarnLevel
		})
		stderrLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.WarnLevel
		})

		return []zapcore.Core{
			zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), stdoutLevel),
			zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), stderrLevel),
		}
	}

	return []zapcore.Core{
		zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), level),
	}
}

// buildConsoleEncoder creates the appropriate encoder for console output.
func buildConsoleEncoder(cfg Config) zapcore.Encoder {
	if cfg.Console.Format == "pretty" || (cfg.Development && cfg.Console.Format == "") {
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		if cfg.Console.Color {
			encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
			encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
		} else {
			encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
			encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		}
		encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
		return zapcore.NewConsoleEncoder(encoderCfg)
	}

	// JSON encoder for production
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.MessageKey = "msg"
	encoderCfg.LevelKey = "level"
	encoderCfg.CallerKey = "caller"
	return zapcore.NewJSONEncoder(encoderCfg)
}

// buildFileCore creates the file output core with rotation.
func buildFileCore(cfg Config, level zap.AtomicLevel) zapcore.Core {
	writer := NewFileWriter(cfg.File)
	if writer == nil {
		return nil
	}

	// Always use JSON for file output
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderCfg)

	return zapcore.NewCore(encoder, zapcore.AddSync(writer), level)
}

// parseLevel converts a string level to zapcore.Level.
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// --- Logger interface implementation ---

func (l *zapLogger) Debug(msg string, fields ...Field) {
	zapFields := toZapFieldsTransient(fields)
	if zapFields != nil {
		l.zap.Debug(msg, *zapFields...)
		putZapFields(zapFields)
	} else {
		l.zap.Debug(msg)
	}
}

func (l *zapLogger) Info(msg string, fields ...Field) {
	zapFields := toZapFieldsTransient(fields)
	if zapFields != nil {
		l.zap.Info(msg, *zapFields...)
		putZapFields(zapFields)
	} else {
		l.zap.Info(msg)
	}
}

func (l *zapLogger) Warn(msg string, fields ...Field) {
	zapFields := toZapFieldsTransient(fields)
	if zapFields != nil {
		l.zap.Warn(msg, *zapFields...)
		putZapFields(zapFields)
	} else {
		l.zap.Warn(msg)
	}
}

func (l *zapLogger) Error(msg string, err error, fields ...Field) {
	zapFields := toZapFieldsTransient(fields)
	// We need to append the error, so we must ensure space or handled separately.
	// zap.Error creates a field.
	// Since we are using pool, appending might realloc if cap exceeded.
	// But our pool is default 16.

	if zapFields == nil {
		// New slice just for error
		if err != nil {
			l.zap.Error(msg, zap.Error(err))
		} else {
			l.zap.Error(msg)
		}
		return
	}

	if err != nil {
		*zapFields = append(*zapFields, zap.Error(err))
	}
	l.zap.Error(msg, *zapFields...)
	putZapFields(zapFields)
}

func (l *zapLogger) Fatal(msg string, err error, fields ...Field) {
	zapFields := toZapFieldsTransient(fields)

	if zapFields == nil {
		if err != nil {
			l.zap.Fatal(msg, zap.Error(err))
		} else {
			l.zap.Fatal(msg)
		}
		return
	}

	if err != nil {
		*zapFields = append(*zapFields, zap.Error(err))
	}
	l.zap.Fatal(msg, *zapFields...)
	putZapFields(zapFields)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		zap:       l.zap.With(toZapFields(fields)...),
		config:    l.config,
		atomicLvl: l.atomicLvl,
	}
}

func (l *zapLogger) WithContext(ctx context.Context) Logger {
	contextFields := extractContextFields(ctx)
	if len(contextFields) == 0 {
		return l
	}
	return l.With(contextFields...)
}

func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{
		zap:       l.zap.Named(name),
		config:    l.config,
		atomicLvl: l.atomicLvl,
	}
}

func (l *zapLogger) Sync() error {
	return l.zap.Sync()
}

func (l *zapLogger) Shutdown(ctx context.Context) error {
	// Sync Zap first
	_ = l.zap.Sync()

	// Shutdown OTEL if present
	if l.otelProvider != nil {
		return l.otelProvider.Shutdown(ctx)
	}
	return nil
}

// SetLevel changes the log level at runtime.
// This is safe to call from multiple goroutines.
func (l *zapLogger) SetLevel(level string) {
	l.atomicLvl.SetLevel(parseLevel(level))
}

// GetLevel returns the current log level.
func (l *zapLogger) GetLevel() string {
	return l.atomicLvl.Level().String()
}

// --- Field conversion ---

var zapFieldPool = sync.Pool{
	New: func() any {
		// Default cap 16 covers most use cases
		slice := make([]zap.Field, 0, 16)
		return &slice
	},
}

// toZapFieldsTransient converts ion.Field slice to a pooled zap.Field slice.
// The caller MUST return the slice to the pool using putZapFields.
// safe for Info/Debug/Error, NOT safe for With/Named.
func toZapFieldsTransient(fields []Field) *[]zap.Field {
	if len(fields) == 0 {
		return nil
	}

	ptr := zapFieldPool.Get().(*[]zap.Field)
	*ptr = (*ptr)[:0]

	for _, f := range fields {
		switch f.Type {
		case StringType:
			*ptr = append(*ptr, zap.String(f.Key, f.StringVal))
		case Int64Type:
			*ptr = append(*ptr, zap.Int64(f.Key, f.Integer))
		case Float64Type:
			*ptr = append(*ptr, zap.Float64(f.Key, f.Float))
		case BoolType:
			*ptr = append(*ptr, zap.Bool(f.Key, f.Integer == 1))
		case ErrorType:
			// Ensure Interface is actually an error to avoid panic, though Err constructor ensures it
			if err, ok := f.Interface.(error); ok {
				*ptr = append(*ptr, zap.Error(err))
			} else {
				*ptr = append(*ptr, zap.Any(f.Key, f.Interface))
			}
		default:
			*ptr = append(*ptr, zap.Any(f.Key, f.Interface))
		}
	}
	return ptr
}

// putZapFields cleans up the slice and returns it to the pool.
func putZapFields(ptr *[]zap.Field) {
	if ptr == nil {
		return
	}
	// Clear slice references to prevent memory leaks (if values held pointers)
	// Although zap.Field is strict, zap.Any depends on the usage.
	// For high-perf pool, we validly just reset length, but better to be safe?
	// Resetting length is enough for the slice, but the array might hold refs.
	// Given we overwrite on Get, it's mostly fine.
	*ptr = (*ptr)[:0]
	zapFieldPool.Put(ptr)
}

// toZapFields converts ion.Field slice to zap.Field slice (allocating).
// Use this for With() where the slice is retained.
func toZapFields(fields []Field) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	zapFields := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		switch f.Type {
		case StringType:
			zapFields = append(zapFields, zap.String(f.Key, f.StringVal))
		case Int64Type:
			zapFields = append(zapFields, zap.Int64(f.Key, f.Integer))
		case Float64Type:
			zapFields = append(zapFields, zap.Float64(f.Key, f.Float))
		case BoolType:
			zapFields = append(zapFields, zap.Bool(f.Key, f.Integer == 1))
		case ErrorType:
			if err, ok := f.Interface.(error); ok {
				zapFields = append(zapFields, zap.Error(err))
			} else {
				zapFields = append(zapFields, zap.Any(f.Key, f.Interface))
			}
		default:
			zapFields = append(zapFields, zap.Any(f.Key, f.Interface))
		}
	}
	return zapFields
}
