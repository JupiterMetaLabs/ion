package ion

import (
	"context"
	"sync"
	"testing"
)

// --- Ion struct tests ---

func TestIon_New(t *testing.T) {
	ctx := context.Background()
	app, err := New(Default())
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
	app, err := New(cfg)
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
	app, _ := New(Default())
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
	app, _ := New(Default())
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
	app, _ := New(Default())

	// Should not error
	if err := app.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}
}

func TestGlobal_SetAndGet(t *testing.T) {
	ctx := context.Background()
	app, _ := New(Default().WithService("test-global"))
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
	// Reset global to test fallback
	globalMu.Lock()
	global = nil
	globalMu.Unlock()
	fallbackOnce = sync.Once{}
	fallbackIon = nil

	ctx := context.Background()
	// Should use fallback, not panic
	Info(ctx, "fallback test")
}
