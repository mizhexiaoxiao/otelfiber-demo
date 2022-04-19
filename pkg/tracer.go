package pkg

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
)

func InitializeGlobalTracer(serviceName, endpointURL string) func() {
	ctx := context.Background()
	//exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(endpointURL)))
	// stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	//exporter, err := stdout.New(stdout.WithPrettyPrint())

	// init otel-agent config
	otelAgentAddr, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if !ok {
		otelAgentAddr = "0.0.0.0:4317"
	}
	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(otelAgentAddr),
		otlptracegrpc.WithDialOption(grpc.WithBlock()))

	exporter, err := otlptrace.New(ctx, traceClient)
	if err != nil {
		log.Fatal(err)
	}

	// init provider
	tp, err := tracerProvider(exporter, serviceName)
	if err != nil {
		log.Fatal(err)
	}

	// Register our TracerProvider and Propagator as the global
	// so any imported instrumentation in the future will default
	// to using it.
	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return func() {
		handerErr(tp.Shutdown(ctx), "failed to shutdown provider")
		handerErr(exporter.Shutdown(ctx), "failed to stop exporter")
	}
}

func tracerProvider(exp tracesdk.SpanExporter, service string) (*tracesdk.TracerProvider, error) {
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
		)),
	)
	return tp, nil
}

func handerErr(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}
