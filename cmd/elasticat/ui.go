// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	osSignal "os/signal"
	"syscall"
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
		sig := tui.SignalLogs
		if len(args) > 0 {
			switch args[0] {
			case "logs":
				sig = tui.SignalLogs
			case "metrics":
				sig = tui.SignalMetrics
			case "traces":
				sig = tui.SignalTraces
			default:
				return fmt.Errorf("unknown signal %q (expected logs, metrics, traces)", args[0])
			}
		}
		return runTUI(cmd.Context(), sig)
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}

func runTUI(parentCtx context.Context, sig tui.SignalType) error {
	notifyCtx, stop := osSignal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Check connection
	ctx, cancel := context.WithTimeout(notifyCtx, 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		fmt.Println("Warning: Could not connect to Elasticsearch. Is the stack running?")
		fmt.Println("Run 'elasticat up' to start the stack.")
		fmt.Println()
	}

	model := tui.NewModel(notifyCtx, client, sig)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(notifyCtx))

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
