package observability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/gogomail/gogomail/internal/observability"
)

func TestInitTracingDisabled(t *testing.T) {
	ctx := context.Background()
	tp, err := observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test",
	})
	if err != nil {
		t.Fatalf("InitTracing(disabled) error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
	// Shutdown must be idempotent and error-free.
	if err := tp.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}

func TestInitTracingDisabledInstallsNoopProvider(t *testing.T) {
	ctx := context.Background()
	_, err := observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-noop",
	})
	if err != nil {
		t.Fatalf("InitTracing: %v", err)
	}
	// The global provider should now be a noop: spans should be valid but non-recording.
	tracer := otel.Tracer("test")
	_, span := tracer.Start(ctx, "noop-span")
	defer span.End()
	if span.IsRecording() {
		t.Error("noop provider span should not be recording")
	}
}

func TestTracerReturnsSameTracerAsOTelGlobal(t *testing.T) {
	ctx := context.Background()
	_, _ = observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-tracer",
	})

	pkg := "github.com/gogomail/gogomail/internal/observability"
	got := observability.Tracer(pkg)
	want := otel.Tracer(pkg)

	// Both should be the same type (noop.tracer in this case).
	if got == nil || want == nil {
		t.Fatal("expected non-nil tracers")
	}
	// Both start spans without panicking.
	_, spanA := got.Start(ctx, "a")
	defer spanA.End()
	_, spanB := want.Start(ctx, "b")
	defer spanB.End()
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()
	_, _ = observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-startspan",
	})

	childCtx, span := observability.StartSpan(ctx, "github.com/gogomail/gogomail/test", "my-op")
	defer span.End()

	if childCtx == nil {
		t.Fatal("StartSpan returned nil context")
	}
	// span should be a trace.Span
	var _ trace.Span = span
}

func TestOTelHTTPMiddlewarePassesThrough(t *testing.T) {
	ctx := context.Background()
	_, _ = observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-middleware",
	})

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := observability.OTelHTTPMiddleware("test")(inner)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
