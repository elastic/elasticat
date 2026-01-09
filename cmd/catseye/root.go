// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"github.com/elastic/elasticat/internal/config"
	"github.com/elastic/elasticat/internal/tui"
	"github.com/spf13/cobra"
)

// Global flags shared across commands.
// Values are bound via Viper; variables keep Cobra compatibility.
var (
	esURL           string
	esIndex         string
	pingTimeoutFlag time.Duration
	profileFlag     string // Profile name override
)

var rootCmd = &cobra.Command{
	Use:   "catseye [signal]",
	Short: "Interactive TUI for viewing logs, metrics, and traces",
	Long: `CatsEye - Interactive terminal UI for viewing logs, metrics, traces, and AI chat.

Signal can be: logs (default), metrics, traces, or chat.
Use 'chat' to start directly in AI chat mode powered by Elastic Agent Builder.

For CLI commands, use 'elasticat'.`,
	Args: cobra.MaximumNArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Set profile flag before loading config
		config.SetProfileFlag(profileFlag)

		cfg, err := config.Load(cmd)
		if err != nil {
			return err
		}
		cmd.SetContext(config.WithContext(cmd.Context(), cfg))
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		sig := tui.SignalLogs
		if len(args) > 0 {
			switch args[0] {
			case "logs":
				sig = tui.SignalLogs
			case "metrics":
				sig = tui.SignalMetrics
			case "traces":
				sig = tui.SignalTraces
			case "chat":
				sig = tui.SignalChat
			default:
				return fmt.Errorf("unknown signal %q (expected logs, metrics, traces, chat)", args[0])
			}
		}
		return runTUI(cmd.Context(), sig)
	},
}

func init() {
	// Global flags (Viper precedence: flags > env > profile > defaults)
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "Configuration profile to use (overrides current-profile in config)")
	rootCmd.PersistentFlags().StringVar(&esURL, "es-url", config.DefaultESURL, "Elasticsearch URL (env: ELASTICAT_ES_URL)")
	rootCmd.PersistentFlags().StringVarP(&esIndex, "index", "i", config.DefaultIndex, "Elasticsearch index/data stream pattern (env: ELASTICAT_ES_INDEX)")
	pingTimeoutFlag = config.DefaultPingTimeout
	rootCmd.PersistentFlags().DurationVar(&pingTimeoutFlag, "ping-timeout", config.DefaultPingTimeout, "Elasticsearch ping timeout (env: ELASTICAT_ES_PING_TIMEOUT)")
}
