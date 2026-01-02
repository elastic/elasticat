package otlp

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/andrewvc/turboelasticat/internal/watch"
)

// Client sends logs to an OTLP endpoint
type Client struct {
	provider *sdklog.LoggerProvider
	logger   log.Logger
	endpoint string
}

// Config holds OTLP client configuration
type Config struct {
	Endpoint    string // OTLP HTTP endpoint (default: localhost:4318)
	ServiceName string // Default service name
	Insecure    bool   // Use HTTP instead of HTTPS
}

// New creates a new OTLP client
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4318"
	}
	// Don't default service name - let it come from the parsed log

	ctx := context.Background()

	// Create OTLP HTTP exporter
	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}

	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource - only set service name if explicitly provided
	// The per-log service name is set as an attribute in SendLog
	var attrs []attribute.KeyValue
	if cfg.ServiceName != "" {
		attrs = append(attrs, semconv.ServiceName(cfg.ServiceName))
	}
	res := resource.NewWithAttributes(semconv.SchemaURL, attrs...)

	// Create logger provider
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	logger := provider.Logger("telasticat")

	return &Client{
		provider: provider,
		logger:   logger,
		endpoint: cfg.Endpoint,
	}, nil
}

// SendLog sends a parsed log to the OTLP endpoint
func (c *Client) SendLog(ctx context.Context, parsed watch.ParsedLog) {
	var record log.Record

	// Set timestamp
	record.SetTimestamp(parsed.Timestamp)
	record.SetObservedTimestamp(time.Now())

	// Set severity
	record.SetSeverity(levelToSeverity(parsed.Level))
	record.SetSeverityText(string(parsed.Level))

	// Set body
	record.SetBody(log.StringValue(parsed.Message))

	// Add attributes
	record.AddAttributes(
		log.String("service.name", parsed.Service),
		log.String("log.source", parsed.Source),
		log.Bool("log.is_json", parsed.IsJSON),
	)

	// Add parsed attributes from JSON logs
	for k, v := range parsed.Attributes {
		switch val := v.(type) {
		case string:
			record.AddAttributes(log.String(k, val))
		case float64:
			record.AddAttributes(log.Float64(k, val))
		case bool:
			record.AddAttributes(log.Bool(k, val))
		case int:
			record.AddAttributes(log.Int(k, val))
		case int64:
			record.AddAttributes(log.Int64(k, int64(val)))
		}
	}

	c.logger.Emit(ctx, record)
}

// Close shuts down the OTLP client
func (c *Client) Close(ctx context.Context) error {
	return c.provider.Shutdown(ctx)
}

// levelToSeverity converts our LogLevel to OTel severity
func levelToSeverity(level watch.LogLevel) log.Severity {
	switch level {
	case watch.LevelTrace:
		return log.SeverityTrace
	case watch.LevelDebug:
		return log.SeverityDebug
	case watch.LevelInfo:
		return log.SeverityInfo
	case watch.LevelWarn:
		return log.SeverityWarn
	case watch.LevelError:
		return log.SeverityError
	case watch.LevelFatal:
		return log.SeverityFatal
	default:
		return log.SeverityInfo
	}
}
