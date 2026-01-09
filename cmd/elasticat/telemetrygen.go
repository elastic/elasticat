// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/elastic/elasticat/internal/config"
	telemetrygenlogs "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/logs"
	telemetrygenmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/metrics"
	telemetrygentraces "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/traces"
	"github.com/spf13/cobra"
)

var (
	telemetrygenLogs    bool
	telemetrygenTraces  bool
	telemetrygenMetrics bool
	telemetrygenCount   int
	telemetrygenService string
)

var telemetrygenCmd = &cobra.Command{
	Use:   "telemetrygen",
	Short: "Send test telemetry to the OTLP endpoint",
	Long: `Generate and send test logs, metrics, and traces to the OTLP endpoint
configured in the current profile.

This is useful for testing connectivity and verifying your observability
pipeline is working correctly.

Examples:
  # Send all three signal types (default)
  elasticat telemetrygen

  # Send only logs
  elasticat telemetrygen --logs

  # Send traces and metrics with custom count
  elasticat telemetrygen --traces --metrics --count 10

  # Use a custom service name
  elasticat telemetrygen --service my-app`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTelemetrygen(cmd)
	},
}

func init() {
	telemetrygenCmd.Flags().BoolVar(&telemetrygenLogs, "logs", false, "Send test logs")
	telemetrygenCmd.Flags().BoolVar(&telemetrygenTraces, "traces", false, "Send test traces")
	telemetrygenCmd.Flags().BoolVar(&telemetrygenMetrics, "metrics", false, "Send test metrics")
	telemetrygenCmd.Flags().IntVar(&telemetrygenCount, "count", 5, "Number of items per signal type")
	telemetrygenCmd.Flags().StringVar(&telemetrygenService, "service", "elasticat-test", "Service name for test telemetry")

	rootCmd.AddCommand(telemetrygenCmd)
}

func runTelemetrygen(cmd *cobra.Command) error {
	cfg, ok := config.FromContext(cmd.Context())
	if !ok {
		return fmt.Errorf("configuration not loaded")
	}

	// If no specific flags set, send all three signal types
	sendAll := !telemetrygenLogs && !telemetrygenTraces && !telemetrygenMetrics
	if sendAll {
		telemetrygenLogs = true
		telemetrygenTraces = true
		telemetrygenMetrics = true
	}

	endpoint := cfg.OTLP.Endpoint
	if endpoint == "" {
		endpoint = "localhost:4318"
	}

	fmt.Printf("Sending test telemetry to %s...\n", stripURLScheme(endpoint))

	var errors []error

	if telemetrygenTraces {
		if err := sendTraces(cfg); err != nil {
			errors = append(errors, fmt.Errorf("traces: %w", err))
			fmt.Printf("  ✗ traces failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ %d traces sent\n", telemetrygenCount)
		}
	}

	if telemetrygenLogs {
		if err := sendLogs(cfg); err != nil {
			errors = append(errors, fmt.Errorf("logs: %w", err))
			fmt.Printf("  ✗ logs failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ %d logs sent\n", telemetrygenCount)
		}
	}

	if telemetrygenMetrics {
		if err := sendMetrics(cfg); err != nil {
			errors = append(errors, fmt.Errorf("metrics: %w", err))
			fmt.Printf("  ✗ metrics failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ %d metrics sent\n", telemetrygenCount)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some signals failed to send")
	}

	fmt.Println("Done!")
	return nil
}

func sendTraces(cfg config.Config) error {
	traceCfg := telemetrygentraces.NewConfig()
	configureTracesConfig(traceCfg, cfg)
	traceCfg.NumTraces = telemetrygenCount
	return telemetrygentraces.Start(traceCfg)
}

func sendLogs(cfg config.Config) error {
	logCfg := telemetrygenlogs.NewConfig()
	configureLogsConfig(logCfg, cfg)
	logCfg.NumLogs = telemetrygenCount
	return telemetrygenlogs.Start(logCfg)
}

func sendMetrics(cfg config.Config) error {
	metricCfg := telemetrygenmetrics.NewConfig()
	configureMetricsConfig(metricCfg, cfg)
	metricCfg.NumMetrics = telemetrygenCount
	return telemetrygenmetrics.Start(metricCfg)
}

// stripURLScheme removes http:// or https:// prefix from an endpoint
// telemetrygen expects just host:port, not a full URL
func stripURLScheme(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return endpoint
}

// configureTracesConfig sets the common OTLP configuration for traces
func configureTracesConfig(tgCfg *telemetrygentraces.Config, cfg config.Config) {
	endpoint := cfg.OTLP.Endpoint
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	useTLS := strings.HasPrefix(endpoint, "https://")
	tgCfg.CustomEndpoint = stripURLScheme(endpoint)
	tgCfg.UseHTTP = true
	tgCfg.Insecure = !useTLS && cfg.OTLP.Insecure
	tgCfg.InsecureSkipVerify = true
	tgCfg.ServiceName = telemetrygenService
	tgCfg.SkipSettingGRPCLogger = true
	tgCfg.NumChildSpans = 3 // Create traces with multiple spans
	tgCfg.Rate = 0          // No rate limiting

	// Copy headers from profile
	for k, v := range cfg.OTLP.Headers {
		tgCfg.Headers[k] = v
	}
}

// configureLogsConfig sets the common OTLP configuration for logs
func configureLogsConfig(tgCfg *telemetrygenlogs.Config, cfg config.Config) {
	endpoint := cfg.OTLP.Endpoint
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	useTLS := strings.HasPrefix(endpoint, "https://")
	tgCfg.CustomEndpoint = stripURLScheme(endpoint)
	tgCfg.UseHTTP = true
	tgCfg.Insecure = !useTLS && cfg.OTLP.Insecure
	tgCfg.InsecureSkipVerify = true
	tgCfg.ServiceName = telemetrygenService
	tgCfg.SkipSettingGRPCLogger = true
	tgCfg.Rate = 0 // No rate limiting

	// Copy headers from profile
	for k, v := range cfg.OTLP.Headers {
		tgCfg.Headers[k] = v
	}
}

// configureMetricsConfig sets the common OTLP configuration for metrics
func configureMetricsConfig(tgCfg *telemetrygenmetrics.Config, cfg config.Config) {
	endpoint := cfg.OTLP.Endpoint
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	useTLS := strings.HasPrefix(endpoint, "https://")
	tgCfg.CustomEndpoint = stripURLScheme(endpoint)
	tgCfg.UseHTTP = true
	tgCfg.Insecure = !useTLS && cfg.OTLP.Insecure
	tgCfg.InsecureSkipVerify = true
	tgCfg.ServiceName = telemetrygenService
	tgCfg.SkipSettingGRPCLogger = true
	tgCfg.Rate = 0 // No rate limiting

	// Copy headers from profile
	for k, v := range cfg.OTLP.Headers {
		tgCfg.Headers[k] = v
	}
}
