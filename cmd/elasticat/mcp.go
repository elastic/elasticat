// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/elasticat/internal/config"
	"github.com/spf13/cobra"
)

// MCPConfig represents the MCP server configuration structure.
type MCPConfig struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

// MCPServer represents a single MCP server entry.
type MCPServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Generate MCP server configuration for Elastic Agent Builder",
	Long: `Generate an MCP (Model Context Protocol) server configuration JSON
for connecting to the Elastic Agent Builder.

The output can be used to configure MCP clients such as Claude Desktop,
Cursor, VS Code, and other compatible tools.

Example:
  # Copy config to clipboard (macOS)
  elasticat mcp | pbcopy

  # Save to file
  elasticat mcp > mcp-config.json

  # Use a specific profile
  elasticat --profile production mcp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, ok := config.FromContext(cmd.Context())
		if !ok {
			return fmt.Errorf("configuration not loaded")
		}

		// Get Kibana URL
		kibanaURL := cfg.Kibana.URL
		if kibanaURL == "" {
			return fmt.Errorf("Kibana URL not configured. Set it with:\n  elasticat config set-profile <name> --kibana-url <url>")
		}

		// Get API key - we need to load the profile to get the resolved API key
		apiKey := cfg.ES.APIKey
		if apiKey == "" {
			return fmt.Errorf("API key not configured. Set it with:\n  elasticat config set-profile <name> --es-api-key <key>")
		}

		// Build the MCP endpoint URL
		mcpEndpoint := buildMCPEndpoint(kibanaURL, cfg.Kibana.Space)

		// Build the authorization header
		authHeader := fmt.Sprintf("Authorization:ApiKey %s", apiKey)

		// Create the MCP configuration
		mcpConfig := MCPConfig{
			MCPServers: map[string]MCPServer{
				"elastic-agent-builder": {
					Command: "npx",
					Args: []string{
						"mcp-remote",
						mcpEndpoint,
						"--header",
						authHeader,
					},
				},
			},
		}

		// Output as pretty-printed JSON
		output, err := json.MarshalIndent(mcpConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}

		fmt.Println(string(output))
		return nil
	},
}

// buildMCPEndpoint constructs the MCP endpoint URL with optional space support.
func buildMCPEndpoint(kibanaURL, space string) string {
	// Remove trailing slash from Kibana URL
	kibanaURL = strings.TrimSuffix(kibanaURL, "/")

	if space != "" {
		return fmt.Sprintf("%s/s/%s/api/agent_builder/mcp", kibanaURL, space)
	}
	return fmt.Sprintf("%s/api/agent_builder/mcp", kibanaURL)
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
