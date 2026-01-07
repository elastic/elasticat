// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	osSignal "os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/config"
	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui [signal]",
	Short: "Open the interactive TUI viewer",
	Long: `Opens the interactive terminal UI for viewing logs, metrics, traces, or chat.

Signal can be: logs (default), metrics, traces, or chat.
Use 'chat' to start directly in AI chat mode powered by Elastic Agent Builder.`,
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
	rootCmd.AddCommand(uiCmd)
}

func runTUI(parentCtx context.Context, sig tui.SignalType) error {
	cfg, ok := config.FromContext(parentCtx)
	if !ok {
		return fmt.Errorf("configuration not loaded")
	}

	notifyCtx, stop := osSignal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := es.NewFromConfig(cfg.ES.URL, cfg.ES.Index, cfg.ES.APIKey, cfg.ES.Username, cfg.ES.Password)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Check connection
	ctx, cancel := context.WithTimeout(notifyCtx, cfg.ES.PingTimeout)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		fmt.Println("Warning: Could not connect to Elasticsearch. Is the stack running?")
		fmt.Println("Run 'elasticat up' to start the stack.")
		fmt.Println()
	}

	model := tui.NewModelWithOpts(notifyCtx, client, sig, cfg.TUI, cfg.Kibana.URL, cfg.Kibana.Space, tui.NewModelOpts{
		ESAPIKey:   cfg.ES.APIKey,
		ESUsername: cfg.ES.Username,
		ESPassword: cfg.ES.Password,
	})
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(notifyCtx))

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
