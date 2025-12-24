package ion

import (
	"context"
	"testing"
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
	defer app.Shutdown(ctx)

	// Test logging works (Ion implements Logger)
	app.Info(ctx, "test message", F("key", "value"))
}

func TestIon_Tracer(t *testing.T) {
	ctx := context.Background()
	cfg := Default()
	cfg.Tracing.Enabled = true
	app, _, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer app.Shutdown(ctx)

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
	defer app.Shutdown(ctx)

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
	defer app.Shutdown(ctx)

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
	app, _, _ := New(Default())

	// Should not error
	if err := app.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}
}

func TestGlobal_SetAndGet(t *testing.T) {
	ctx := context.Background()
	app, _, _ := New(Default().WithService("test-global"))
	SetGlobal(app)
	defer app.Shutdown(ctx)

	// Global logging should work
	Info(ctx, "global info")
	Debug(ctx, "global debug")

	// L() should return the same instance
	got := L()
	if got != app {
		t.Error("L() did not return the global instance")
	}

	// GetTracer should work
	tracer := GetTracer("global.test")
	if tracer == nil {
		t.Fatal("expected non-nil global tracer")
	}
}

func TestGlobal_Fallback(t *testing.T) {
	// Test that getGlobal() returns a working fallback when global is nil
	// Note: We cannot truly reset sync.Once, so we test the behavior indirectly
	// by verifying package-level functions work without panic even before SetGlobal
	ctx := context.Background()

	// Save current global
	globalMu.Lock()
	savedGlobal := global
	savedFallback := fallbackIon
	global = nil
	globalMu.Unlock()

	// Restore after test
	defer func() {
		globalMu.Lock()
		global = savedGlobal
		fallbackIon = savedFallback
		globalMu.Unlock()
	}()

	// This should use fallback (or previously created fallback), not panic
	Info(ctx, "fallback test")

	// Verify fallback was used (fallbackIon should now be set if it wasn't before)
	if global == nil && fallbackIon == nil {
		t.Error("expected fallbackIon to be created")
	}
}
