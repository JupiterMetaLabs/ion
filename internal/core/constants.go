package core

// SentinelKey is the context key for passing context.Context through zap.Reflect.
// This is used by the filter core to hide the ugly output in console/file logs,
// while allowing the OTEL bridge to extract trace IDs.
const SentinelKey = "__ion_ctx__"

// SystemFieldPrefix is the reserved prefix for internal system fields.
// Users should avoid keys starting with this prefix.
const SystemFieldPrefix = "__ion_"
