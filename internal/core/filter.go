// Package core provides the internal implementation of Ion's logging and tracing.
package core

import "go.uber.org/zap/zapcore"

// filteringCore wraps a zapcore.Core to filter out specific field keys.
type filteringCore struct {
	zapcore.Core
	filterKeys []string
}

// NewFilteringCore creates a core that filters out specific keys.
func NewFilteringCore(core zapcore.Core, keys ...string) zapcore.Core {
	return &filteringCore{Core: core, filterKeys: keys}
}

func (c *filteringCore) With(fields []zapcore.Field) zapcore.Core {
	filtered := make([]zapcore.Field, 0, len(fields))
	for _, f := range fields {
		if !c.shouldFilter(f.Key) {
			filtered = append(filtered, f)
		}
	}
	return &filteringCore{Core: c.Core.With(filtered), filterKeys: c.filterKeys}
}

func (c *filteringCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *filteringCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	filtered := make([]zapcore.Field, 0, len(fields))
	for _, f := range fields {
		if !c.shouldFilter(f.Key) {
			filtered = append(filtered, f)
		}
	}
	return c.Core.Write(entry, filtered)
}

func (c *filteringCore) shouldFilter(key string) bool {
	for _, k := range c.filterKeys {
		if k == key {
			return true
		}
	}
	return false
}
