// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/elastic/elasticat/internal/es"
	"github.com/spf13/cobra"
)

var (
	followFlag bool
)

var tailCmd = &cobra.Command{
	Use:   "tail [service]",
	Short: "Tail logs in real-time (non-interactive)",
	RunE: func(cmd *cobra.Command, args []string) error {
		service := ""
		if len(args) > 0 {
			service = args[0]
		}
		return runTail(service)
	},
}

func init() {
	tailCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Filter by service name")
	tailCmd.Flags().StringVarP(&levelFlag, "level", "l", "", "Filter by log level (ERROR, WARN, INFO, DEBUG)")
	tailCmd.Flags().BoolVarP(&followFlag, "follow", "f", true, "Follow logs in real-time")

	rootCmd.AddCommand(tailCmd)
}

func runTail(service string) error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx := context.Background()

	// Use service from flag if provided
	if serviceFlag != "" {
		service = serviceFlag
	}

	fmt.Printf("Tailing logs")
	if service != "" {
		fmt.Printf(" for service: %s", service)
	}
	if levelFlag != "" {
		fmt.Printf(" (level: %s)", levelFlag)
	}
	fmt.Println("... (Ctrl+C to stop)")
	fmt.Println()

	var lastTimestamp time.Time

	for {
		opts := es.TailOptions{
			Size:    50,
			Service: service,
			Level:   levelFlag,
			Since:   lastTimestamp,
		}

		result, err := client.Tail(ctx, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching logs: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Print logs in reverse order (oldest first)
		for i := len(result.Logs) - 1; i >= 0; i-- {
			log := result.Logs[i]
			if log.Timestamp.After(lastTimestamp) {
				printLogLine(log)
				lastTimestamp = log.Timestamp
			}
		}

		if !followFlag {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func printLogLine(log es.LogEntry) {
	ts := log.Timestamp.Format("15:04:05.000")
	level := log.GetLevel()
	service := log.ServiceName
	if service == "" && log.ContainerID != "" {
		service = log.ContainerID[:min(12, len(log.ContainerID))]
	}
	if service == "" {
		service = "unknown"
	}

	msg := log.GetMessage()

	// Color the level
	levelColor := "\033[0m"
	switch level {
	case "ERROR", "FATAL", "error", "fatal":
		levelColor = "\033[31m" // Red
	case "WARN", "WARNING", "warn", "warning":
		levelColor = "\033[33m" // Yellow
	case "INFO", "info":
		levelColor = "\033[32m" // Green
	case "DEBUG", "debug":
		levelColor = "\033[36m" // Cyan
	}

	fmt.Printf("%s %s%-5s\033[0m [%s] %s\n", ts, levelColor, level, service, msg)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
