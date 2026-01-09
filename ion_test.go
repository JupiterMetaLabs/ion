package ion

import (
	"context"
	"sync"
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
	cfg.Tracing.Endpoint = "localhost:4317"
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
	cfg := Default()
	cfg.Console.Enabled = false // Disable console to avoid sync errors on test pipes
	app, _, _ := New(cfg)

	// Should not error when console is disabled
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

	// GetGlobal() should return the same instance
	got := GetGlobal()
	if got != app {
		t.Error("GetGlobal() did not return the global instance")
	}

	// GetTracer should work (ion.go implementation)
	tracer := GetTracer("global.test")
	if tracer == nil {
		t.Fatal("expected non-nil global tracer")
	}
}

func TestGlobal_Fallback(t *testing.T) {
	// Test that GetGlobal() returns a working fallback when global is nil
	ctx := context.Background()

	// Save current global
	globalMu.Lock()
	savedGlobal := globalLogger
	// Reset global
	globalLogger = nil
	globalOnce = sync.Once{} // Reset once (requires re-init if we used variables for Once, but globalOnce is package var)
	// We can't easily reset sync.Once if it's a global var without unsafe or reflection.
	// However, GetGlobal checks globalLogger == nil. sync.Once is for the warning.
	// The fallback logic in GetGlobal() creates a new logger if global is nil.
	globalMu.Unlock()

	// Restore after test
	defer func() {
		globalMu.Lock()
		globalLogger = savedGlobal
		globalMu.Unlock()
	}()

	// This should use fallback, not panic
	Info(ctx, "fallback test")

	// Check GetGlobal returns non-nil
	if GetGlobal() == nil {
		t.Error("expected fallback logger")
	}
}

func TestGetGlobal_NoSideEffects(t *testing.T) {
	// Ensure GetGlobal doesn't allocate/panic/create heavy objects if SetGlobal wasn't called.
	// We specifically want to ensure it's safe to call repeatedly.

	// Reset global for this test (using lock to be safe, though tests run sequentially mostly)
	globalMu.Lock()
	savedGlobal := globalLogger
	globalLogger = nil
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalLogger = savedGlobal
		globalMu.Unlock()
	}()

	l1 := GetGlobal()
	l2 := GetGlobal()

	if l1 == nil {
		t.Fatal("GetGlobal returned nil")
	}
	if l1 != l2 {
		t.Error("GetGlobal should return stable instance (e.g. static nop) when not set")
	}
}
