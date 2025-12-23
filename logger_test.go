package ion

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNew_Default(t *testing.T) {
	ctx := context.Background()
	logger := newZapLogger(Default())
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer func() { _ = logger.Sync() }()

	// Should not panic
	logger.Info(ctx, "test message", F("key", "value"))
}

func TestNew_Development(t *testing.T) {
	ctx := context.Background()
	logger := newZapLogger(Development())
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer func() { _ = logger.Sync() }()

	logger.Debug(ctx, "debug message")
	logger.Info(ctx, "info message")
	logger.Warn(ctx, "warn message")
}

func TestLogger_With(t *testing.T) {
	ctx := context.Background()
	logger := newZapLogger(Default())
	defer func() { _ = logger.Sync() }()

	childLogger := logger.With(F("component", "test"))
	if childLogger == nil {
		t.Fatal("expected non-nil child logger")
	}

	// Should not panic
	childLogger.Info(ctx, "child message")
}

func TestLogger_Named(t *testing.T) {
	ctx := context.Background()
	logger := newZapLogger(Default())
	defer func() { _ = logger.Sync() }()

	namedLogger := logger.Named("my-component")
	if namedLogger == nil {
		t.Fatal("expected non-nil named logger")
	}

	// Should not panic
	namedLogger.Info(ctx, "named message")
}

func TestLogger_ContextExtraction(t *testing.T) {
	logger := newZapLogger(Default())
	defer func() { _ = logger.Sync() }()

	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithUserID(ctx, "user-456")

	// Context is passed directly to log methods
	// Should not panic and should extract trace fields
	logger.Info(ctx, "context message")
}

func TestLogger_AllLevels(t *testing.T) {
	ctx := context.Background()
	logger := newZapLogger(Development())
	defer func() { _ = logger.Sync() }()

	logger.Debug(ctx, "debug", F("level", "debug"))
	logger.Info(ctx, "info", F("level", "info"))
	logger.Warn(ctx, "warn", F("level", "warn"))
	logger.Error(ctx, "error", nil, F("level", "error"))
}

func TestLogger_Error_WithError(t *testing.T) {
	ctx := context.Background()
	logger := newZapLogger(Default())
	defer func() { _ = logger.Sync() }()

	testErr := &testError{msg: "test error"}
	logger.Error(ctx, "operation failed", testErr, F("op", "test"))
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Default()

	if cfg.Level != "info" {
		t.Errorf("expected level 'info', got '%s'", cfg.Level)
	}
	if !cfg.Console.Enabled {
		t.Error("expected console enabled by default")
	}
	if cfg.Console.Format != "json" {
		t.Errorf("expected console format 'json', got '%s'", cfg.Console.Format)
	}
	if cfg.File.Enabled {
		t.Error("expected file disabled by default")
	}
	if cfg.OTEL.Enabled {
		t.Error("expected OTEL disabled by default")
	}
	if cfg.OTEL.Protocol != "grpc" {
		t.Errorf("expected OTEL protocol 'grpc', got '%s'", cfg.OTEL.Protocol)
	}
}

func TestLogger_SetLevel(t *testing.T) {
	logger := newZapLogger(Default())
	defer func() { _ = logger.Sync() }()

	// Initial level is "info"
	if got := logger.GetLevel(); got != "info" {
		t.Errorf("expected initial level 'info', got '%s'", got)
	}

	// Change to debug
	logger.SetLevel("debug")
	if got := logger.GetLevel(); got != "debug" {
		t.Errorf("expected level 'debug', got '%s'", got)
	}

	// Change to error
	logger.SetLevel("error")
	if got := logger.GetLevel(); got != "error" {
		t.Errorf("expected level 'error', got '%s'", got)
	}
}

func TestConfig_Development(t *testing.T) {
	cfg := Development()

	if cfg.Level != "debug" {
		t.Errorf("expected level 'debug', got '%s'", cfg.Level)
	}
	if !cfg.Development {
		t.Error("expected development mode enabled")
	}
	if cfg.Console.Format != "pretty" {
		t.Errorf("expected console format 'pretty', got '%s'", cfg.Console.Format)
	}
}

func TestConfig_Builders(t *testing.T) {
	cfg := Default().
		WithLevel("debug").
		WithService("my-service").
		WithOTEL("localhost:4317").
		WithFile("/var/log/app.log")

	if cfg.Level != "debug" {
		t.Errorf("expected level 'debug', got '%s'", cfg.Level)
	}
	if cfg.ServiceName != "my-service" {
		t.Errorf("expected service 'my-service', got '%s'", cfg.ServiceName)
	}
	if !cfg.OTEL.Enabled {
		t.Error("expected OTEL enabled")
	}
	if cfg.OTEL.Endpoint != "localhost:4317" {
		t.Errorf("expected OTEL endpoint 'localhost:4317', got '%s'", cfg.OTEL.Endpoint)
	}
	if !cfg.File.Enabled {
		t.Error("expected file enabled")
	}
	if cfg.File.Path != "/var/log/app.log" {
		t.Errorf("expected file path '/var/log/app.log', got '%s'", cfg.File.Path)
	}
}

func TestField_Helpers(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		expected any
	}{
		{"String", F("key", "val"), "val"},
		{"Int", F("key", 123), int64(123)},
		{"Int64", F("key", int64(123)), int64(123)},
		{"Float64", F("key", 12.34), 12.34},
		{"Bool", F("key", true), int64(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.field.Type {
			case StringType:
				if tt.field.StringVal != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, tt.field.StringVal)
				}
			case Int64Type:
				if tt.field.Integer != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, tt.field.Integer)
				}
			case Float64Type:
				if tt.field.Float != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, tt.field.Float)
				}
			case BoolType:
				if tt.field.Integer != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, tt.field.Integer)
				}
			default:
				if tt.field.Interface != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, tt.field.Interface)
				}
			}
		})
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	ctx = WithRequestID(ctx, "req-123")
	if got := RequestIDFromContext(ctx); got != "req-123" {
		t.Errorf("expected request ID 'req-123', got '%s'", got)
	}

	ctx = WithUserID(ctx, "user-456")
	if got := UserIDFromContext(ctx); got != "user-456" {
		t.Errorf("expected user ID 'user-456', got '%s'", got)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "debug"},
		{"DEBUG", "debug"},
		{"info", "info"},
		{"INFO", "info"},
		{"warn", "warn"},
		{"warning", "warn"},
		{"error", "error"},
		{"ERROR", "error"},
		{"invalid", "info"}, // defaults to info
		{"", "info"},        // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			got := strings.ToLower(level.String())
			if got != tt.want {
				t.Errorf("parseLevel(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

// Silence the test output
var _ = bytes.Buffer{}

func ExampleLogger() {
	ctx := context.Background()

	// 1. Initialize the logger
	logger := newZapLogger(Development())
	defer func() { _ = logger.Sync() }()

	// 2. Log a simple message (context-first)
	logger.Info(ctx, "Hello, World!")

	// 3. Log with structured fields
	logger.Info(ctx, "User logged in",
		F("user_id", 42),
		F("ip", "192.168.1.1"),
	)
}

func ExampleLogger_contextIntegration() {
	// Initialize logger
	logger := newZapLogger(Default())
	defer func() { _ = logger.Sync() }()

	// Create a context (in a real app, this comes from the request)
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")

	// Context is ALWAYS the first parameter
	// Trace IDs are extracted automatically
	logger.Info(ctx, "Processing request")
}
