package ion

import (
	"os"
	"time"
)

// Config holds the complete logger configuration.
type Config struct {
	// Level sets the minimum log level: debug, info, warn, error.
	// Default: "info"
	Level string `yaml:"level" json:"level" env:"LOG_LEVEL"`

	// Development enables development mode with:
	// - Pretty console output by default
	// - Caller information in logs
	// - Stack traces on error/fatal
	Development bool `yaml:"development" json:"development" env:"LOG_DEVELOPMENT"`

	// ServiceName identifies this service in logs and OTEL.
	// Default: "unknown"
	ServiceName string `yaml:"service_name" json:"service_name" env:"SERVICE_NAME"`

	// Version is the application version, included in logs.
	Version string `yaml:"version" json:"version" env:"SERVICE_VERSION"`

	// Console output configuration.
	Console ConsoleConfig `yaml:"console" json:"console"`

	// File output configuration (with rotation).
	File FileConfig `yaml:"file" json:"file"`

	// OTEL (OpenTelemetry) exporter configuration.
	OTEL OTELConfig `yaml:"otel" json:"otel"`
}

// ConsoleConfig configures console (stdout/stderr) output.
type ConsoleConfig struct {
	// Enabled controls whether console output is active.
	// Default: true
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Format: "json" for structured JSON, "pretty" for human-readable.
	// Default: "json" (production), "pretty" (development)
	Format string `yaml:"format" json:"format"`

	// Color enables ANSI colors in pretty format.
	// Default: true
	Color bool `yaml:"color" json:"color"`

	// ErrorsToStderr sends warn/error/fatal to stderr, others to stdout.
	// Default: true
	ErrorsToStderr bool `yaml:"errors_to_stderr" json:"errors_to_stderr"`
}

// FileConfig configures file output with rotation.
type FileConfig struct {
	// Enabled controls whether file output is active.
	// Default: false
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Path is the log file path.
	// Example: "/var/log/app/app.log"
	Path string `yaml:"path" json:"path"`

	// MaxSizeMB is the maximum size in MB before rotation.
	// Default: 100
	MaxSizeMB int `yaml:"max_size_mb" json:"max_size_mb"`

	// MaxAgeDays is the maximum age in days to retain old logs.
	// Default: 7
	MaxAgeDays int `yaml:"max_age_days" json:"max_age_days"`

	// MaxBackups is the maximum number of old log files to keep.
	// Default: 5
	MaxBackups int `yaml:"max_backups" json:"max_backups"`

	// Compress enables gzip compression of rotated log files.
	// Default: true
	Compress bool `yaml:"compress" json:"compress"`
}

// OTELConfig configures OpenTelemetry log export.
type OTELConfig struct {
	// Enabled controls whether OTEL export is active.
	// Default: false
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Protocol: "grpc" or "http". gRPC is recommended for performance.
	// Default: "grpc"
	Protocol string `yaml:"protocol" json:"protocol"`

	// Endpoint is the OTEL collector endpoint.
	// Examples: "localhost:4317" (gRPC), "localhost:4318/v1/logs" (HTTP)
	Endpoint string `yaml:"endpoint" json:"endpoint"`

	// Insecure disables TLS for the connection.
	// Default: false
	Insecure bool `yaml:"insecure" json:"insecure"`

	// Username for Basic Authentication (optional).
	Username string `yaml:"username" json:"username" env:"OTEL_USERNAME"`

	// Password for Basic Authentication (optional).
	Password string `yaml:"password" json:"password" env:"OTEL_PASSWORD"`

	// Headers are additional headers to send (e.g., auth tokens).
	Headers map[string]string `yaml:"headers" json:"headers"`

	// Timeout is the export timeout.
	// Default: 10s
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// BatchSize is the number of logs per export batch.
	// Default: 512
	BatchSize int `yaml:"batch_size" json:"batch_size"`

	// ExportInterval is how often to export batched logs.
	// Default: 5s
	ExportInterval time.Duration `yaml:"export_interval" json:"export_interval"`

	// Attributes are additional resource attributes for OTEL.
	// Example: {"environment": "production", "chain": "solana"}
	Attributes map[string]string `yaml:"attributes" json:"attributes"`
}

// Default returns a Config with sensible production defaults.
func Default() Config {
	return Config{
		Level:       "info",
		Development: false,
		ServiceName: "unknown",
		Console: ConsoleConfig{
			Enabled:        true,
			Format:         "json",
			Color:          true,
			ErrorsToStderr: true,
		},
		File: FileConfig{
			Enabled:    false,
			MaxSizeMB:  100,
			MaxAgeDays: 7,
			MaxBackups: 5,
			Compress:   true,
		},
		OTEL: OTELConfig{
			Enabled:        false,
			Protocol:       "grpc",
			Insecure:       false,
			Timeout:        10 * time.Second,
			BatchSize:      512,
			ExportInterval: 5 * time.Second,
		},
	}
}

// Development returns a Config optimized for development.
func Development() Config {
	cfg := Default()
	cfg.Level = "debug"
	cfg.Development = true
	cfg.Console.Format = "pretty"
	return cfg
}

// WithLevel returns a copy of the config with the specified level.
func (c Config) WithLevel(level string) Config {
	c.Level = level
	return c
}

// WithService returns a copy of the config with the specified service name.
func (c Config) WithService(name string) Config {
	c.ServiceName = name
	return c
}

// WithOTEL returns a copy of the config with OTEL enabled.
func (c Config) WithOTEL(endpoint string) Config {
	c.OTEL.Enabled = true
	c.OTEL.Endpoint = endpoint
	return c
}

// WithFile returns a copy of the config with file logging enabled.
func (c Config) WithFile(path string) Config {
	c.File.Enabled = true
	c.File.Path = path
	return c
}

// InitFromEnv initializes a logger using environment variables.
// This is the recommended way to configure ion in production deployments.
//
// Supported variables:
//   - LOG_LEVEL: debug, info, warn, error (default: info)
//   - LOG_DEVELOPMENT: "true" for pretty console output
//   - SERVICE_NAME: name of the service (default: unknown)
//   - SERVICE_VERSION: version of the service
//   - OTEL_ENDPOINT: collector address, enables OTEL if set (e.g., "localhost:4317")
//   - OTEL_INSECURE: "true" to disable TLS for OTEL connection
//   - OTEL_USERNAME: Basic Auth username for OTEL collector
//   - OTEL_PASSWORD: Basic Auth password for OTEL collector
func InitFromEnv() Logger {
	cfg := Default()

	// Core settings
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		cfg.Level = val
	}
	if val := os.Getenv("SERVICE_NAME"); val != "" {
		cfg.ServiceName = val
	}
	if val := os.Getenv("SERVICE_VERSION"); val != "" {
		cfg.Version = val
	}
	if os.Getenv("LOG_DEVELOPMENT") == "true" {
		cfg.Development = true
		cfg.Console.Format = "pretty"
	}

	// OTEL settings
	if val := os.Getenv("OTEL_ENDPOINT"); val != "" {
		cfg.OTEL.Enabled = true
		cfg.OTEL.Endpoint = val
	}
	if os.Getenv("OTEL_INSECURE") == "true" {
		cfg.OTEL.Insecure = true
	}
	if val := os.Getenv("OTEL_USERNAME"); val != "" {
		cfg.OTEL.Username = val
	}
	if val := os.Getenv("OTEL_PASSWORD"); val != "" {
		cfg.OTEL.Password = val
	}

	// Create logger with OTEL if configured
	if cfg.OTEL.Enabled && cfg.OTEL.Endpoint != "" {
		l, err := NewWithOTEL(cfg)
		if err == nil {
			return l
		}
	}

	return New(cfg)
}
