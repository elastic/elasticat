// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/elasticat/internal/es"
	"github.com/spf13/cobra"
)

var (
	clearForce bool
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all collected telemetry (logs, metrics, traces) from Elasticsearch",
	Long: `Clears all logs, metrics, and traces from Elasticsearch. Useful during development
when you want a fresh start.

Examples:
  elasticat clear          # Prompts for confirmation
  elasticat clear --force  # Skip confirmation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClear()
	},
}

func init() {
	clearCmd.Flags().BoolVarP(&clearForce, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(clearCmd)
}

func runClear() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a client just for ping check (use logs index)
	pingClient, err := es.New([]string{esURL}, "logs-*")
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Check connection first
	if err := pingClient.Ping(ctx); err != nil {
		return fmt.Errorf("cannot connect to Elasticsearch: %w\nIs the stack running? Try 'elasticat up'", err)
	}

	// Prompt for confirmation unless --force
	if !clearForce {
		fmt.Print("This will delete ALL collected telemetry (logs, metrics, traces). Are you sure? [y/N] ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("Clearing all telemetry data...")

	// Define the index patterns for each signal type
	signals := []struct {
		name  string
		index string
	}{
		{"logs", "logs-*"},
		{"metrics", "metrics-*"},
		{"traces", "traces-*"},
	}

	var totalDeleted int64
	for _, sig := range signals {
		client, err := es.New([]string{esURL}, sig.index)
		if err != nil {
			fmt.Printf("  Warning: failed to create client for %s: %v\n", sig.name, err)
			continue
		}

		deleted, err := client.Clear(ctx)
		if err != nil {
			// Don't fail completely if one index pattern doesn't exist
			fmt.Printf("  %s: 0 (no data or index not found)\n", sig.name)
			continue
		}

		fmt.Printf("  %s: %d deleted\n", sig.name, deleted)
		totalDeleted += deleted
	}

	fmt.Printf("\nTotal: %d documents deleted.\n", totalDeleted)
	return nil
}
