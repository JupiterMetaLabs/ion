// Package grpc provides gRPC server and client instrumentation using OpenTelemetry.
//
// Server instrumentation using stats handler:
//
//	server := grpc.NewServer(
//	    grpc.StatsHandler(iongrpc.ServerHandler()),
//	)
//
// Client instrumentation using stats handler:
//
//	conn, err := grpc.Dial(addr,
//	    grpc.WithStatsHandler(iongrpc.ClientHandler()),
//	)
package iongrpc

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc/stats"
)

// ServerHandler returns a stats.Handler for gRPC server instrumentation.
// Use with grpc.StatsHandler() option when creating a gRPC server.
//
// Example:
//
//	server := grpc.NewServer(
//	    grpc.StatsHandler(iongrpc.ServerHandler()),
//	)
func ServerHandler(opts ...Option) stats.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt.apply(o)
	}

	otelOpts := []otelgrpc.Option{}
	if o.filter != nil {
		otelOpts = append(otelOpts, otelgrpc.WithInterceptorFilter(o.filter))
	}

	return otelgrpc.NewServerHandler(otelOpts...)
}

// ClientHandler returns a stats.Handler for gRPC client instrumentation.
// Use with grpc.WithStatsHandler() option when dialing.
//
// Example:
//
//	conn, err := grpc.Dial(addr,
//	    grpc.WithStatsHandler(iongrpc.ClientHandler()),
//	)
func ClientHandler(opts ...Option) stats.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt.apply(o)
	}

	otelOpts := []otelgrpc.Option{}
	if o.filter != nil {
		otelOpts = append(otelOpts, otelgrpc.WithInterceptorFilter(o.filter))
	}

	return otelgrpc.NewClientHandler(otelOpts...)
}

// --- Options ---

type options struct {
	filter otelgrpc.InterceptorFilter
}

func defaultOptions() *options {
	return &options{}
}

// Option configures gRPC instrumentation.
type Option interface {
	apply(*options)
}

type filterOption struct {
	filter otelgrpc.InterceptorFilter
}

func (f filterOption) apply(o *options) { o.filter = f.filter }

// WithFilter sets a filter function to exclude methods from tracing.
// Return false to skip tracing for the given request.
//
// Example:
//
//	iongrpc.ServerHandler(iongrpc.WithFilter(func(info *otelgrpc.InterceptorInfo) bool {
//	    return info.Method != "/grpc.health.v1.Health/Check"
//	}))
func WithFilter(filter otelgrpc.InterceptorFilter) Option {
	return filterOption{filter: filter}
}
