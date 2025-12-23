package ion

import (
	"context"
	"sync"
)

var (
	globalMu sync.RWMutex
	global   *Ion
)

// SetGlobal sets the global Ion instance.
func SetGlobal(ion *Ion) {
	globalMu.Lock()
	global = ion
	globalMu.Unlock()
}

// L returns the global Ion instance.
func L() *Ion {
	globalMu.RLock()
	g := global
	globalMu.RUnlock()
	if g == nil {
		panic("ion: global not set, call SetGlobal first")
	}
	return g
}

func getGlobal() *Ion {
	globalMu.RLock()
	g := global
	globalMu.RUnlock()
	if g == nil {
		return &Ion{logger: newZapLogger(Default())}
	}
	return g
}

// Debug logs at debug level using global logger.
func Debug(ctx context.Context, msg string, fields ...Field) {
	getGlobal().Debug(ctx, msg, fields...)
}

// Info logs at info level using global logger.
func Info(ctx context.Context, msg string, fields ...Field) {
	getGlobal().Info(ctx, msg, fields...)
}

// Warn logs at warn level using global logger.
func Warn(ctx context.Context, msg string, fields ...Field) {
	getGlobal().Warn(ctx, msg, fields...)
}

// Error logs at error level using global logger.
func Error(ctx context.Context, msg string, err error, fields ...Field) {
	getGlobal().Error(ctx, msg, err, fields...)
}

// Fatal logs at fatal level using global logger.
func Fatal(ctx context.Context, msg string, err error, fields ...Field) {
	getGlobal().Fatal(ctx, msg, err, fields...)
}

// GetTracer returns a named tracer from global Ion.
func GetTracer(name string) Tracer {
	return getGlobal().Tracer(name)
}

// Sync flushes the global logger.
func Sync() error {
	globalMu.RLock()
	g := global
	globalMu.RUnlock()
	if g == nil {
		return nil
	}
	return g.Sync()
}

// Named returns a child logger from global.
func Named(name string) Logger {
	return getGlobal().Named(name)
}
