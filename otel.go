package ion

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OTELProvider manages the OpenTelemetry log provider.
// It should be created and stored at application startup,
// and Shutdown() should be called before exit.
type OTELProvider struct {
	provider *sdklog.LoggerProvider
}

// SetupOTEL initializes OpenTelemetry logging with the given configuration.
// Returns an OTELProvider that must be shut down on application exit.
//
// Usage:
//
//	provider, err := ion.SetupOTEL(cfg.OTEL, cfg.ServiceName, cfg.Version)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Shutdown(context.Background())
func SetupOTEL(cfg OTELConfig, serviceName, version string) (*OTELProvider, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		return nil, nil
	}

	ctx := context.Background()

	// Build resource with service info and custom attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version),
	}

	// Add custom attributes from config
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Create exporter based on protocol
	var exporter sdklog.Exporter
	switch cfg.Protocol {
	case "http":
		exporter, err = createHTTPExporter(ctx, cfg)
	default: // "grpc" is default
		exporter, err = createGRPCExporter(ctx, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL exporter: %w", err)
	}

	// Build processor
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 512
	}
	exportInterval := cfg.ExportInterval
	if exportInterval <= 0 {
		exportInterval = 5 * time.Second
	}

	processor := sdklog.NewBatchProcessor(
		exporter,
		sdklog.WithMaxQueueSize(batchSize*2),
		sdklog.WithExportMaxBatchSize(batchSize),
		sdklog.WithExportInterval(exportInterval),
	)

	// Create provider
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	// Set as global provider
	global.SetLoggerProvider(provider)

	return &OTELProvider{provider: provider}, nil
}

// createGRPCExporter creates a gRPC-based OTEL log exporter.
func createGRPCExporter(ctx context.Context, cfg OTELConfig) (sdklog.Exporter, error) {
	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlploggrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		opts = append(opts, otlploggrpc.WithInsecure())
	}

	if cfg.Timeout > 0 {
		opts = append(opts, otlploggrpc.WithTimeout(cfg.Timeout))
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, otlploggrpc.WithHeaders(cfg.Headers))
	}

	return otlploggrpc.New(ctx, opts...)
}

// createHTTPExporter creates an HTTP-based OTEL log exporter.
func createHTTPExporter(ctx context.Context, cfg OTELConfig) (sdklog.Exporter, error) {
	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}

	if cfg.Timeout > 0 {
		opts = append(opts, otlploghttp.WithTimeout(cfg.Timeout))
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, otlploghttp.WithHeaders(cfg.Headers))
	}

	return otlploghttp.New(ctx, opts...)
}

// Shutdown gracefully shuts down the OTEL provider.
// Should be called before application exit.
func (p *OTELProvider) Shutdown(ctx context.Context) error {
	if p == nil || p.provider == nil {
		return nil
	}
	return p.provider.Shutdown(ctx)
}
