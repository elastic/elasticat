// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/elasticat/internal/otlp"
	"github.com/elastic/elasticat/internal/watch"
	"github.com/spf13/cobra"
)

var (
	watchLines   int
	watchNoColor bool
	watchOTLP    string
	watchNoSend  bool
	watchOneshot bool
)

var watchCmd = &cobra.Command{
	Use:   "watch <file>...",
	Short: "Watch log files like tail -F and send to Elasticsearch",
	Long: `Watch one or more log files in real-time, displaying them with colors
and automatically sending them to Elasticsearch via OTLP.

Examples:
  elasticat watch server.log
  elasticat watch server.log server-err.log
  elasticat watch ./logs/*.log
  elasticat watch -n 50 server.log     # Show last 50 lines
  elasticat watch --no-send server.log # Display only, don't send to ES`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWatch(args)
	},
}

func init() {
	watchCmd.Flags().IntVarP(&watchLines, "lines", "n", 10, "Number of lines to show from end of file")
	watchCmd.Flags().BoolVar(&watchNoColor, "no-color", false, "Disable colored output")
	watchCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Override service name (otherwise derived from filename)")
	watchCmd.Flags().StringVar(&watchOTLP, "otlp", "localhost:4318", "OTLP HTTP endpoint")
	watchCmd.Flags().BoolVar(&watchNoSend, "no-send", false, "Don't send logs to Elasticsearch, display only")
	watchCmd.Flags().BoolVar(&watchOneshot, "oneshot", false, "Import all logs and exit (don't follow)")

	rootCmd.AddCommand(watchCmd)
}

func runWatch(files []string) error {
	// Create watcher
	watcher, err := watch.New(watch.Config{
		Files:     files,
		Service:   serviceFlag,
		TailLines: watchLines,
		Follow:    !watchOneshot, // Don't follow in oneshot mode
		NoColor:   watchNoColor,
		Oneshot:   watchOneshot,
	})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Create OTLP client if sending is enabled
	var otlpClient *otlp.Client
	if !watchNoSend {
		otlpClient, err = otlp.New(otlp.Config{
			Endpoint:    watchOTLP,
			ServiceName: serviceFlag,
			Insecure:    true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create OTLP client: %v\n", err)
			fmt.Fprintf(os.Stderr, "Logs will be displayed but not sent to Elasticsearch.\n\n")
		}
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		watcher.Stop()
		cancel()
	}()

	// Determine if we need to show filename prefix (multiple files)
	showFilename := watcher.FileCount() > 1

	// Add handler for terminal output + OTLP sending
	watcher.AddHandler(func(log watch.ParsedLog) {
		// Print to terminal
		fmt.Println(watch.FormatLog(log, watchNoColor, showFilename))

		// Send to OTLP if client is available
		if otlpClient != nil {
			otlpClient.SendLog(ctx, log)
		}
	})

	// Print startup message
	if watchOneshot {
		fmt.Printf("Importing all logs from %d file(s)", watcher.FileCount())
	} else {
		fmt.Printf("Watching %d file(s)", watcher.FileCount())
	}
	if !watchNoSend {
		fmt.Printf(" → sending to OTLP at %s", watchOTLP)
	}
	fmt.Println()
	if !watchOneshot {
		fmt.Println("Press Ctrl+C to stop")
	}
	fmt.Println()

	// Oneshot mode: read all lines and exit
	if watchOneshot {
		lineCount, err := watcher.ReadAll()
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
		fmt.Printf("\nImported %d log lines.\n", lineCount)
	} else {
		// Follow mode: start watching
		if err := watcher.Start(); err != nil {
			return fmt.Errorf("watcher error: %w", err)
		}
	}

	// Cleanup OTLP client
	if otlpClient != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := otlpClient.Close(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to close OTLP client: %v\n", err)
		}
	}

	return nil
}
