// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Query and display logs (CLI)",
	Long: `Query logs from Elasticsearch and display in the terminal.

For the interactive TUI, use 'elasticat ui logs'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("CLI logs command not yet implemented.")
		fmt.Println("Use 'elasticat ui logs' for the interactive viewer.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
