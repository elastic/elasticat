// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"time"

	"github.com/elastic/elasticat/internal/config"
	"github.com/spf13/cobra"
)

// Global flags shared across commands.
// Values are bound via Viper; variables keep Cobra compatibility.
var (
	esURL           string
	esIndex         string
	pingTimeoutFlag time.Duration
	serviceFlag     string
	levelFlag       string
)

var rootCmd = &cobra.Command{
	Use:   "elasticat",
	Short: "AI-powered local development log viewer",
	Long: `ElastiCat - View and search your local development logs with the power of Elasticsearch.

Start the stack with 'elasticat up', then view logs with 'elasticat ui'.
Your AI assistant can query logs via the Elasticsearch MCP server.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cmd)
		if err != nil {
			return err
		}
		cmd.SetContext(config.WithContext(cmd.Context(), cfg))
		return nil
	},
}

func init() {
	// Global flags (Viper precedence: flags > env > defaults)
	rootCmd.PersistentFlags().StringVar(&esURL, "es-url", config.DefaultESURL, "Elasticsearch URL (env: ELASTICAT_ES_URL)")
	rootCmd.PersistentFlags().StringVarP(&esIndex, "index", "i", config.DefaultIndex, "Elasticsearch index/data stream pattern (env: ELASTICAT_INDEX)")
	pingTimeoutFlag = config.DefaultPingTimeout
	rootCmd.PersistentFlags().DurationVar(&pingTimeoutFlag, "ping-timeout", config.DefaultPingTimeout, "Elasticsearch ping timeout (env: ELASTICAT_PING_TIMEOUT)")
}
