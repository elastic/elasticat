// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/elastic/elasticat/internal/config"
	telemetrygenlogs "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/logs"
	telemetrygenmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/metrics"
	telemetrygentraces "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/traces"
)

func TestStripURLScheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "https prefix",
			input:    "https://example.com:443",
			expected: "example.com:443",
		},
		{
			name:     "http prefix",
			input:    "http://localhost:4318",
			expected: "localhost:4318",
		},
		{
			name:     "no prefix",
			input:    "localhost:4318",
			expected: "localhost:4318",
		},
		{
			name:     "https with path",
			input:    "https://example.com:443/v1/traces",
			expected: "example.com:443/v1/traces",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "cloud endpoint",
			input:    "https://abc123.ingest.us-central1.gcp.elastic.cloud:443",
			expected: "abc123.ingest.us-central1.gcp.elastic.cloud:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripURLScheme(tt.input)
			if result != tt.expected {
				t.Errorf("stripURLScheme(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfigureTracesConfig(t *testing.T) {
	// Save and restore global state
	oldService := telemetrygenService
	defer func() { telemetrygenService = oldService }()
	telemetrygenService = "test-service"

	tests := []struct {
		name             string
		cfg              config.Config
		expectedEndpoint string
		expectedInsecure bool
		expectedHeaders  map[string]string
	}{
		{
			name: "https endpoint disables insecure",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "https://example.com:443",
					Insecure: true, // Should be overridden
				},
			},
			expectedEndpoint: "example.com:443",
			expectedInsecure: false,
		},
		{
			name: "http endpoint respects insecure flag",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "http://localhost:4318",
					Insecure: true,
				},
			},
			expectedEndpoint: "localhost:4318",
			expectedInsecure: true,
		},
		{
			name: "no scheme with insecure true",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "localhost:4318",
					Insecure: true,
				},
			},
			expectedEndpoint: "localhost:4318",
			expectedInsecure: true,
		},
		{
			name: "empty endpoint uses default",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "",
					Insecure: true,
				},
			},
			expectedEndpoint: "localhost:4318",
			expectedInsecure: true,
		},
		{
			name: "headers are copied",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "localhost:4318",
					Headers: map[string]string{
						"Authorization": "Bearer token123",
						"X-Custom":      "value",
					},
				},
			},
			expectedEndpoint: "localhost:4318",
			expectedHeaders: map[string]string{
				"Authorization": "Bearer token123",
				"X-Custom":      "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgCfg := telemetrygentraces.NewConfig()
			configureTracesConfig(tgCfg, tt.cfg)

			if tgCfg.CustomEndpoint != tt.expectedEndpoint {
				t.Errorf("CustomEndpoint = %q, want %q", tgCfg.CustomEndpoint, tt.expectedEndpoint)
			}
			if tgCfg.Insecure != tt.expectedInsecure {
				t.Errorf("Insecure = %v, want %v", tgCfg.Insecure, tt.expectedInsecure)
			}
			if !tgCfg.UseHTTP {
				t.Error("UseHTTP should be true")
			}
			if tgCfg.ServiceName != "test-service" {
				t.Errorf("ServiceName = %q, want %q", tgCfg.ServiceName, "test-service")
			}
			if tgCfg.NumChildSpans != 3 {
				t.Errorf("NumChildSpans = %d, want 3", tgCfg.NumChildSpans)
			}
			if tgCfg.Rate != 0 {
				t.Errorf("Rate = %f, want 0 (no rate limiting)", tgCfg.Rate)
			}
			if tt.expectedHeaders != nil {
				for k, v := range tt.expectedHeaders {
					if got, ok := tgCfg.Headers[k]; !ok || got != v {
						t.Errorf("Headers[%q] = %v, want %q", k, got, v)
					}
				}
			}
		})
	}
}

func TestConfigureLogsConfig(t *testing.T) {
	oldService := telemetrygenService
	defer func() { telemetrygenService = oldService }()
	telemetrygenService = "test-service"

	tests := []struct {
		name             string
		cfg              config.Config
		expectedEndpoint string
		expectedInsecure bool
	}{
		{
			name: "https endpoint",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "https://logs.example.com:443",
					Insecure: true,
				},
			},
			expectedEndpoint: "logs.example.com:443",
			expectedInsecure: false,
		},
		{
			name: "plain endpoint",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "localhost:4318",
					Insecure: true,
				},
			},
			expectedEndpoint: "localhost:4318",
			expectedInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgCfg := telemetrygenlogs.NewConfig()
			configureLogsConfig(tgCfg, tt.cfg)

			if tgCfg.CustomEndpoint != tt.expectedEndpoint {
				t.Errorf("CustomEndpoint = %q, want %q", tgCfg.CustomEndpoint, tt.expectedEndpoint)
			}
			if tgCfg.Insecure != tt.expectedInsecure {
				t.Errorf("Insecure = %v, want %v", tgCfg.Insecure, tt.expectedInsecure)
			}
			if !tgCfg.UseHTTP {
				t.Error("UseHTTP should be true")
			}
			if tgCfg.Rate != 0 {
				t.Errorf("Rate = %f, want 0 (no rate limiting)", tgCfg.Rate)
			}
		})
	}
}

func TestConfigureMetricsConfig(t *testing.T) {
	oldService := telemetrygenService
	defer func() { telemetrygenService = oldService }()
	telemetrygenService = "test-service"

	tests := []struct {
		name             string
		cfg              config.Config
		expectedEndpoint string
		expectedInsecure bool
	}{
		{
			name: "https endpoint",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "https://metrics.example.com:443",
					Insecure: true,
				},
			},
			expectedEndpoint: "metrics.example.com:443",
			expectedInsecure: false,
		},
		{
			name: "plain endpoint",
			cfg: config.Config{
				OTLP: config.OTLPConfig{
					Endpoint: "localhost:4318",
					Insecure: true,
				},
			},
			expectedEndpoint: "localhost:4318",
			expectedInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgCfg := telemetrygenmetrics.NewConfig()
			configureMetricsConfig(tgCfg, tt.cfg)

			if tgCfg.CustomEndpoint != tt.expectedEndpoint {
				t.Errorf("CustomEndpoint = %q, want %q", tgCfg.CustomEndpoint, tt.expectedEndpoint)
			}
			if tgCfg.Insecure != tt.expectedInsecure {
				t.Errorf("Insecure = %v, want %v", tgCfg.Insecure, tt.expectedInsecure)
			}
			if !tgCfg.UseHTTP {
				t.Error("UseHTTP should be true")
			}
			if tgCfg.Rate != 0 {
				t.Errorf("Rate = %f, want 0 (no rate limiting)", tgCfg.Rate)
			}
		})
	}
}
