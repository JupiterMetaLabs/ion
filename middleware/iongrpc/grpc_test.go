package iongrpc

import (
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

func TestServerHandler(t *testing.T) {
	// ServerHandler returns a stats.Handler
	handler := ServerHandler()
	if handler == nil {
		t.Fatal("expected non-nil server handler")
	}
}

func TestClientHandler(t *testing.T) {
	// ClientHandler returns a stats.Handler
	handler := ClientHandler()
	if handler == nil {
		t.Fatal("expected non-nil client handler")
	}
}

func TestServerHandler_WithFilter(t *testing.T) {
	// Test that WithFilter option is accepted
	handler := ServerHandler(WithFilter(func(info *otelgrpc.InterceptorInfo) bool {
		return info.Method != "/grpc.health.v1.Health/Check"
	}))
	if handler == nil {
		t.Fatal("expected non-nil server handler with filter")
	}
}

func TestClientHandler_WithFilter(t *testing.T) {
	// Test that WithFilter option is accepted
	handler := ClientHandler(WithFilter(func(info *otelgrpc.InterceptorInfo) bool {
		return true
	}))
	if handler == nil {
		t.Fatal("expected non-nil client handler with filter")
	}
}
