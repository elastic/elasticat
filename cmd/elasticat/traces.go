// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tracesCmd = &cobra.Command{
	Use:   "traces",
	Short: "Query and display traces (CLI)",
	Long: `Query traces from Elasticsearch and display in the terminal.

For the interactive TUI, use 'elasticat ui traces'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("CLI traces command not yet implemented.")
		fmt.Println("Use 'elasticat ui traces' for the interactive viewer.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tracesCmd)
}
