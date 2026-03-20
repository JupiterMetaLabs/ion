package ion

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// --- Ion struct tests ---

func TestIon_New(t *testing.T) {
	ctx := context.Background()
	app, _, err := New(Default())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil Ion")
	}
	// Test logging works (Ion implements Logger)
	app.Info(ctx, "test message", F("key", "value"))
	_ = app.Shutdown(ctx)
}

func TestIon_Tracer(t *testing.T) {
	ctx := context.Background()
	cfg := Default()
	cfg.Tracing.Enabled = true
	cfg.Tracing.Endpoint = "localhost:4317"
	app, _, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	// Get tracer - returns no-op since no endpoint configured
	tracer := app.Tracer("test.component")
	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}

	// Create a span (no-op but should not panic)

	ctx, span := tracer.Start(ctx, "TestOperation")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.End()
}

func TestIon_ChildLogger(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default())
	defer func() { _ = app.Shutdown(ctx) }()

	// Named child
	child := app.Named("child")
	if child == nil {
		t.Fatal("expected non-nil child logger")
	}
	child.Info(ctx, "child message")

	// With child
	withChild := app.With(String("key", "value"))
	if withChild == nil {
		t.Fatal("expected non-nil with child")
	}
	withChild.Info(ctx, "with message")
}

func TestIon_SetLevel(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default())
	defer func() { _ = app.Shutdown(ctx) }()

	// Default level is info
	if got := app.GetLevel(); got != "info" {
		t.Errorf("GetLevel() = %q, want %q", got, "info")
	}

	// Change to debug
	app.SetLevel("debug")
	if got := app.GetLevel(); got != "debug" {
		t.Errorf("GetLevel() = %q, want %q", got, "debug")
	}
}

func TestIon_Shutdown(t *testing.T) {
	ctx := context.Background()
	cfg := Default()
	cfg.Console.Enabled = false // Disable console to avoid sync errors on test pipes
	app, _, _ := New(cfg)

	// Should not error when console is disabled
	if err := app.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}
}

// --- Phase 5: New tests for Solution 4 ---

// TestIon_CallerDepth verifies that log output reports the test file
// as the caller, not ion.go or logger_impl.go.
// This is the regression test for the caller-depth bug (Issue 1).
// Note: AddCaller is only active in Development mode.
func TestIon_CallerDepth(t *testing.T) {
	// Build an observer core that captures log entries including caller info.
	// We add AddCaller + AddCallerSkip(1) to match the real config.
	obsCore, logs := observer.New(zapcore.DebugLevel)
	testZap := zap.New(obsCore, zap.AddCaller(), zap.AddCallerSkip(1))

	app := &Ion{
		zapLogger: &zapLogger{
			zap:       testZap,
			config:    Default(),
			atomicLvl: zap.NewAtomicLevelAt(zapcore.DebugLevel),
		},
	}

	ctx := context.Background()

	// Direct call on *Ion — this is the path that was broken before the fix.
	app.Info(ctx, "caller depth test") // <-- THIS line's file:line should be reported

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	caller := entry.Caller.File

	// The caller should be this test file, not ion.go or logger_impl.go
	if strings.Contains(caller, "ion.go") || strings.Contains(caller, "logger_impl.go") {
		t.Errorf("caller reported internal file %q, expected ion_test.go", caller)
	}
	if !strings.Contains(caller, "ion_test.go") {
		t.Errorf("caller = %q, want it to contain 'ion_test.go'", caller)
	}

	// Also test a child — should also report this test file
	child := app.Named("child")
	child.Info(ctx, "child caller depth test") // <-- THIS line should be reported

	if logs.Len() != 2 {
		t.Fatalf("expected 2 log entries, got %d", logs.Len())
	}

	childEntry := logs.All()[1]
	childCaller := childEntry.Caller.File
	if !strings.Contains(childCaller, "ion_test.go") {
		t.Errorf("child caller = %q, want it to contain 'ion_test.go'", childCaller)
	}
}

// TestIon_NamedPreservesObservability verifies that Named() returns
// a Logger whose concrete type is *Ion with access to Tracer and Meter.
func TestIon_NamedPreservesObservability(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default())
	defer func() { _ = app.Shutdown(ctx) }()

	child := app.Named("test-child")
	if child == nil {
		t.Fatal("Named() returned nil")
	}

	// The concrete type should be *Ion
	ionChild, ok := child.(*Ion)
	if !ok {
		t.Fatalf("Named() concrete type = %T, want *Ion", child)
	}

	// Should be able to call Tracer without panic
	tracer := ionChild.Tracer("test")
	if tracer == nil {
		t.Fatal("Tracer() returned nil on child")
	}

	// Should be able to call Meter without panic
	meter := ionChild.Meter("test")
	if meter == nil {
		t.Fatal("Meter() returned nil on child")
	}

	// Logging should still work
	ionChild.Info(ctx, "child log message")
}

// TestIon_WithPreservesObservability verifies that With() returns
// a Logger whose concrete type is *Ion with access to Tracer and Meter.
func TestIon_WithPreservesObservability(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default())
	defer func() { _ = app.Shutdown(ctx) }()

	child := app.With(String("component", "test"))
	if child == nil {
		t.Fatal("With() returned nil")
	}

	ionChild, ok := child.(*Ion)
	if !ok {
		t.Fatalf("With() concrete type = %T, want *Ion", child)
	}

	tracer := ionChild.Tracer("test")
	if tracer == nil {
		t.Fatal("Tracer() returned nil on With() child")
	}

	ionChild.Info(ctx, "with child log message")
}

// TestIon_Child verifies the Child() convenience method returns *Ion
// with full observability and correct named logger behavior.
func TestIon_Child(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default())
	defer func() { _ = app.Shutdown(ctx) }()

	child := app.Child("http", String("version", "v2"))
	if child == nil {
		t.Fatal("Child() returned nil")
	}

	// Direct *Ion — no type assertion needed
	child.Info(ctx, "request received")

	tracer := child.Tracer("http.handler")
	if tracer == nil {
		t.Fatal("Tracer() returned nil on Child()")
	}

	meter := child.Meter("http.metrics")
	if meter == nil {
		t.Fatal("Meter() returned nil on Child()")
	}
}

// TestIon_ChildChaining verifies that Child().Child() works correctly.
func TestIon_ChildChaining(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default())
	defer func() { _ = app.Shutdown(ctx) }()

	child := app.Child("http").Child("handler")
	child.Info(ctx, "chained child message")

	// Should still have observability
	tracer := child.Tracer("http.handler")
	if tracer == nil {
		t.Fatal("Tracer() returned nil on chained Child()")
	}
}

// TestIon_NamedChaining verifies Named().Named() via Logger interface
// returns *Ion at each level.
func TestIon_NamedChaining(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default())
	defer func() { _ = app.Shutdown(ctx) }()

	child := app.Named("http")
	grandchild := child.(*Ion).Named("handler")

	ionGrandchild, ok := grandchild.(*Ion)
	if !ok {
		t.Fatalf("chained Named() type = %T, want *Ion", grandchild)
	}
	ionGrandchild.Info(ctx, "grandchild message")
}

// TestIon_SetLevelPropagation verifies that SetLevel on parent
// affects children (shared atomicLvl).
func TestIon_SetLevelPropagation(t *testing.T) {
	app, _, _ := New(Default())
	child := app.Child("test")

	app.SetLevel("error")
	if got := child.GetLevel(); got != "error" {
		t.Errorf("child.GetLevel() = %q after parent SetLevel(\"error\"), want \"error\"", got)
	}

	app.SetLevel("debug")
	if got := child.GetLevel(); got != "debug" {
		t.Errorf("child.GetLevel() = %q after parent SetLevel(\"debug\"), want \"debug\"", got)
	}
}

// TestIon_InterfaceCompliance is a compile-time + runtime check
// that *Ion satisfies Logger.
func TestIon_InterfaceCompliance(t *testing.T) {
	app, _, _ := New(Default())
	var _ Logger = app // compile-time check

	// Runtime: Named() result should also satisfy Logger
	child := app.Named("test")
	if child == nil {
		t.Fatal("Named() result should satisfy Logger interface")
	}
}
