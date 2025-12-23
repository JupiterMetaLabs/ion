package otel

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
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config mirrors the public OTELConfig but for internal use
type Config struct {
	Enabled        bool
	Endpoint       string
	Protocol       string
	Insecure       bool
	Timeout        time.Duration
	Headers        map[string]string
	Attributes     map[string]string
	BatchSize      int
	ExportInterval time.Duration
}

// Provider manages the OpenTelemetry log provider.
type Provider struct {
	loggerProvider *sdklog.LoggerProvider
}

// LoggerProvider returns the underlying sdklog.LoggerProvider
func (p *Provider) LoggerProvider() *sdklog.LoggerProvider {
	return p.loggerProvider
}

// Setup initializes OpenTelemetry logging.
func Setup(cfg Config, serviceName, version string) (*Provider, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		return nil, nil
	}

	ctx := context.Background()

	// Build resource attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version),
	}
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	// Create resource with explicit detectors to avoid schema URL conflicts.
	// Using resource.New() instead of resource.Merge(resource.Default(), ...)
	// prevents schema version mismatches between the SDK's internal schema
	// and our semconv import.
	res, err := resource.New(ctx,
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Create exporter
	var exporter sdklog.Exporter
	switch cfg.Protocol {
	case "http":
		exporter, err = createHTTPExporter(ctx, cfg)
	default:
		exporter, err = createGRPCExporter(ctx, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL exporter: %w", err)
	}

	// Processor
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

	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	global.SetLoggerProvider(provider)

	return &Provider{loggerProvider: provider}, nil
}

func createGRPCExporter(ctx context.Context, cfg Config) (sdklog.Exporter, error) {
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

func createHTTPExporter(ctx context.Context, cfg Config) (sdklog.Exporter, error) {
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

func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.loggerProvider == nil {
		return nil
	}
	return p.loggerProvider.Shutdown(ctx)
}
