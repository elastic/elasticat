// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"
)

// Sample docker-compose.yml content for testing
const sampleDockerCompose = `configs:
  edot-collector-config:
    content: |
      receivers:
        otlp:
          protocols:
            grpc:
              endpoint: 0.0.0.0:4317
            http:
              endpoint: 0.0.0.0:4318
      processors:
        batch:
          timeout: 1s
      exporters:
        elasticsearch:
          endpoint: ${ES_LOCAL_ENDPOINT}
      service:
        pipelines:
          logs:
            receivers: [otlp]
            processors: [batch]
            exporters: [elasticsearch]

services:
  edot-collector:
    image: docker.elastic.co/beats/elastic-agent:8.16.0
    configs:
      - source: edot-collector-config
        target: /etc/otelcol-contrib/config.yaml
    ports:
      - "4317:4317"
      - "4318:4318"
`

func TestExtractInlineConfig(t *testing.T) {
	t.Parallel()

	t.Run("extracts valid config", func(t *testing.T) {
		t.Parallel()

		config, err := extractInlineConfig(sampleDockerCompose)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that the config contains expected content
		if !strings.Contains(config, "receivers:") {
			t.Error("expected config to contain 'receivers:'")
		}
		if !strings.Contains(config, "otlp:") {
			t.Error("expected config to contain 'otlp:'")
		}
		if !strings.Contains(config, "processors:") {
			t.Error("expected config to contain 'processors:'")
		}
		if !strings.Contains(config, "exporters:") {
			t.Error("expected config to contain 'exporters:'")
		}
		if !strings.Contains(config, "service:") {
			t.Error("expected config to contain 'service:'")
		}

		// Check that top-level indentation is removed properly
		// The first line should start with "receivers:" without leading spaces
		lines := strings.Split(config, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], " ") {
			t.Errorf("first line should not have leading spaces, got %q", lines[0])
		}
		if len(lines) > 0 && lines[0] != "receivers:" {
			t.Errorf("expected first line to be 'receivers:', got %q", lines[0])
		}

		// Should end with newline
		if !strings.HasSuffix(config, "\n") {
			t.Error("config should end with newline")
		}
	})

	t.Run("returns error for missing config section", func(t *testing.T) {
		t.Parallel()

		invalidCompose := `services:
  some-service:
    image: nginx
`
		_, err := extractInlineConfig(invalidCompose)
		if err == nil {
			t.Fatal("expected error for missing config section")
		}
		if !strings.Contains(err.Error(), "could not find edot-collector-config content") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("handles empty lines in config", func(t *testing.T) {
		t.Parallel()

		composeWithEmptyLines := `configs:
  edot-collector-config:
    content: |
      receivers:
        otlp:

      processors:
        batch:

services:
  test:
    image: test
`
		config, err := extractInlineConfig(composeWithEmptyLines)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should preserve empty lines within the config
		if !strings.Contains(config, "receivers:") {
			t.Error("expected config to contain 'receivers:'")
		}
		if !strings.Contains(config, "processors:") {
			t.Error("expected config to contain 'processors:'")
		}
	})

	t.Run("extracts multiline config correctly", func(t *testing.T) {
		t.Parallel()

		config, err := extractInlineConfig(sampleDockerCompose)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify structure is preserved
		lines := strings.Split(config, "\n")
		foundReceivers := false
		foundProcessors := false
		foundExporters := false
		foundService := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			switch trimmed {
			case "receivers:":
				foundReceivers = true
			case "processors:":
				foundProcessors = true
			case "exporters:":
				foundExporters = true
			case "service:":
				foundService = true
			}
		}

		if !foundReceivers || !foundProcessors || !foundExporters || !foundService {
			t.Error("expected all main sections to be present in extracted config")
		}
	})
}

func TestModifyComposeForFilemount(t *testing.T) {
	t.Parallel()

	t.Run("modifies compose correctly", func(t *testing.T) {
		t.Parallel()

		result, err := modifyComposeForFilemount(sampleDockerCompose)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should not contain the configs section at the top
		if strings.HasPrefix(result, "configs:") {
			t.Error("result should not start with 'configs:'")
		}

		// Should start with services
		if !strings.HasPrefix(result, "services:") {
			t.Error("result should start with 'services:'")
		}

		// Should contain the volume mount
		if !strings.Contains(result, "./config/otel-config.yaml:/etc/otelcol-contrib/config.yaml:ro") {
			t.Error("expected volume mount in result")
		}

		// Should contain the environment section
		if !strings.Contains(result, "environment:") {
			t.Error("expected environment section in result")
		}
		if !strings.Contains(result, "ES_LOCAL_PASSWORD=${ES_LOCAL_PASSWORD}") {
			t.Error("expected ES_LOCAL_PASSWORD environment variable in result")
		}

		// Should not contain the old configs block under edot-collector
		if strings.Contains(result, "source: edot-collector-config") {
			t.Error("result should not contain 'source: edot-collector-config'")
		}

		// Should preserve other parts like ports
		if !strings.Contains(result, "4317:4317") {
			t.Error("expected ports to be preserved")
		}
	})

	t.Run("returns error when configs section not found", func(t *testing.T) {
		t.Parallel()

		invalidCompose := `services:
  edot-collector:
    image: test
    configs:
      - source: edot-collector-config
        target: /etc/otelcol-contrib/config.yaml
`
		_, err := modifyComposeForFilemount(invalidCompose)
		if err == nil {
			t.Fatal("expected error when configs section not found")
		}
		if !strings.Contains(err.Error(), "failed to remove top-level configs section") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("returns error when edot-collector configs block not found", func(t *testing.T) {
		t.Parallel()

		// Has top-level configs but edot-collector doesn't have the right structure
		invalidCompose := `configs:
  edot-collector-config:
    content: |
      test: value

services:
  edot-collector:
    image: test
    # No configs block here
`
		_, err := modifyComposeForFilemount(invalidCompose)
		if err == nil {
			t.Fatal("expected error when edot-collector configs block not found")
		}
		if !strings.Contains(err.Error(), "failed to replace configs block under edot-collector") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("preserves service structure", func(t *testing.T) {
		t.Parallel()

		result, err := modifyComposeForFilemount(sampleDockerCompose)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// edot-collector should still be present
		if !strings.Contains(result, "edot-collector:") {
			t.Error("expected edot-collector service to be preserved")
		}

		// Image should be preserved
		if !strings.Contains(result, "docker.elastic.co/beats/elastic-agent:8.16.0") {
			t.Error("expected image to be preserved")
		}
	})
}

func TestOtelConfigValidationResult(t *testing.T) {
	t.Parallel()

	t.Run("valid result", func(t *testing.T) {
		t.Parallel()

		result := OtelConfigValidationResult{Valid: true, Message: ""}
		if !result.Valid {
			t.Error("expected Valid to be true")
		}
		if result.Message != "" {
			t.Errorf("expected empty message, got %q", result.Message)
		}
	})

	t.Run("invalid result with message", func(t *testing.T) {
		t.Parallel()

		result := OtelConfigValidationResult{Valid: false, Message: "configuration error"}
		if result.Valid {
			t.Error("expected Valid to be false")
		}
		if result.Message != "configuration error" {
			t.Errorf("expected message 'configuration error', got %q", result.Message)
		}
	})
}

func TestConstants(t *testing.T) {
	t.Parallel()

	t.Run("EdotCollectorContainerName", func(t *testing.T) {
		t.Parallel()

		if EdotCollectorContainerName != "edot-collector" {
			t.Errorf("expected 'edot-collector', got %q", EdotCollectorContainerName)
		}
	})

	t.Run("EdotCollectorBinary", func(t *testing.T) {
		t.Parallel()

		if EdotCollectorBinary != "/usr/share/elastic-agent/otelcol" {
			t.Errorf("expected '/usr/share/elastic-agent/otelcol', got %q", EdotCollectorBinary)
		}
	})

	t.Run("EdotCollectorConfigPath", func(t *testing.T) {
		t.Parallel()

		if EdotCollectorConfigPath != "/etc/otelcol-contrib/config.yaml" {
			t.Errorf("expected '/etc/otelcol-contrib/config.yaml', got %q", EdotCollectorConfigPath)
		}
	})
}

func TestExtractInlineConfigEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("handles config with special characters", func(t *testing.T) {
		t.Parallel()

		composeWithSpecialChars := `configs:
  edot-collector-config:
    content: |
      receivers:
        otlp:
          endpoint: "${HOST}:${PORT}"
      exporters:
        debug:
          verbosity: detailed # comment with special chars: @#$%

services:
  test:
    image: test
`
		config, err := extractInlineConfig(composeWithSpecialChars)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(config, `"${HOST}:${PORT}"`) {
			t.Error("expected special characters to be preserved")
		}
		if !strings.Contains(config, "# comment with special chars: @#$%") {
			t.Error("expected comments with special chars to be preserved")
		}
	})

	t.Run("handles minimal config", func(t *testing.T) {
		t.Parallel()

		minimalCompose := `configs:
  edot-collector-config:
    content: |
      service:

services:
  x:
    image: x
`
		config, err := extractInlineConfig(minimalCompose)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(config, "service:") {
			t.Error("expected minimal config to contain 'service:'")
		}
	})
}

func TestModifyComposePreservesYAMLStructure(t *testing.T) {
	t.Parallel()

	t.Run("preserves multi-service compose", func(t *testing.T) {
		t.Parallel()

		multiServiceCompose := `configs:
  edot-collector-config:
    content: |
      test: value

services:
  elasticsearch:
    image: elasticsearch:8.16.0
    ports:
      - "9200:9200"

  edot-collector:
    image: docker.elastic.co/beats/elastic-agent:8.16.0
    configs:
      - source: edot-collector-config
        target: /etc/otelcol-contrib/config.yaml
    depends_on:
      - elasticsearch

  kibana:
    image: kibana:8.16.0
    ports:
      - "5601:5601"
`
		result, err := modifyComposeForFilemount(multiServiceCompose)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// All services should be preserved
		if !strings.Contains(result, "elasticsearch:") {
			t.Error("expected elasticsearch service to be preserved")
		}
		if !strings.Contains(result, "edot-collector:") {
			t.Error("expected edot-collector service to be preserved")
		}
		if !strings.Contains(result, "kibana:") {
			t.Error("expected kibana service to be preserved")
		}

		// depends_on should be preserved
		if !strings.Contains(result, "depends_on:") {
			t.Error("expected depends_on to be preserved")
		}
	})
}
