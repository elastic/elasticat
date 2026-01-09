// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/spf13/cobra"

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Query and display metrics (CLI)",
	Long: `Query metrics from Elasticsearch and display in the terminal.

For the interactive TUI, use 'catseye metrics'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSignalCommand(cmd, signalKindMetrics, args)
	},
}

func init() {
	registerSignalFlags(metricsCmd)
	rootCmd.AddCommand(metricsCmd)
}
