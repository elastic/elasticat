// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui [signal]",
	Short: "Open the interactive TUI viewer",
	Long: `Opens the interactive terminal UI for viewing logs, metrics, or traces.

Signal can be: logs (default), metrics, or traces.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signal := tui.SignalLogs
		if len(args) > 0 {
			switch args[0] {
			case "logs":
				signal = tui.SignalLogs
			case "metrics":
				signal = tui.SignalMetrics
			case "traces":
				signal = tui.SignalTraces
			default:
				return fmt.Errorf("unknown signal %q (expected logs, metrics, traces)", args[0])
			}
		}
		return runTUI(signal)
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}

func runTUI(signal tui.SignalType) error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		fmt.Println("Warning: Could not connect to Elasticsearch. Is the stack running?")
		fmt.Println("Run 'elasticat up' to start the stack.")
		fmt.Println()
	}

	model := tui.NewModel(client, signal)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
