package ion

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
// - Console/File: filter "ctx" (shows ugly {}), keep trace_id/span_id strings
// - OTEL: filter trace_id/span_id strings (redundant), keep "ctx" for LogRecord correlation
func buildLogger(cfg Config, otelCore zapcore.Core, otelProvider *otel.Provider) Logger {
	atomicLevel := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	cores := make([]zapcore.Core, 0, 4)

	// Console output - filter "ctx" field (otelzap artifact, not readable)
	if cfg.Console.Enabled {
		consoleCores := buildConsoleCores(cfg, atomicLevel)
		for _, c := range consoleCores {
			cores = append(cores, newFilteringCore(c, "ctx"))
		}
	}

	// File output - same filtering as console
	if cfg.File.Enabled && cfg.File.Path != "" {
		fileCore := buildFileCore(cfg, atomicLevel)
		if fileCore != nil {
			cores = append(cores, newFilteringCore(fileCore, "ctx"))
		}
	}

	// OTEL core - filter trace_id/span_id strings (redundant, LogRecord has them)
	// Keep "ctx" so otelzap bridge can extract trace context for LogRecord.TraceID
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
// 1. Converting ion.Field to zap.Field (with pooling)
// 2. Extracting context fields (trace_id, span_id, etc.)
// 3. Adding ctx for OTEL LogRecord trace correlation
// 4. Calling the appropriate zap log method
// 5. Returning the pooled slice
//
// Trace correlation strategy:
// - ctx is passed as zap.Reflect for otelzap bridge to extract LogRecord.TraceID
// - trace_id/span_id strings are added for console/file readability
// - filtercore strips ctx from console (ugly), strips strings from OTEL (redundant)
//
// Performance optimization: We skip context extraction for context.Background()
// since it can never contain trace information.
func (l *zapLogger) logWithFields(ctx context.Context, logFn zapLogFunc, msg string, fields []Field) {
	zapFields := toZapFieldsTransient(fields)

	// Short-circuit: context.Background() and context.TODO() never have trace info
	var contextZapFields []zap.Field
	hasTraceContext := ctx != nil && ctx != context.Background() && ctx != context.TODO()

	if hasTraceContext {
		// Extract readable trace_id/span_id strings for console/file
		contextZapFields = extractContextZapFields(ctx)
		// Add ctx for otelzap bridge to extract LogRecord.TraceID/SpanID
		// filtercore strips this from console (shows {}) but OTEL uses it
		contextZapFields = append(contextZapFields, zap.Reflect("ctx", ctx))
	}

	if zapFields != nil {
		if len(contextZapFields) > 0 {
			*zapFields = append(*zapFields, contextZapFields...)
		}
		logFn(msg, *zapFields...)
		putZapFields(zapFields)
	} else if len(contextZapFields) > 0 {
		logFn(msg, contextZapFields...)
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

	zapFields := toZapFieldsTransient(fields)
	contextZapFields := extractContextZapFields(ctx)

	// Add ctx for otelzap bridge trace correlation
	hasTraceContext := ctx != nil && ctx != context.Background() && ctx != context.TODO()
	if hasTraceContext {
		contextZapFields = append(contextZapFields, zap.Reflect("ctx", ctx))
	}

	if zapFields == nil {
		var allFields []zap.Field
		if err != nil {
			allFields = append(allFields, zap.Error(err))
		}
		allFields = append(allFields, contextZapFields...)
		l.zap.Error(msg, allFields...)
		return
	}

	if err != nil {
		*zapFields = append(*zapFields, zap.Error(err))
	}
	*zapFields = append(*zapFields, contextZapFields...)
	l.zap.Error(msg, *zapFields...)
	putZapFields(zapFields)
}

// Fatal logs a message at fatal level and calls os.Exit(1).
// Note: This method syncs the logger before exiting to ensure logs are flushed.
// Pool cleanup is skipped since the process exits immediately.
func (l *zapLogger) Fatal(ctx context.Context, msg string, err error, fields ...Field) {
	// Use allocating conversion since os.Exit prevents pool cleanup
	zapFields := toZapFields(fields)
	contextZapFields := extractContextZapFields(ctx)

	// Add ctx for otelzap bridge trace correlation
	hasTraceContext := ctx != nil && ctx != context.Background() && ctx != context.TODO()
	if hasTraceContext {
		contextZapFields = append(contextZapFields, zap.Reflect("ctx", ctx))
	}

	var allFields []zap.Field
	if err != nil {
		allFields = append(allFields, zap.Error(err))
	}
	allFields = append(allFields, zapFields...)
	allFields = append(allFields, contextZapFields...)

	// Sync before Fatal to flush buffered logs
	_ = l.zap.Sync()

	// Shutdown OTEL provider to flush traces (best effort)
	if l.otelProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = l.otelProvider.Shutdown(ctx)
		cancel()
	}

	l.zap.Fatal(msg, allFields...)
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

// convertField converts a single ion.Field to zap.Field.
// Shared by both pooled and allocating conversion paths.
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

// toZapFieldsTransient converts ion.Field slice to a pooled zap.Field slice.
// The caller MUST return the slice to the pool using putZapFields.
// Safe for Info/Debug/Error, NOT safe for With/Named.
func toZapFieldsTransient(fields []Field) *[]zap.Field {
	if len(fields) == 0 {
		return nil
	}

	ptr := zapFieldPool.Get().(*[]zap.Field)
	*ptr = (*ptr)[:0]

	for _, f := range fields {
		*ptr = append(*ptr, convertField(f))
	}
	return ptr
}

// putZapFields cleans up the slice and returns it to the pool.
func putZapFields(ptr *[]zap.Field) {
	if ptr == nil {
		return
	}
	// Reset length - underlying array may hold refs but they're overwritten on Get
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
		zapFields = append(zapFields, convertField(f))
	}
	return zapFields
}
