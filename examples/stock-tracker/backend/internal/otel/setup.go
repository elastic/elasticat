package otel

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds OTel configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
}

// Logger provides structured logging with OTel context
type Logger struct {
	serviceName string
	tracer      trace.Tracer
}

// Setup initializes OpenTelemetry with tracing and returns a shutdown function
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.OTLPEndpoint == "" {
		cfg.OTLPEndpoint = getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")
	}
	if cfg.Environment == "" {
		cfg.Environment = getEnv("OTEL_ENVIRONMENT", "development")
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create OTLP trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Create trace provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global providers
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return shutdown function
	return func(ctx context.Context) error {
		return tracerProvider.Shutdown(ctx)
	}, nil
}

// NewLogger creates a new structured logger
func NewLogger(serviceName string) *Logger {
	return &Logger{
		serviceName: serviceName,
		tracer:      otel.Tracer(serviceName),
	}
}

// Info logs an info message with optional span context
func (l *Logger) Info(ctx context.Context, msg string, attrs ...attribute.KeyValue) {
	l.logWithLevel(ctx, "INFO", msg, attrs...)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, msg string, attrs ...attribute.KeyValue) {
	l.logWithLevel(ctx, "WARN", msg, attrs...)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, msg string, attrs ...attribute.KeyValue) {
	l.logWithLevel(ctx, "ERROR", msg, attrs...)
}

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, msg string, attrs ...attribute.KeyValue) {
	l.logWithLevel(ctx, "DEBUG", msg, attrs...)
}

func (l *Logger) logWithLevel(ctx context.Context, level, msg string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	traceID := ""
	spanID := ""
	if span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}

	// Build attribute string for structured output
	attrStr := ""
	for _, attr := range attrs {
		attrStr += " " + string(attr.Key) + "=" + attr.Value.Emit()
	}

	// Log to stdout in JSON format for OTel collector to pick up
	log.Printf(`{"timestamp":"%s","level":"%s","service":"%s","message":"%s","trace_id":"%s","span_id":"%s"%s}`,
		time.Now().UTC().Format(time.RFC3339Nano),
		level,
		l.serviceName,
		msg,
		traceID,
		spanID,
		attrStr,
	)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

