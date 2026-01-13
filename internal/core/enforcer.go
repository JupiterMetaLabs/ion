package core

import "go.uber.org/zap/zapcore"

// levelEnforcer wraps a Core and overrides its Enabled check
// to respect the provided LevelEnabler (e.g. AtomicLevel).
// This is useful when a wrapped core (like OtelZap) defaults to Info
// but we want to force it to respect the global config level (e.g. Debug).
type levelEnforcer struct {
	zapcore.Core
	level zapcore.LevelEnabler
}

func (l *levelEnforcer) Enabled(lvl zapcore.Level) bool {
	return l.level.Enabled(lvl)
}

func (l *levelEnforcer) With(fields []zapcore.Field) zapcore.Core {
	return &levelEnforcer{
		Core:  l.Core.With(fields),
		level: l.level,
	}
}

func (l *levelEnforcer) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if l.Enabled(ent.Level) {
		return ce.AddCore(ent, l)
	}
	return ce
}
