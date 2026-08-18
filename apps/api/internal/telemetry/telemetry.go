package telemetry

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type ShutdownFunc func(context.Context) error

// Init is a no-op when otlpEndpoint is empty so Compose without Collector still runs.
func Init(ctx context.Context, serviceName, otlpEndpoint string) (ShutdownFunc, error) {
	noop := func(context.Context) error { return nil }
	if otlpEndpoint == "" {
		return noop, nil
	}
	endpoint := strings.TrimRight(strings.TrimSpace(otlpEndpoint), "/")
	// A trailing slash makes the OTLP HTTP exporter POST to "/" (404) instead of /v1/traces.
	if !strings.HasSuffix(endpoint, "/v1/traces") {
		endpoint += "/v1/traces"
	}
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, err
	}
	traceExp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}
	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExp),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp.Shutdown, nil
}
