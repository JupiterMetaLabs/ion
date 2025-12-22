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
	"github.com/JupiterMetaLabs/ion/fields"
)

// ============================================================================
// Example 1: Simple Global Logger (Singleton Pattern)
// Best for: Small apps, scripts, or teams that prefer simplicity.
// ============================================================================

func example1_GlobalLogger() {
	// One-liner setup from environment variables:
	// LOG_LEVEL, SERVICE_NAME, LOG_DEVELOPMENT, OTEL_ENDPOINT
	ion.SetGlobal(ion.InitFromEnv())
	defer func() { _ = ion.Sync() }()

	// Use package-level helpers anywhere in your code
	ion.Info("application started")
	ion.Debug("debug info", ion.String("key", "value"))
	ion.Warn("something might be wrong")

	// Child logger from global
	dbLog := ion.Named("database")
	dbLog.Info("connected to postgres")
}

// ============================================================================
// Example 2: Dependency Injection Pattern
// Best for: Libraries, large apps, or teams that prefer explicit dependencies.
// ============================================================================

func example2_DependencyInjection() {
	logger := ion.New(ion.Default().WithService("payment-api"))
	defer func() { _ = logger.Sync() }()

	// Pass logger explicitly to components
	server := NewServer(logger)
	server.Start()
}

type Server struct {
	log ion.Logger
}

func NewServer(l ion.Logger) *Server {
	// Create a child logger for this component
	return &Server{log: l.Named("server")}
}

func (s *Server) Start() {
	s.log.Info("server listening", ion.Int("port", 8080))
}

// ============================================================================
// Example 3: Child Loggers (With and Named)
// Demonstrates how to scope loggers for specific contexts.
// ============================================================================

func example3_ChildLoggers() {
	logger := ion.New(ion.Default())

	// Named: Adds a "logger" field to identify the component
	httpLog := logger.Named("http")
	grpcLog := logger.Named("grpc")

	// With: Adds permanent fields to all log entries from this child
	userLogger := logger.With(
		ion.Int("user_id", 42),
		ion.String("tenant", "acme-corp"),
	)

	httpLog.Info("request received") // {"logger": "http", ...}
	grpcLog.Info("rpc called")       // {"logger": "grpc", ...}
	userLogger.Info("action taken")  // {"user_id": 42, "tenant": "acme-corp", ...}
}

// ============================================================================
// Example 4: Context Integration (for Tracing)
// Demonstrates WithContext for OpenTelemetry trace correlation.
// ============================================================================

func example4_ContextIntegration() {
	logger := ion.New(ion.Default())

	// Simulate a request context (in real code, this comes from HTTP middleware)
	ctx := context.Background()

	// WithContext extracts trace_id and span_id from the context
	logger.WithContext(ctx).Info("processing request",
		ion.String("endpoint", "/api/v1/orders"),
	)
}

// ============================================================================
// Example 5: Blockchain Fields
// Demonstrates domain-specific field helpers.
// ============================================================================

func example5_BlockchainFields() {
	logger := ion.New(ion.Default().WithService("mempool-router"))

	logger.Info("transaction routed",
		fields.TxHash("0xabc123..."),
		fields.ShardID(3),
		fields.Slot(150_000_000),
		fields.Epoch(350),
		fields.BlockHeight(19_500_000),
		fields.LatencyMs(12.5),
	)
}

// ============================================================================
// Example 6: Production Setup with Graceful Shutdown
// The recommended pattern for real-world services.
// ============================================================================

func example6_ProductionSetup() {
	cfg := ion.Config{
		Level:       "info",
		Development: false,
		ServiceName: "order-service",
		Version:     "v2.1.0",

		Console: ion.ConsoleConfig{
			Enabled:        true,
			Format:         "json",
			ErrorsToStderr: true,
		},

		File: ion.FileConfig{
			Enabled:    true,
			Path:       "/var/log/orders/app.log",
			MaxSizeMB:  100,
			MaxBackups: 5,
			Compress:   true,
		},

		OTEL: ion.OTELConfig{
			Enabled:  true,
			Endpoint: "otel-collector:4317",
			Protocol: "grpc",
			Username: "admin",       // Optional Basic Auth
			Password: "supersecret", // Optional Basic Auth
			Attributes: map[string]string{
				"env":    "production",
				"region": "us-east-1",
			},
		},
	}

	logger, err := ion.NewWithOTEL(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}

	// Set as global for convenience
	ion.SetGlobal(logger)

	// CRITICAL: Graceful shutdown to flush all logs and traces
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := logger.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "logger shutdown error: %v\n", err)
		}
	}()

	// Run your application
	runProductionApp(logger)
}

func runProductionApp(logger ion.Logger) {
	log := logger.Named("main")
	log.Info("service started")

	// Simulate work
	time.Sleep(100 * time.Millisecond)

	// Simulate an error
	err := errors.New("database connection lost")
	log.Error("critical failure", err, ion.String("component", "db"))

	// Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Auto-exit for example purposes
		time.Sleep(200 * time.Millisecond)
		sigChan <- syscall.SIGTERM
	}()

	log.Info("waiting for shutdown signal...")
	<-sigChan
	log.Info("received shutdown signal, exiting...")
}

// ============================================================================
// Main: Run all examples
// ============================================================================

func main() {
	fmt.Println("=== Example 1: Global Logger ===")
	example1_GlobalLogger()

	fmt.Println("\n=== Example 2: Dependency Injection ===")
	example2_DependencyInjection()

	fmt.Println("\n=== Example 3: Child Loggers ===")
	example3_ChildLoggers()

	fmt.Println("\n=== Example 4: Context Integration ===")
	example4_ContextIntegration()

	fmt.Println("\n=== Example 5: Blockchain Fields ===")
	example5_BlockchainFields()

	fmt.Println("\n=== Example 6: Production Setup ===")
	example6_ProductionSetup()
}
