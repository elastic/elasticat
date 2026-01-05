// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/spf13/cobra"

var tailCmd = &cobra.Command{
	Use:   "tail [service]",
	Short: "Tail logs in real-time (non-interactive)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Service override is taken from args before `--`.
		return runSignalCommandWithConfig(cmd, signalRunConfig{
			kind:          signalKindLogs,
			defaultFollow: true,
		}, args)
	},
}

func init() {
	tailCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Filter by service name")
	tailCmd.Flags().StringVarP(&levelFlag, "level", "l", "", "Filter by log level (ERROR, WARN, INFO, DEBUG)")
	// Reuse shared signal flags but with follow default true.
	registerSignalFlagsWithDefaults(tailCmd, true)
	rootCmd.AddCommand(tailCmd)
}
