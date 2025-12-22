package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JupiterMetaLabs/ion"
)

func main() {
	// 1. Production Configuration
	// In a real app, load this from YAML/JSON or Env Vars.
	cfg := ion.Config{
		Level:       "info",
		Development: false, // Use JSON in production (via Console.Format default)
		ServiceName: "payment-service",
		Version:     "v1.2.0",

		// Configure Console (JSON default for prod)
		Console: ion.ConsoleConfig{
			Enabled: true,
			Format:  "json",
		},

		// Configure File Rotation (Lumberjack)
		File: ion.FileConfig{
			Enabled:    true,
			Path:       "/var/log/payment/app.log",
			MaxSizeMB:  100, // MB
			MaxBackups: 5,
			MaxAgeDays: 30, // Days
			Compress:   true,
		},

		// Configure OpenTelemetry (OTLP)
		OTEL: ion.OTELConfig{
			Enabled:        true,
			Endpoint:       "otel-collector:4317",
			Protocol:       "grpc",
			BatchSize:      1000,
			ExportInterval: 5 * time.Second,
			Attributes: map[string]string{
				"env":    "production",
				"region": "us-east-1",
			},
		},
	}

	// 2. Initialize Logger
	logger, err := ion.NewWithOTEL(cfg)
	if err != nil {
		panic(err)
	}

	// 3. Set Global (Optional, for legacy code compatibility)
	ion.SetGlobal(logger)

	// 4. Ensure Flush on Exit using defer
	// This is critical for not losing the last few logs/traces.
	// We use a context with timeout to avoid blocking forever.
	// Note: We use a wrapper function for the defer to handle the error return
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := logger.Shutdown(ctx); err != nil {
			fmt.Printf("Failed to shutdown logger: %v\n", err)
		}
	}()

	// 5. Usage (Dependency Injection preferred)
	runApp(logger)
}

func runApp(logger ion.Logger) {
	// Create a child logger with common fields
	log := logger.With(ion.String("module", "http_server"))

	log.Info("server starting",
		ion.Int("port", 8080),
		ion.String("mode", "production"),
	)

	// Simulate some work
	time.Sleep(100 * time.Millisecond)

	// Simulate an error
	err := errors.New("failed to connect to external service")
	log.Error("service error", err, ion.String("service_name", "payment_gateway"))

	// Simulate graceful shutdown handling
	// In a real app, you'd integrate this with your HTTP server's Shutdown method
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Simulate exit after a few seconds for the example
		time.Sleep(200 * time.Millisecond)
		sigChan <- syscall.SIGTERM
	}()

	log.Info("Waiting for signal (or auto-exit)...")
	<-sigChan
	log.Info("shutting down application...")
}
