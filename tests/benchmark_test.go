package ion_test

import (
	"testing"

	"github.com/JupiterMetaLabs/ion"
)

func BenchmarkAllocations(b *testing.B) {
	// Setup silent logger
	cfg := ion.Default()
	cfg.Console.Enabled = false // Disable output to test core logic only
	logger := ion.New(cfg)

	// Create a huge slice to ensure pool is engaged if we were verifying that
	// But here we verify zero-alloc field creation
	b.Run("Field_Int", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info("test", ion.Int("key", 123))
		}
	})

	b.Run("Field_String", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info("test", ion.String("key", "val"))
		}
	})

	b.Run("Field_F_Int", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// F accepts any, so this WILL allocate (box the int)
			logger.Info("test", ion.F("key", 123))
		}
	})

	b.Run("Complex_Usage", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info("transaction processed",
				ion.Int("user_id", 12345),
				ion.String("status", "ok"),
				ion.Float64("latency", 10.5),
			)
		}
	})
}

func BenchmarkZapPool(b *testing.B) {
	// Verify sync.Pool reduction
	// We need to bypass the "Enabled" check to force encoding?
	// Config logic: if !Enabled, it might skip core.
	// So we need a NopCore or a DiscardCore to measuring encoding cost?
	// The current Default() with Enabled=false creates a NopCore in buildLogger because no cores are added.
	// NopCore returns immediately.
	// To strictly test allocation of the `toZapFields` (our pool logic), we need a real core that discards.

	// Hack: Configure file output to /dev/null to force conversion logic?
	// Or just trust the profile. Use existing Logger logic.

	// Actually, if we use Console.Enabled=false and no File, buildLogger returns zapcore.NewNopCore().
	// zap.Logger.Info checks if core is enabled. NopCore is disabled for all levels?
	// Wait, zapcore.NewNopCore() returns a core that is enabled for nothing?
	// Or enabled for everything but does nothing?
	// Checked: zapcore.NewNopCore() -> Write does nothing, Check returns checked entry that writes nothing.

	// To test OUR logic (the zapLogger wrapper methods), we need to ensure l.zap methods are called.
	// wrapper methods:
	// func (l *zapLogger) Info(msg string, fields ...Field) {
	//    zapFields := toZapFieldsTransient(fields) <-- THIS IS WHAT WE WANT TO BENCHMARK
	//    if zapFields != nil {
	//        l.zap.Info(msg, *zapFields...) <-- Zap will check Core here.

	// So even if Core is Nop, our wrapper executes `toZapFieldsTransient`.
	// So the benchmark IS valid for measuring OUR allocation overhead.

	cfg := ion.Default()
	cfg.Console.Enabled = false
	logger := ion.New(cfg)

	b.Run("Pool_Reuse", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			logger.Info("benchmark", ion.Int("a", 1), ion.Int("b", 2), ion.Int("c", 3))
		}
	})
}
