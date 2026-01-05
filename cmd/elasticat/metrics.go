// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Query and display metrics (CLI)",
	Long: `Query metrics from Elasticsearch and display in the terminal.

For the interactive TUI, use 'elasticat ui metrics'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("CLI metrics command not yet implemented.")
		fmt.Println("Use 'elasticat ui metrics' for the interactive viewer.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}
