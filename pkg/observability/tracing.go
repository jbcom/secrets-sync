package observability

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// tracerName is the instrumentation scope for all secrets-sync spans.
const tracerName = "github.com/jbcom/secrets-sync"

// TracingConfig configures OpenTelemetry distributed tracing. It maps directly
// onto the pipeline's observability.tracing config block.
type TracingConfig struct {
	// Enabled turns tracing on. When false, Tracer() returns a no-op tracer.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Exporter selects the span exporter: "otlp-grpc", "otlp-http", "zipkin",
	// or "stdout". Defaults to "otlp-grpc".
	Exporter string `mapstructure:"exporter" yaml:"exporter"`
	// Endpoint is the exporter endpoint (e.g. "localhost:4317" for otlp-grpc,
	// "http://localhost:9411/api/v2/spans" for zipkin). Honors OTEL_EXPORTER_*
	// env vars when empty.
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint"`
	// Insecure disables transport security for OTLP exporters.
	Insecure bool `mapstructure:"insecure" yaml:"insecure"`
	// SampleRatio is the head-based sampling ratio in [0,1]. 0 means never,
	// 1 means always. Defaults to 1.0 when unset and Enabled.
	SampleRatio float64 `mapstructure:"sample_ratio" yaml:"sample_ratio"`
	// ServiceName labels the traced service. Defaults to "secrets-sync".
	ServiceName string `mapstructure:"service_name" yaml:"service_name"`
}

// TracerProvider wraps the SDK provider so callers can shut it down cleanly.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
}

// Tracer returns the secrets-sync tracer from the global provider. When tracing
// is not configured this is OpenTelemetry's no-op tracer, so span calls are
// safe and cheap regardless of configuration.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// InitTracing configures the global OpenTelemetry tracer provider from cfg and
// returns a TracerProvider whose Shutdown must be called on exit. When
// cfg.Enabled is false it installs nothing and returns a no-op handle, so
// callers can unconditionally defer Shutdown.
func InitTracing(ctx context.Context, cfg TracingConfig) (*TracerProvider, error) {
	if !cfg.Enabled {
		return &TracerProvider{}, nil
	}

	exp, err := buildExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "secrets-sync"
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		return nil, fmt.Errorf("build trace resource: %w", err)
	}

	ratio := cfg.SampleRatio
	if ratio == 0 {
		ratio = 1.0
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return &TracerProvider{provider: tp}, nil
}

func buildExporter(ctx context.Context, cfg TracingConfig) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(cfg.Exporter) {
	case "", "otlp-grpc":
		opts := []otlptracegrpc.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)
	case "otlp-http":
		opts := []otlptracehttp.Option{}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	case "zipkin":
		return zipkin.New(cfg.Endpoint)
	case "stdout":
		return stdouttrace.New()
	default:
		return nil, fmt.Errorf("unsupported trace exporter %q (want otlp-grpc, otlp-http, zipkin, or stdout)", cfg.Exporter)
	}
}

// Shutdown flushes and stops the tracer provider. It is safe to call on a nil
// or no-op handle.
func (t *TracerProvider) Shutdown(ctx context.Context) error {
	if t == nil || t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

// Span attribute keys for secrets-sync spans.
const (
	AttrPhase     = attribute.Key("secrets_sync.phase")
	AttrTarget    = attribute.Key("secrets_sync.target")
	AttrSource    = attribute.Key("secrets_sync.source")
	AttrOperation = attribute.Key("secrets_sync.operation")
	AttrDriver    = attribute.Key("secrets_sync.driver")
)

// StartPhaseSpan starts a span for a pipeline phase (merge/sync) on a target.
// The returned context carries the span; the caller must End it.
func StartPhaseSpan(ctx context.Context, phase, target string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, "pipeline."+phase,
		trace.WithAttributes(AttrPhase.String(phase), AttrTarget.String(target)),
	)
}

// StartBackendSpan starts a span for a backend API call (driver + operation).
func StartBackendSpan(ctx context.Context, driver, operation string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, "backend."+operation,
		trace.WithAttributes(AttrDriver.String(driver), AttrOperation.String(operation)),
	)
}
