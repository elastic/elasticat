// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"github.com/elastic/elasticat/internal/config"
	"github.com/elastic/elasticat/internal/es"
	"github.com/spf13/cobra"
)

var (
	jsonFlag bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search logs with ES query syntax",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		return runSearch(cmd.Context(), query)
	},
}

func init() {
	searchCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Filter by service name")
	searchCmd.Flags().StringVarP(&levelFlag, "level", "l", "", "Filter by log level")
	searchCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")

	rootCmd.AddCommand(searchCmd)
}

func runSearch(parentCtx context.Context, query string) error {
	cfg, ok := config.FromContext(parentCtx)
	if !ok {
		return fmt.Errorf("configuration not loaded")
	}

	client, err := es.NewFromConfig(cfg.ES.URL, cfg.ES.Index, cfg.ES.APIKey, cfg.ES.Username, cfg.ES.Password)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(parentCtx, cfg.ES.Timeout)
	defer cancel()

	result, err := client.Search(ctx, query, es.SearchOptions{
		Size:    100,
		Service: serviceFlag,
		Level:   levelFlag,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	fmt.Printf("Found %d logs matching '%s'\n\n", result.Total, query)

	if jsonFlag {
		printEntriesJSON(result.Logs)
		return nil
	}

	renderer := newTableRenderer(signalKindLogs, nil)
	renderer.RenderHeader()
	renderer.RenderRows(result.Logs)

	return nil
}
