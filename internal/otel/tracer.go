package otel

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TracerConfig configures the tracer provider.
type TracerConfig struct {
	Enabled        bool
	Endpoint       string
	Protocol       string
	Insecure       bool
	Sampler        string
	Propagators    []string
	BatchSize      int
	ExportInterval time.Duration
	Timeout        time.Duration
	Headers        map[string]string
	Attributes     map[string]string
}

// TracerProvider wraps the OTEL TracerProvider.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
}

// Shutdown shuts down the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider != nil {
		return tp.provider.Shutdown(ctx)
	}
	return nil
}

// SetupTracer creates and configures the OTEL tracer provider.
func SetupTracer(cfg TracerConfig, serviceName, version string) (*TracerProvider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if Debug {
		log.Printf("[ion/otel] SetupTracer called: enabled=%v endpoint=%q protocol=%q sampler=%q",
			cfg.Enabled, cfg.Endpoint, cfg.Protocol, cfg.Sampler)
	}

	// Use timeout to prevent hanging indefinitely on exporter creation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	if Debug {
		log.Printf("[ion/otel] Trace resource created successfully")
	}

	// Create exporter
	var exporter sdktrace.SpanExporter
	switch cfg.Protocol {
	case "http":
		if Debug {
			log.Printf("[ion/otel] Creating HTTP trace exporter for endpoint=%q insecure=%v",
				cfg.Endpoint, cfg.Insecure)
		}
		exporter, err = createHTTPTraceExporter(ctx, cfg)
	default:
		if Debug {
			log.Printf("[ion/otel] Creating gRPC trace exporter for endpoint=%q insecure=%v",
				cfg.Endpoint, cfg.Insecure)
		}
		exporter, err = createGRPCTraceExporter(ctx, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	if Debug {
		log.Printf("[ion/otel] Trace exporter created successfully")
	}

	// Parse sampler
	sampler := parseSampler(cfg.Sampler)
	if Debug {
		log.Printf("[ion/otel] Using sampler: %q", cfg.Sampler)
	}

	// Batch processor config
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 512
	}
	exportInterval := cfg.ExportInterval
	if exportInterval <= 0 {
		exportInterval = 5 * time.Second
	}

	if Debug {
		log.Printf("[ion/otel] BatchSpanProcessor: batchSize=%d exportInterval=%v", batchSize, exportInterval)
	}

	// Create provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxExportBatchSize(batchSize),
			sdktrace.WithBatchTimeout(exportInterval),
		),
		sdktrace.WithSampler(sampler),
	)

	// Set as global
	otel.SetTracerProvider(tp)

	// Configure propagators
	props := []propagation.TextMapPropagator{
		propagation.TraceContext{},
		propagation.Baggage{},
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(props...))

	if Debug {
		log.Printf("[ion/otel] TracerProvider created and set as global")
	}

	return &TracerProvider{provider: tp}, nil
}

func createGRPCTraceExporter(ctx context.Context, cfg TracerConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	opts = append(opts, otlptracegrpc.WithTimeout(timeout))

	return otlptracegrpc.New(ctx, opts...)
}

func createHTTPTraceExporter(ctx context.Context, cfg TracerConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	opts = append(opts, otlptracehttp.WithTimeout(timeout))

	return otlptracehttp.New(ctx, opts...)
}

func parseSampler(s string) sdktrace.Sampler {
	switch {
	case s == "" || s == "always":
		return sdktrace.AlwaysSample()
	case s == "never":
		return sdktrace.NeverSample()
	case strings.HasPrefix(s, "ratio:"):
		ratioStr := strings.TrimPrefix(s, "ratio:")
		ratio, err := strconv.ParseFloat(ratioStr, 64)
		if err != nil {
			return sdktrace.AlwaysSample()
		}
		return sdktrace.TraceIDRatioBased(ratio)
	default:
		return sdktrace.AlwaysSample()
	}
}

// GetTracer returns a tracer from the global provider.
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
