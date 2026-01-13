package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JupiterMetaLabs/ion/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DebugOTEL enables debug logging for OTEL setup.
var DebugOTEL bool

// LogProvider manages the OpenTelemetry log provider.
type LogProvider struct {
	loggerProvider *sdklog.LoggerProvider
}

// LoggerProvider returns the underlying sdklog.LoggerProvider
func (p *LogProvider) LoggerProvider() *sdklog.LoggerProvider {
	if p == nil {
		return nil
	}
	return p.loggerProvider
}

// Shutdown shuts down the log provider.
func (p *LogProvider) Shutdown(ctx context.Context) error {
	if p == nil || p.loggerProvider == nil {
		return nil
	}
	return p.loggerProvider.Shutdown(ctx)
}

// TracerProvider wraps the OTEL TracerProvider.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
}

// Shutdown shuts down the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp == nil || tp.provider == nil {
		return nil
	}
	return tp.provider.Shutdown(ctx)
}

// SetupLogProvider initializes OpenTelemetry logging.
func SetupLogProvider(cfg config.OTELConfig, serviceName, version string) (*LogProvider, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Resources
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version),
	}
	for k, v := range cfg.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	res, err := resource.New(ctx,
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Exporter
	var exporter sdklog.Exporter
	switch cfg.Protocol {
	case "http":
		exporter, err = createHTTPLogExporter(ctx, cfg)
	default:
		exporter, err = createGRPCLogExporter(ctx, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL log exporter: %w", err)
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

	// Set global logger provider (optional, but good for libs using global API)
	global.SetLoggerProvider(provider)

	return &LogProvider{loggerProvider: provider}, nil
}

// SetupTracerProvider creates and configures the OTEL tracer provider.
func SetupTracerProvider(cfg config.TracingConfig, serviceName, version string) (*TracerProvider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Handle Basic Auth - inject Authorization header
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}
	if cfg.Username != "" && cfg.Password != "" {
		auth := fmt.Sprintf("%s:%s", cfg.Username, cfg.Password)
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		cfg.Headers["Authorization"] = "Basic " + encodedAuth
	}

	if DebugOTEL {
		// We could use an internal logger here, but for now we silence it or return warnings.
		// Retaining debug flag check but removing direct log.Printf to avoid library side effects.
		// If we want to support debugging, we should accept a logger in the config.
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Exporter
	var exporter sdktrace.SpanExporter
	switch cfg.Protocol {
	case "http":
		exporter, err = createHTTPTraceExporter(ctx, cfg)
	default:
		exporter, err = createGRPCTraceExporter(ctx, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Sampler
	sampler := parseSampler(cfg.Sampler)

	// Processor
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 512
	}
	exportInterval := cfg.ExportInterval
	if exportInterval <= 0 {
		exportInterval = 5 * time.Second
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxExportBatchSize(batchSize),
			sdktrace.WithBatchTimeout(exportInterval),
		),
		sdktrace.WithSampler(sampler),
	)

	// Set globals
	otel.SetTracerProvider(tp)

	props := []propagation.TextMapPropagator{
		propagation.TraceContext{},
		propagation.Baggage{},
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(props...))

	return &TracerProvider{provider: tp}, nil
}

// --- Helpers ---

func createGRPCLogExporter(ctx context.Context, cfg config.OTELConfig) (sdklog.Exporter, error) {
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

func createHTTPLogExporter(ctx context.Context, cfg config.OTELConfig) (sdklog.Exporter, error) {
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

func createGRPCTraceExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
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
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracegrpc.WithTimeout(cfg.Timeout))
	}
	return otlptracegrpc.New(ctx, opts...)
}

func createHTTPTraceExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracehttp.WithTimeout(cfg.Timeout))
	}
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
