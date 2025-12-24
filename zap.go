package ion

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/JupiterMetaLabs/ion/internal/otel"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// _ctxFieldKey is a sentinel key for passing context to the otelzap bridge.
// This internal key is filtered by filtercore to avoid collision with user fields
// and prevent ugly {} output on console.
const _ctxFieldKey = "__ion_ctx__"

// zapLogger implements Logger using Uber's Zap.
type zapLogger struct {
	zap          *zap.Logger
	config       Config
	atomicLvl    zap.AtomicLevel
	otelProvider *otel.Provider
}

// newZapLogger creates a new Logger from the provided configuration.
// Internal - use ion.New() instead.
func newZapLogger(cfg Config) Logger {
	return buildLogger(cfg, nil, nil)
}

// newZapLoggerWithOTEL creates a logger with OTEL export enabled.
// Internal - use ion.New() instead.
func newZapLoggerWithOTEL(cfg Config) (Logger, error) {
	// Handle Basic Auth - inject Authorization header from Username/Password
	headers := cfg.OTEL.Headers
	if headers == nil {
		headers = make(map[string]string)
	}

	if cfg.OTEL.Username != "" && cfg.OTEL.Password != "" {
		auth := fmt.Sprintf("%s:%s", cfg.OTEL.Username, cfg.OTEL.Password)
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		headers["Authorization"] = "Basic " + encodedAuth
	}

	// Map config to internal OTEL config
	otelCfg := otel.Config{
		Enabled:        cfg.OTEL.Enabled,
		Endpoint:       cfg.OTEL.Endpoint,
		Protocol:       cfg.OTEL.Protocol,
		Insecure:       cfg.OTEL.Insecure,
		Timeout:        cfg.OTEL.Timeout,
		Headers:        headers,
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
//
// Filtering strategy for clean trace correlation:
// - Console/File: filter sentinel key (shows ugly {}), keep trace_id/span_id strings
// - OTEL: filter trace_id/span_id strings (redundant), keep sentinel for LogRecord
func buildLogger(cfg Config, otelCore zapcore.Core, otelProvider *otel.Provider) Logger {
	atomicLevel := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	cores := make([]zapcore.Core, 0, 4)

	// Console output - filter sentinel key (otelzap artifact, not readable)
	if cfg.Console.Enabled {
		consoleCores := buildConsoleCores(cfg, atomicLevel)
		for _, c := range consoleCores {
			cores = append(cores, newFilteringCore(c, _ctxFieldKey))
		}
	}

	// File output - same filtering as console
	if cfg.File.Enabled && cfg.File.Path != "" {
		fileCore := buildFileCore(cfg, atomicLevel)
		if fileCore != nil {
			cores = append(cores, newFilteringCore(fileCore, _ctxFieldKey))
		}
	}

	// OTEL core - filter trace_id/span_id strings (redundant, LogRecord has them via bridge)
	// Keep sentinel key so otelzap bridge can extract trace context for LogRecord.TraceID
	if otelCore != nil {
		cores = append(cores, newFilteringCore(otelCore, "trace_id", "span_id"))
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

// zapLogFunc is a function type for zap's log methods.
// This allows us to pass the appropriate log method to the helper.
type zapLogFunc func(msg string, fields ...zap.Field)

// logWithFields is a helper that consolidates the common pattern of:
// 1. Converting ion.Field to zap.Field
// 2. Extracting context fields (trace_id, span_id, etc.)
// 3. Adding ctx for OTEL LogRecord trace correlation
// 4. Calling the appropriate zap log method
//
// Trace correlation strategy:
// - ctx is passed as zap.Reflect for otelzap bridge to extract LogRecord.TraceID
// - trace_id/span_id strings are added for console/file readability
// - filtercore strips ctx from console (ugly), strips strings from OTEL (redundant)
//
// Note: We use allocating field conversion (not pooling) because zap fields may be
// encoded asynchronously by cores/exporters. Pooling risks corrupted logs.
func (l *zapLogger) logWithFields(ctx context.Context, logFn zapLogFunc, msg string, fields []Field) {
	zapFields := toZapFields(fields)

	// Short-circuit: context.Background() and context.TODO() never have trace info
	hasTraceContext := ctx != nil && ctx != context.Background() && ctx != context.TODO()

	if hasTraceContext {
		// Extract readable trace_id/span_id strings for console/file
		contextFields := extractContextZapFields(ctx)
		// Add ctx for otelzap bridge to extract LogRecord.TraceID/SpanID
		// filtercore strips this from console but OTEL uses it
		contextFields = append(contextFields, zap.Reflect(_ctxFieldKey, ctx))
		zapFields = append(zapFields, contextFields...)
	}

	if len(zapFields) > 0 {
		logFn(msg, zapFields...)
	} else {
		logFn(msg)
	}
}

// Debug logs a message at debug level.
func (l *zapLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.DebugLevel) {
		return // Zero allocation for filtered levels
	}
	l.logWithFields(ctx, l.zap.Debug, msg, fields)
}

// Info logs a message at info level.
func (l *zapLogger) Info(ctx context.Context, msg string, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.InfoLevel) {
		return
	}
	l.logWithFields(ctx, l.zap.Info, msg, fields)
}

// Warn logs a message at warn level.
func (l *zapLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.WarnLevel) {
		return
	}
	l.logWithFields(ctx, l.zap.Warn, msg, fields)
}

// Error logs a message at error level with an optional error.
func (l *zapLogger) Error(ctx context.Context, msg string, err error, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.ErrorLevel) {
		return
	}

	zapFields := toZapFields(fields)

	// Add error field
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	// Add trace context if present
	hasTraceContext := ctx != nil && ctx != context.Background() && ctx != context.TODO()
	if hasTraceContext {
		contextFields := extractContextZapFields(ctx)
		contextFields = append(contextFields, zap.Reflect(_ctxFieldKey, ctx))
		zapFields = append(zapFields, contextFields...)
	}

	l.zap.Error(msg, zapFields...)
}

// Critical logs a message at fatal level but does NOT exit the process.
//
// Unlike Fatal, this method logs at the highest severity (fatal level) but
// leaves process lifecycle control to the caller. Shared infrastructure
// libraries must never call os.Exit - only main() should decide when to exit.
//
// For graceful shutdown after a critical error, return the error up the
// call stack and call app.Shutdown() explicitly.
func (l *zapLogger) Critical(ctx context.Context, msg string, err error, fields ...Field) {
	zapFields := toZapFields(fields)

	// Add error field
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	// Add trace context if present
	hasTraceContext := ctx != nil && ctx != context.Background() && ctx != context.TODO()
	if hasTraceContext {
		contextFields := extractContextZapFields(ctx)
		contextFields = append(contextFields, zap.Reflect(_ctxFieldKey, ctx))
		zapFields = append(zapFields, contextFields...)
	}

	// Log at fatal level severity but using DPanic (which doesn't exit in production)
	// This gives fatal-level visibility for alerting without process termination
	l.zap.DPanic(msg, zapFields...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		zap:       l.zap.With(toZapFields(fields)...),
		config:    l.config,
		atomicLvl: l.atomicLvl,
	}
}

// NOTE: WithContext was removed - context is now passed directly to log methods.

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
	var errs []error

	// Sync Zap first
	if err := l.zap.Sync(); err != nil {
		errs = append(errs, fmt.Errorf("zap sync: %w", err))
	}

	// Shutdown OTEL if present
	if l.otelProvider != nil {
		if err := l.otelProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("otel: %w", err))
		}
	}

	return errors.Join(errs...)
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

// convertField converts a single ion.Field to zap.Field.
func convertField(f Field) zap.Field {
	switch f.Type {
	case StringType:
		return zap.String(f.Key, f.StringVal)
	case Int64Type:
		return zap.Int64(f.Key, f.Integer)
	case Uint64Type:
		return zap.Uint64(f.Key, f.Interface.(uint64))
	case Float64Type:
		return zap.Float64(f.Key, f.Float)
	case BoolType:
		return zap.Bool(f.Key, f.Integer == 1)
	case ErrorType:
		if err, ok := f.Interface.(error); ok {
			return zap.Error(err)
		}
		return zap.Any(f.Key, f.Interface)
	default:
		return zap.Any(f.Key, f.Interface)
	}
}

// toZapFields converts ion.Field slice to zap.Field slice (allocating).
// Use this for With() where the slice is retained.
func toZapFields(fields []Field) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	zapFields := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		zapFields = append(zapFields, convertField(f))
	}
	return zapFields
}
