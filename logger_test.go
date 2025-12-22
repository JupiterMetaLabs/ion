package ion

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNew_Default(t *testing.T) {
	logger := New(Default())
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Sync()

	// Should not panic
	logger.Info("test message", F("key", "value"))
}

func TestNew_Development(t *testing.T) {
	logger := New(Development())
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Sync()

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
}

func TestLogger_With(t *testing.T) {
	logger := New(Default())
	defer logger.Sync()

	childLogger := logger.With(F("component", "test"))
	if childLogger == nil {
		t.Fatal("expected non-nil child logger")
	}

	// Should not panic
	childLogger.Info("child message")
}

func TestLogger_Named(t *testing.T) {
	logger := New(Default())
	defer logger.Sync()

	namedLogger := logger.Named("my-component")
	if namedLogger == nil {
		t.Fatal("expected non-nil named logger")
	}

	// Should not panic
	namedLogger.Info("named message")
}

func TestLogger_WithContext(t *testing.T) {
	logger := New(Default())
	defer logger.Sync()

	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithUserID(ctx, "user-456")

	ctxLogger := logger.WithContext(ctx)
	if ctxLogger == nil {
		t.Fatal("expected non-nil context logger")
	}

	// Should not panic
	ctxLogger.Info("context message")
}

func TestLogger_AllLevels(t *testing.T) {
	logger := New(Development())
	defer logger.Sync()

	logger.Debug("debug", F("level", "debug"))
	logger.Info("info", F("level", "info"))
	logger.Warn("warn", F("level", "warn"))
	logger.Error("error", nil, F("level", "error"))
}

func TestLogger_Error_WithError(t *testing.T) {
	logger := New(Default())
	defer logger.Sync()

	testErr := &testError{msg: "test error"}
	logger.Error("operation failed", testErr, F("op", "test"))
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
	logger := New(Default())
	defer logger.Sync()

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
		wantKey  string
		wantType string
	}{
		{"F", F("key", "value"), "key", "string"},
		{"String", String("str", "val"), "str", "string"},
		{"Int", Int("num", 42), "num", "int"},
		{"Int64", Int64("big", 123456789), "big", "int64"},
		{"Float64", Float64("flt", 3.14), "flt", "float64"},
		{"Bool", Bool("flag", true), "flag", "bool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Key != tt.wantKey {
				t.Errorf("expected key '%s', got '%s'", tt.wantKey, tt.field.Key)
			}
			if tt.field.Value == nil {
				t.Error("expected non-nil value")
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
