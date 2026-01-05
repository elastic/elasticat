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
	jsonFlag bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search logs with ES query syntax",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		return runSearch(query)
	},
}

func init() {
	searchCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Filter by service name")
	searchCmd.Flags().StringVarP(&levelFlag, "level", "l", "", "Filter by log level")
	searchCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")

	rootCmd.AddCommand(searchCmd)
}

func runSearch(query string) error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	for _, log := range result.Logs {
		printLogLine(log)
	}

	return nil
}
