// Package trace is EXPERIMENTAL: These functions are still in flux. Its signature, behavior, or semantics may
// change without notice in upcoming releases.
// Package trace provides OpenTelemetry integration for distributed tracing.
//
// It simplifies the setup and configuration of OpenTelemetry with Jaeger as the trace exporter,
// enabling distributed tracing across microservices. The package integrates seamlessly with
// Gin for automatic HTTP request tracing and logrus for log correlation.
//
// Core Features:
//
//   - OpenTelemetry Integration: Full support for OpenTelemetry APIs and standards.
//   - Jaeger Exporter: Automatically exports traces to a Jaeger collector.
//   - Gin Middleware: Automatic HTTP request tracing with Gin instrumentation.
//   - Log Correlation: Integrates with logrus to include trace IDs in log output.
//   - Configurable Tracing: Enable/disable tracing and customize skip paths.
//   - Global Registration: Registers the tracer provider globally for all instrumented libraries.
//   - No-Op Mode: When disabled, uses no-op provider for zero overhead.
//
// Basic Usage:
//
//	import "github.com/ing-bank/golibs/pkg/trace"
//
//	// Create a tracer provider from configuration
//	cfg := &trace.Config{
//		Enabled:        true,
//		JaegerEndpoint: "http://jaeger:14268/api/traces",
//		ServiceName:    "my-service",
//		ServiceVersion: "1.0.0",
//		Environment:    "production",
//		SkipPaths:      []string{"/health", "/metrics"},
//	}
//	provider, err := trace.NewForConfig(cfg)
//	if err != nil {
//		// handle error
//	}
//	defer provider.Close(context.Background())
//
//	// Register middleware with Gin
//	router := gin.New()
//	provider.Register(router)
//
// Configuration:
//
// The Config struct controls tracing behavior:
//
//   - Enabled: Enable or disable all tracing (useful for development).
//   - JaegerEndpoint: URL of the Jaeger collector (e.g., http://jaeger:14268/api/traces).
//   - ServiceName: Logical name of the service, used to identify traces.
//   - ServiceVersion: Version of the service for correlation.
//   - Environment: Deployment environment (development, staging, production).
//   - SkipPaths: HTTP paths to exclude from tracing (e.g., health checks, metrics).
//
// Integration:
//
// The package automatically:
//   - Instruments all HTTP requests through Gin middleware
//   - Correlates logs with traces via logrus hook
//   - Propagates trace context across service boundaries
//   - Includes service information in resource attributes
//
// Related Packages:
//
// - OpenTelemetry: go.opentelemetry.io/otel - Core tracing APIs
// - Jaeger Client: github.com/jaegertracing/jaeger-client-go - Jaeger exporter
package trace

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/slices"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var registered bool

// Provider represents the tracer provider. Depending on the `config.Disabled`
// parameter, it will either use a "live" provider or a "no operations" version.
// The "no operations" means, tracing will be globally disabled.
type Provider struct {
	provider    trace.TracerProvider
	skipPaths   []string
	serviceName string
}

// NewForConfig returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func NewForConfig(cfg *Config) (*Provider, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// apply default values to the configuration
	ApplyDefaults(cfg)

	if !cfg.Enabled {
		return &Provider{provider: noop.NewTracerProvider()}, nil
	}

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerEndpoint)))
	if err != nil {
		return nil, err
	}
	// exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// tracesdk.WithSpanProcessor(tracesdk.NewBatchSpanProcessor(exp)),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			// semconv.K8SClusterName()
			// semconv.K8SContainerName()
			// semconv.K8SNamespaceName()
			// semconv.K8SNodeName()
			// semconv.K8SPodName()
			// semconv.ContainerID()
		)),
	)
	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)
	registered = true

	log.AddHook(otellogrus.NewHook(otellogrus.WithLevels(
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	)))

	return &Provider{provider: tp, skipPaths: cfg.SkipPaths, serviceName: cfg.ServiceName}, nil
}

func IsTracerProviderRegistered() bool {
	// it seems that otel does not provide a way to check if a tracer provider is registered correctly
	// otel.GetTracerProvider()
	return registered
}

func (p *Provider) RouteRegister(rg gin.IRouter, opts ...otelgin.Option) {
	defaultOptions := []otelgin.Option{otelgin.WithTracerProvider(p.provider)}
	if len(opts) == 0 {
		defaultOptions = append(defaultOptions,
			otelgin.WithPropagators(propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			)),
			otelgin.WithFilter(SkipPaths(p.skipPaths)),
		)
	}
	rg.Use(otelgin.Middleware(
		p.serviceName,
		defaultOptions...,
	))
}

func (p *Provider) Register(rg gin.IRouter) {
	p.RouteRegister(rg)
}

// Close shuts down the tracer provider only if it was not "no operations"
func (p *Provider) Close(ctx context.Context) error {
	if tp, ok := p.provider.(*tracesdk.TracerProvider); ok {
		return tp.Shutdown(ctx)
	}
	return nil
}

func SkipPaths(skipPaths []string) otelgin.Filter {
	allSkipPaths := slices.Concat(skipPaths, DefaultSkipPaths)

	return func(r *http.Request) bool {
		return !slices.Contains(allSkipPaths, r.URL.Path)
	}
}
