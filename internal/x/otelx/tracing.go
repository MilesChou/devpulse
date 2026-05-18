package otelx

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Config controls TracerProvider construction.
//
// When Endpoint is empty the provider is a noop — useful for unit tests
// and one-off CLI runs that don't need to ship traces anywhere.
type Config struct {
	ServiceName    string  // semconv service.name
	ServiceVersion string  // semconv service.version
	Environment    string  // semconv deployment.environment
	Endpoint       string  // OTLP HTTP endpoint host:port, e.g. localhost:4318
	Insecure       bool    // OTLP without TLS (dev / local Jaeger)
	SampleRatio    float64 // 0.0 - 1.0; 1.0 means always sample
}

// Provider bundles the TracerProvider and a Shutdown helper. The caller
// must invoke Shutdown before exiting so any buffered spans flush.
type Provider struct {
	TracerProvider trace.TracerProvider
	shutdown       func(context.Context) error
}

// NewProvider wires the global OTel TracerProvider and TextMapPropagator
// and returns the bundle for graceful shutdown.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		tp := tracenoop.NewTracerProvider()
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.TraceContext{})
		return &Provider{
			TracerProvider: tp,
			shutdown:       func(context.Context) error { return nil },
		}, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: build resource: %w", err)
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("otel: build exporter: %w", err)
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 {
		ratio = 1.0
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return &Provider{
		TracerProvider: tp,
		shutdown:       tp.Shutdown,
	}, nil
}

// Shutdown flushes spans and tears the exporter down.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

