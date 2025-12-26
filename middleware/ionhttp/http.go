// Package http provides HTTP server and client instrumentation using OpenTelemetry.
//
// Server middleware creates spans for incoming requests:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/api", handler)
//	instrumented := ionhttp.Handler(mux, "my-service")
//	http.ListenAndServe(":8080", instrumented)
//
// Client instrumentation wraps an http.Client:
//
//	client := ionhttp.Client()
//	resp, err := client.Get("https://api.example.com")
package ionhttp

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Handler wraps an http.Handler with OpenTelemetry instrumentation.
// It creates spans for each incoming request with attributes for:
// - HTTP method
// - URL path
// - Status code
// - Request/response size
func Handler(handler http.Handler, operation string, opts ...Option) http.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt.apply(o)
	}

	otelOpts := []otelhttp.Option{}
	if o.filter != nil {
		otelOpts = append(otelOpts, otelhttp.WithFilter(o.filter))
	}

	return otelhttp.NewHandler(handler, operation, otelOpts...)
}

// Client returns an HTTP client instrumented with OpenTelemetry.
// Each request creates a client span linked to the current trace context.
func Client(opts ...Option) *http.Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt.apply(o)
	}

	transport := otelhttp.NewTransport(http.DefaultTransport)
	return &http.Client{Transport: transport}
}

// Transport returns an http.RoundTripper instrumented with OpenTelemetry.
// Use this to instrument custom transports.
func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

// --- Options ---

type options struct {
	filter otelhttp.Filter
}

func defaultOptions() *options {
	return &options{}
}

// Option configures HTTP instrumentation.
type Option interface {
	apply(*options)
}

type filterOption struct {
	filter otelhttp.Filter
}

func (f filterOption) apply(o *options) { o.filter = f.filter }

// WithFilter sets a filter function to exclude requests from tracing.
// Return true to include the request, false to skip.
//
// Example:
//
//	ionhttp.Handler(mux, "api", ionhttp.WithFilter(func(r *http.Request) bool {
//	    return r.URL.Path != "/health"
//	}))
func WithFilter(filter func(r *http.Request) bool) Option {
	return filterOption{filter: otelhttp.Filter(filter)}
}
