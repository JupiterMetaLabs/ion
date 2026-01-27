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
	//nolint:ineffassign // ctx update is standard tracing pattern, even if unused here
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
