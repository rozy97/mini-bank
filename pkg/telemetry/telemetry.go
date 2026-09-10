// Package telemetry wires up OpenTelemetry tracing for Mini Bank.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string

	// ExporterEndpoint is the OTLP/HTTP collector host:port (e.g.
	// "jaeger:4318"). When empty, spans are still generated — so trace/span
	// IDs remain available for log correlation — but are never exported
	// anywhere, which keeps running without a collector configured safe.
	ExporterEndpoint string
	Insecure         bool
}

// InitTracerProvider installs a global TracerProvider and W3C trace-context
// propagator. The returned shutdown func flushes and stops the provider; the
// caller must invoke it (e.g. during graceful shutdown).
func InitTracerProvider(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironmentNameKey.String(cfg.Environment),
	))
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}

	if cfg.ExporterEndpoint != "" {
		exporterOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.ExporterEndpoint)}
		if cfg.Insecure {
			exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
		}

		exporter, err := otlptracehttp.New(ctx, exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("create otlp trace exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
