// Package main tests ion trace correlation with OTEL and Jaeger.
//
// Run with:
//
//	docker compose up -d
//	go run examples/otel-test/main.go
//	# View traces at http://localhost:16686
//	# View logs in docker compose logs otel-collector
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/JupiterMetaLabs/ion"
	"github.com/JupiterMetaLabs/ion/fields"
)

func main() {
	ctx := context.Background()

	// Configure with OTEL and tracing enabled
	cfg := ion.Config{
		ServiceName: "ion-otel-test",
		Version:     "1.0.0",
		Level:       "debug",
		Development: true,
		Console: ion.ConsoleConfig{
			Enabled: true,
			Format:  "pretty",
			Color:   true,
		},
		OTEL: ion.OTELConfig{
			Enabled:  true,
			Endpoint: "localhost:4317",
			Protocol: "grpc",
			Insecure: true,
		},
		Tracing: ion.TracingConfig{
			Enabled:  true,
			Endpoint: "localhost:4317",
			Protocol: "grpc",
			Insecure: true,
			Sampler:  "always",
		},
	}

	app, warnings, err := ion.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create ion: %v", err)
	}
	for _, w := range warnings {
		log.Printf("ion warning: %v", w)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	fmt.Println("========================================")
	fmt.Println("ION OTEL TRACE CORRELATION TEST")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("1. Log WITHOUT trace context (context.Background)")
	fmt.Println("   Expected: No trace_id/span_id in console or OTEL")
	fmt.Println()
	app.Info(ctx, "startup log without trace", ion.String("phase", "init"))

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("2. Creating span and logging WITH trace context")
	fmt.Println("   Expected:")
	fmt.Println("   - Console: trace_id, span_id as readable strings")
	fmt.Println("   - OTEL: LogRecord.TraceID, LogRecord.SpanID (not attributes)")
	fmt.Println()

	tracer := app.Tracer("otel-test")
	ctx, span := tracer.Start(ctx, "ProcessTransaction")

	app.Info(ctx, "processing transaction",
		fields.TxHash("0xabc123def456..."),
		fields.BlockHeight(18_500_000),
		ion.String("status", "pending"),
	)

	// Nested span
	ctx2, childSpan := tracer.Start(ctx, "ValidateSignature")
	app.Debug(ctx2, "validating signature",
		ion.String("algorithm", "ed25519"),
	)
	childSpan.End()

	app.Info(ctx, "transaction processed",
		fields.TxStatus("confirmed"),
		fields.LatencyMs(42.5),
	)

	span.End()

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("3. Multiple logs with same trace context")
	fmt.Println()

	ctx3, span3 := tracer.Start(context.Background(), "BatchProcess")
	for i := 1; i <= 3; i++ {
		app.Info(ctx3, "processing item",
			ion.Int("item", i),
			ion.Int("total", 3),
		)
	}
	span3.End()

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("TEST COMPLETE!")
	fmt.Println()
	fmt.Println("Check results:")
	fmt.Println("  - Console output above (trace_id/span_id should be visible)")
	fmt.Println("  - Jaeger UI: http://localhost:16686")
	fmt.Println("  - OTEL logs: docker compose logs otel-collector")
	fmt.Println("========================================")

	// Give OTEL time to flush
	time.Sleep(2 * time.Second)
}
