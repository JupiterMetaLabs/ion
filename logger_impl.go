package ion

import (
	"context"
	"errors"
	"fmt"

	"github.com/JupiterMetaLabs/ion/internal/core"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger implements Logger using Uber's Zap.
type zapLogger struct {
	zap          *zap.Logger
	config       Config
	atomicLvl    zap.AtomicLevel
	otelProvider *core.LogProvider
}

// prepareFields consolidates context extraction and field conversion.
// It returns a slice of zap fields ready for logging.
func (l *zapLogger) prepareFields(ctx context.Context, fields []Field) []zap.Field {
	zapFields := toZapFields(fields)

	// Short-circuit: context.Background() and context.TODO() never have trace info
	if ctx != nil && ctx != context.Background() && ctx != context.TODO() {
		// Extract readable trace_id/span_id strings for console/file
		contextFields := extractContextZapFields(ctx)
		// Add ctx for otelzap bridge to extract LogRecord.TraceID/SpanID
		contextFields = append(contextFields, zap.Reflect(core.SentinelKey, ctx))
		zapFields = append(zapFields, contextFields...)
	}

	return zapFields
}

// Debug logs a message at debug level.
func (l *zapLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.DebugLevel) {
		return
	}
	// Stack depth: User -> (*Ion).Debug -> (*zapLogger).Debug
	// Zap skips: 2 (configured in core)
	l.zap.Debug(msg, l.prepareFields(ctx, fields)...)
}

// Info logs a message at info level.
func (l *zapLogger) Info(ctx context.Context, msg string, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.InfoLevel) {
		return
	}
	l.zap.Info(msg, l.prepareFields(ctx, fields)...)
}

// Warn logs a message at warn level.
func (l *zapLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.WarnLevel) {
		return
	}
	l.zap.Warn(msg, l.prepareFields(ctx, fields)...)
}

// Error logs a message at error level with an optional error.
func (l *zapLogger) Error(ctx context.Context, msg string, err error, fields ...Field) {
	if !l.atomicLvl.Enabled(zapcore.ErrorLevel) {
		return
	}

	zapFields := l.prepareFields(ctx, fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	l.zap.Error(msg, zapFields...)
}

// Critical logs a message at fatal level but does NOT exit the process.
func (l *zapLogger) Critical(ctx context.Context, msg string, err error, fields ...Field) {
	// Critical maps to Fatal level, but we use a WithFatalHook(WriteThenNoop) in the factory
	// so this will log "FATAL" and then RETURN, not exit.
	zapFields := l.prepareFields(ctx, fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	l.zap.Fatal(msg, zapFields...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		zap:          l.zap.With(toZapFields(fields)...),
		config:       l.config,
		atomicLvl:    l.atomicLvl,
		otelProvider: l.otelProvider,
	}
}

func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{
		zap:          l.zap.Named(name),
		config:       l.config,
		atomicLvl:    l.atomicLvl,
		otelProvider: l.otelProvider,
	}
}

func (l *zapLogger) Sync() error {
	return l.zap.Sync()
}

func (l *zapLogger) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown OTEL first (stop producing logs to backend)
	if l.otelProvider != nil {
		if err := l.otelProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("otel: %w", err))
		}
	}

	// Sync Zap (flush buffers)
	if err := l.zap.Sync(); err != nil {
		errs = append(errs, fmt.Errorf("zap sync: %w", err))
	}

	return errors.Join(errs...)
}

func (l *zapLogger) SetLevel(level string) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err == nil {
		l.atomicLvl.SetLevel(lvl)
	}
}

func (l *zapLogger) GetLevel() string {
	return l.atomicLvl.Level().String()
}

// --- Field conversion ---

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
