// filtercore.go provides a zapcore.Core wrapper that filters specific fields.
//
// Used for trace correlation with collision avoidance:
// - Console/File: filters sentinel key "__ion_ctx__" (ugly {} output from zap.Reflect)
// - OTEL: filters "trace_id"/"span_id" strings (redundant, LogRecord has them via bridge)
//
// The sentinel key prevents accidental collision with user-defined field names.
package ion

import "go.uber.org/zap/zapcore"

// filteringCore wraps a zapcore.Core to filter out specific field keys.
type filteringCore struct {
	zapcore.Core
	filterKeys []string
}

func newFilteringCore(core zapcore.Core, keys ...string) zapcore.Core {
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
