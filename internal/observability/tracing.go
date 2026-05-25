package observability

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracingConfig holds OpenTelemetry configuration.
type TracingConfig struct {
	// Enabled activates the OTel tracing pipeline. When false, a no-op
	// tracer is used and no spans are exported.
	Enabled bool
	// ExporterEndpoint is the OTLP HTTP endpoint (e.g., "http://localhost:4318").
	// Only used when Enabled is true.
	ExporterEndpoint string
	// ServiceName is the logical name of this service in traces.
	ServiceName string
	// ServiceVersion is the version of this service (e.g., commit SHA or tag).
	ServiceVersion string
}

// TracerProvider wraps the OTel SDK provider. Call Shutdown on process exit.
type TracerProvider struct {
	provider trace.TracerProvider
	shutdown func(context.Context) error
}

// InitTracing initialises the global OTel tracer provider. It returns a
// *TracerProvider whose Shutdown method must be called on process exit to
// flush and upload any buffered spans.
//
// When cfg.Enabled is false, a no-op provider is installed — no spans are
// recorded and no network traffic is produced.
func InitTracing(ctx context.Context, cfg TracingConfig) (*TracerProvider, error) {
	if !cfg.Enabled {
		tp := noop.NewTracerProvider()
		otel.SetTracerProvider(tp)
		return &TracerProvider{
			provider: tp,
			shutdown: func(context.Context) error { return nil },
		}, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.ExporterEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create OTLP exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: build resource: %w", err)
	}

	sdkProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		// ParentBased(AlwaysSample) honours W3C TraceContext / B3 headers from
		// upstream callers and samples all local root spans.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	// Install the W3C TraceContext + Baggage propagators globally.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otel.SetTracerProvider(sdkProvider)

	return &TracerProvider{
		provider: sdkProvider,
		shutdown: sdkProvider.Shutdown,
	}, nil
}

// Shutdown flushes and exports any buffered spans. Call this on process exit.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	return tp.shutdown(ctx)
}

// Tracer returns a named tracer from the global provider. The name should be
// the fully-qualified package name of the caller (e.g.,
// "github.com/gogomail/gogomail/internal/delivery").
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// OTelHTTPMiddleware wraps an http.Handler to create a server span for every
// request. It propagates W3C TraceContext from incoming headers so that
// distributed traces span multiple services.
//
// The span name is "<method> <pattern>" where pattern is the registered route
// pattern (or the raw URL path if unavailable).
func OTelHTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, serviceName,
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				if p := r.Pattern; p != "" {
					return r.Method + " " + p
				}
				return r.Method + " " + r.URL.Path
			}),
		)
	}
}

// StartSpan starts a child span with the given name on the tracer for pkg.
// It returns the child context (with the span attached) and the span itself.
// Callers must call span.End() when the operation is complete.
//
// Example:
//
//	ctx, span := observability.StartSpan(ctx, "github.com/.../delivery", "delivery.attempt")
//	defer span.End()
func StartSpan(ctx context.Context, pkg, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer(pkg).Start(ctx, name, opts...)
}
