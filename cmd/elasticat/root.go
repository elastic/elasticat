// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/spf13/cobra"

// Global flags shared across commands.
var (
	esURL       string
	esIndex     string
	serviceFlag string
	levelFlag   string
)

var rootCmd = &cobra.Command{
	Use:   "elasticat",
	Short: "AI-powered local development log viewer",
	Long: `ElastiCat - View and search your local development logs with the power of Elasticsearch.

Start the stack with 'elasticat up', then view logs with 'elasticat ui'.
Your AI assistant can query logs via the Elasticsearch MCP server.`,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&esURL, "es-url", "http://localhost:9200", "Elasticsearch URL")
	rootCmd.PersistentFlags().StringVar(&esIndex, "index", "logs-*", "Elasticsearch index/data stream pattern (e.g., 'logs-*', 'logs-myapp-*')")
}
