package ion_test

import (
	"context"
	"testing"

	"github.com/JupiterMetaLabs/ion"
)

// BenchmarkAllocations measures allocation overhead for different field creation patterns.
// Note: context.Background() is short-circuited (no extraction), so this measures the minimal path.
func BenchmarkAllocations(b *testing.B) {
	ctx := context.Background()

	// Setup silent logger - Console.Enabled=false creates a NopCore.
	// Even with NopCore, our wrapper methods (toZapFieldsTransient, etc.) still execute,
	// so we're measuring the ion overhead, not Zap's encoding.
	cfg := ion.Default()
	cfg.Console.Enabled = false
	logger := ion.New(cfg)

	b.Run("Field_Int", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info(ctx, "test", ion.Int("key", 123))
		}
	})

	b.Run("Field_String", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info(ctx, "test", ion.String("key", "val"))
		}
	})

	b.Run("Field_F_Int", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// F() accepts any, so this WILL allocate (boxing the int to interface{})
			logger.Info(ctx, "test", ion.F("key", 123))
		}
	})

	b.Run("Complex_Usage", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info(ctx, "transaction processed",
				ion.Int("user_id", 12345),
				ion.String("status", "ok"),
				ion.Float64("latency", 10.5),
			)
		}
	})
}

// BenchmarkContextExtraction measures the allocation cost when context contains trace info.
// This is the realistic production path where trace_id/span_id are extracted.
func BenchmarkContextExtraction(b *testing.B) {
	cfg := ion.Default()
	cfg.Console.Enabled = false
	logger := ion.New(cfg)

	// Create context WITH trace info (not Background, so extraction happens)
	baseCtx := context.Background()
	ctxWithTrace := ion.WithRequestID(baseCtx, "req-12345")
	ctxWithTrace = ion.WithUserID(ctxWithTrace, "user-67890")

	b.Run("Background_NoExtraction", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			logger.Info(ctx, "test", ion.String("key", "val"))
		}
	})

	b.Run("WithTraceContext", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info(ctxWithTrace, "test", ion.String("key", "val"))
		}
	})
}

// BenchmarkZapPool verifies that the sync.Pool for zap.Field slices is working correctly.
// Pool reuse reduces allocations when logging with multiple fields.
//
// Implementation note:
// - toZapFieldsTransient() gets a slice from pool, populates it, and putZapFields() returns it
// - Even with NopCore (no actual output), we measure our wrapper overhead
// - The pool is sized for 16 fields by default
func BenchmarkZapPool(b *testing.B) {
	ctx := context.Background()

	cfg := ion.Default()
	cfg.Console.Enabled = false
	logger := ion.New(cfg)

	b.Run("Pool_Reuse", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info(ctx, "benchmark", ion.Int("a", 1), ion.Int("b", 2), ion.Int("c", 3))
		}
	})
}
