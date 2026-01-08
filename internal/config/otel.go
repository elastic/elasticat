// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// EdotCollectorContainerName is the name of the EDOT collector container.
const EdotCollectorContainerName = "edot-collector"

// ExtractOtelConfig extracts the inline OTel config from docker-compose.yml
// to a separate file and modifies docker-compose.yml to use a file mount.
// This is a one-time operation that enables easy config editing.
func ExtractOtelConfig() error {
	if !IsStartLocalInstalled() {
		return fmt.Errorf("start-local stack not installed. Run 'elasticat up' first")
	}

	// Check if docker-compose.yml is already using the file mount
	if IsComposeUsingFilemount() {
		return nil // Already configured
	}

	composePath, err := GetDockerComposePath()
	if err != nil {
		return fmt.Errorf("get docker-compose path: %w", err)
	}

	// Read docker-compose.yml
	data, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("read docker-compose.yml: %w", err)
	}

	content := string(data)

	// Get the config file path
	configPath, err := GetOtelConfigPath()
	if err != nil {
		return fmt.Errorf("get otel config path: %w", err)
	}

	// Check if the config file already exists (partial extraction state)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist - extract it from docker-compose.yml
		configContent, err := extractInlineConfig(content)
		if err != nil {
			return fmt.Errorf("extract inline config: %w", err)
		}

		// Ensure the config directory exists
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}

		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			return fmt.Errorf("write otel config: %w", err)
		}
	}

	// Modify docker-compose.yml to use a file mount instead of inline config
	newCompose, err := modifyComposeForFilemount(content)
	if err != nil {
		return fmt.Errorf("modify docker-compose: %w", err)
	}

	if err := os.WriteFile(composePath, []byte(newCompose), 0644); err != nil {
		return fmt.Errorf("write docker-compose.yml: %w", err)
	}

	return nil
}

// extractInlineConfig extracts the OTel config content from docker-compose.yml.
// The config is embedded under: configs.edot-collector-config.content
func extractInlineConfig(composeContent string) (string, error) {
	// Find the content: | block under edot-collector-config
	// The pattern matches from "content: |" to the next top-level section
	pattern := regexp.MustCompile(`(?s)edot-collector-config:\s*\n\s+content:\s*\|\n((?:\s{6}.*\n?)+)`)
	matches := pattern.FindStringSubmatch(composeContent)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find edot-collector-config content in docker-compose.yml")
	}

	// Remove the 6-space indentation from each line
	lines := strings.Split(matches[1], "\n")
	var configLines []string
	for _, line := range lines {
		// Remove exactly 6 spaces of indentation (the YAML block scalar indent)
		if len(line) >= 6 && strings.HasPrefix(line, "      ") {
			configLines = append(configLines, line[6:])
		} else if strings.TrimSpace(line) == "" {
			configLines = append(configLines, "")
		} else {
			// Stop when we hit a line that's not properly indented (end of block)
			break
		}
	}

	// Join and trim trailing empty lines
	config := strings.TrimRight(strings.Join(configLines, "\n"), "\n") + "\n"
	return config, nil
}

// IsComposeUsingFilemount checks if docker-compose.yml mounts the external config file.
func IsComposeUsingFilemount() bool {
	composePath, err := GetDockerComposePath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(composePath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "./config/otel-config.yaml:/etc/otelcol-contrib/config.yaml")
}

// modifyComposeForFilemount modifies the docker-compose.yml to use a file mount
// for the OTel config instead of the inline Docker config.
func modifyComposeForFilemount(composeContent string) (string, error) {
	// Strategy: Replace the configs section and update the edot-collector service
	// to use a volume mount instead of a Docker config.

	// 1. Remove the entire configs section at the top (everything from configs: to services:)
	// Note: Go regexp doesn't support lookahead, so we match up to and including \nservices:
	// then replace with just \nservices:
	configsPattern := regexp.MustCompile(`(?s)^configs:\s*\n.*?\nservices:`)
	newContent := configsPattern.ReplaceAllString(composeContent, "services:")
	if newContent == composeContent {
		return "", fmt.Errorf("failed to remove top-level configs section - pattern not found")
	}

	// 2. Update the edot-collector service to use a volume mount and add environment
	// Replace:
	//     configs:
	//       - source: edot-collector-config
	//         target: /etc/otelcol-contrib/config.yaml
	// With:
	//     volumes:
	//       - ./config/otel-config.yaml:/etc/otelcol-contrib/config.yaml:ro
	//     environment:
	//       - ES_LOCAL_PASSWORD=${ES_LOCAL_PASSWORD}
	beforeReplace := newContent
	configsBlockPattern := regexp.MustCompile(`(?s)(edot-collector:.*?)(    configs:\s*\n\s+- source: edot-collector-config\s*\n\s+target: /etc/otelcol-contrib/config\.yaml\s*\n)`)
	newContent = configsBlockPattern.ReplaceAllString(newContent, "${1}    volumes:\n      - ./config/otel-config.yaml:/etc/otelcol-contrib/config.yaml:ro\n    environment:\n      - ES_LOCAL_PASSWORD=${ES_LOCAL_PASSWORD}\n")
	if newContent == beforeReplace {
		return "", fmt.Errorf("failed to replace configs block under edot-collector - pattern not found")
	}

	return newContent, nil
}

// GetContainerRuntime returns the available container runtime ("docker" or "podman").
// Returns an error if neither is found.
func GetContainerRuntime() (string, error) {
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker", nil
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman", nil
	}
	return "", fmt.Errorf("neither docker nor podman found in PATH")
}

// OtelConfigValidationResult contains the result of validating an OTel config.
type OtelConfigValidationResult struct {
	Valid   bool   // Whether the config is valid
	Message string // Error message if invalid, empty if valid
}

// EdotCollectorBinary is the path to the EDOT collector binary inside the container.
const EdotCollectorBinary = "/usr/share/elastic-agent/otelcol"

// EdotCollectorConfigPath is the path where the config is mounted inside the container.
const EdotCollectorConfigPath = "/etc/otelcol-contrib/config.yaml"

// ValidateOtelConfig validates the OTel collector config file by running
// the collector's validate command inside the running container.
func ValidateOtelConfig() OtelConfigValidationResult {
	runtime, err := GetContainerRuntime()
	if err != nil {
		return OtelConfigValidationResult{Valid: false, Message: err.Error()}
	}

	composePath, err := GetDockerComposePath()
	if err != nil {
		return OtelConfigValidationResult{Valid: false, Message: fmt.Sprintf("get docker-compose path: %v", err)}
	}

	// Check if the collector is running first
	if !IsOtelCollectorRunning() {
		return OtelConfigValidationResult{
			Valid:   false,
			Message: "Collector not running. Run 'elasticat down && up' to restart with file mount.",
		}
	}

	// Use docker compose exec to run validate inside the running container
	// EDOT collector binary is at /usr/share/elastic-agent/otelcol
	cmd := exec.Command(runtime, "compose", "-f", composePath, "exec", "-T",
		EdotCollectorContainerName,
		EdotCollectorBinary, "validate", "--config="+EdotCollectorConfigPath)

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		// The validate command returns non-zero on invalid config
		// Extract useful error message from output
		if outputStr != "" {
			// Try to extract just the error lines (skip info/warning preamble)
			lines := strings.Split(outputStr, "\n")
			var errorLines []string
			for _, line := range lines {
				lineLower := strings.ToLower(line)
				if strings.Contains(lineLower, "error") ||
					strings.Contains(lineLower, "cannot") ||
					strings.Contains(lineLower, "invalid") ||
					strings.Contains(lineLower, "failed") ||
					strings.Contains(lineLower, "yaml:") {
					errorLines = append(errorLines, strings.TrimSpace(line))
				}
			}
			if len(errorLines) > 0 {
				return OtelConfigValidationResult{Valid: false, Message: strings.Join(errorLines, "\n")}
			}
			return OtelConfigValidationResult{Valid: false, Message: outputStr}
		}
		return OtelConfigValidationResult{Valid: false, Message: err.Error()}
	}

	return OtelConfigValidationResult{Valid: true, Message: ""}
}

// ReloadOtelCollector restarts the OTel collector container to apply config changes.
// We use a full restart instead of SIGHUP because SIGHUP can fail if the collector
// has pending work (e.g., bulk indexing), causing it to exit without restarting.
func ReloadOtelCollector() error {
	runtime, err := GetContainerRuntime()
	if err != nil {
		return err
	}

	composePath, err := GetDockerComposePath()
	if err != nil {
		return fmt.Errorf("get docker-compose path: %w", err)
	}

	// Use docker compose restart for a clean restart
	// This is more reliable than SIGHUP which can fail during pending work
	cmd := exec.Command(runtime, "compose", "-f", composePath, "restart", EdotCollectorContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart collector: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// IsOtelCollectorRunning returns true if the EDOT collector container is running.
func IsOtelCollectorRunning() bool {
	runtime, err := GetContainerRuntime()
	if err != nil {
		return false
	}

	composePath, err := GetDockerComposePath()
	if err != nil {
		return false
	}

	// Use docker compose ps to check if the service is running
	cmd := exec.Command(runtime, "compose", "-f", composePath, "ps", "--status=running", "--format", "{{.Service}}", EdotCollectorContainerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == EdotCollectorContainerName
}

// IsCollectorUsingFilemount checks if the running collector container
// has the config file mounted (not using inline Docker config).
func IsCollectorUsingFilemount() bool {
	runtime, err := GetContainerRuntime()
	if err != nil {
		return false
	}

	// The container name includes the project suffix
	containerName := EdotCollectorContainerName + "-elastic-start-local"

	// Use docker inspect to check mounts
	cmd := exec.Command(runtime, "inspect", containerName,
		"--format", "{{range .Mounts}}{{.Destination}}{{end}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), "/etc/otelcol-contrib/config.yaml")
}

// RecreateCollector recreates the collector container to pick up config changes.
// This is needed when docker-compose.yml is modified to use a file mount.
func RecreateCollector() error {
	runtime, err := GetContainerRuntime()
	if err != nil {
		return err
	}

	composePath, err := GetDockerComposePath()
	if err != nil {
		return fmt.Errorf("get docker-compose path: %w", err)
	}

	// Use docker compose up -d to recreate the container with new config
	cmd := exec.Command(runtime, "compose", "-f", composePath, "up", "-d", EdotCollectorContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("recreate collector: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}
