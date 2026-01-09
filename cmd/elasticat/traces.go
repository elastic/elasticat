// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/spf13/cobra"

var tracesCmd = &cobra.Command{
	Use:   "traces",
	Short: "Query and display traces (CLI)",
	Long: `Query traces from Elasticsearch and display in the terminal.

For the interactive TUI, use 'catseye traces'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSignalCommand(cmd, signalKindTraces, args)
	},
}

func init() {
	registerSignalFlags(tracesCmd)
	rootCmd.AddCommand(tracesCmd)
}
