// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/elasticat/internal/config"
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
		return runWatch(cmd, args)
	},
}

func init() {
	// Use config defaults (which respect env vars)
	watchCmd.Flags().IntVarP(&watchLines, "lines", "n", config.DefaultTailLines, "Number of lines to show from end of file (env: ELASTICAT_TAIL_LINES)")
	watchCmd.Flags().BoolVar(&watchNoColor, "no-color", false, "Disable colored output")
	watchCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Override service name (env: ELASTICAT_SERVICE)")
	watchCmd.Flags().StringVar(&watchOTLP, "otlp", config.DefaultOTLPEndpoint, "OTLP HTTP endpoint (env: ELASTICAT_OTLP_ENDPOINT)")
	watchCmd.Flags().BoolVar(&watchNoSend, "no-send", false, "Don't send logs to Elasticsearch, display only")
	watchCmd.Flags().BoolVar(&watchOneshot, "oneshot", false, "Import all logs and exit (don't follow)")

	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, files []string) error {
	cfg, ok := config.FromContext(cmd.Context())
	if !ok {
		return fmt.Errorf("configuration not loaded")
	}

	// Listen for SIGINT/SIGTERM and cancel the run context.
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create watcher
	watcher, err := watch.New(watch.Config{
		Context:   ctx,
		Files:     files,
		Service:   cfg.Watch.Service,
		TailLines: cfg.Watch.TailLines,
		Follow:    !cfg.Watch.Oneshot, // Don't follow in oneshot mode
		NoColor:   cfg.Watch.NoColor,
		Oneshot:   cfg.Watch.Oneshot,
	})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	go func() {
		<-ctx.Done()
		fmt.Println("\nShutting down...")
	}()

	// Create OTLP client if sending is enabled
	var otlpClient *otlp.Client
	if !cfg.Watch.NoSend {
		otlpClient, err = otlp.New(otlp.Config{
			Endpoint:    cfg.OTLP.Endpoint,
			ServiceName: cfg.Watch.Service,
			Insecure:    cfg.OTLP.Insecure,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create OTLP client: %v\n", err)
			fmt.Fprintf(os.Stderr, "Logs will be displayed but not sent to Elasticsearch.\n\n")
		}
	}

	// Determine if we need to show filename prefix (multiple files)
	showFilename := watcher.FileCount() > 1

	// Add handler for terminal output + OTLP sending
	watcher.AddHandler(func(log watch.ParsedLog) {
		// Print to terminal
		fmt.Println(watch.FormatLog(log, cfg.Watch.NoColor, showFilename))

		// Send to OTLP if client is available
		if otlpClient != nil {
			otlpClient.SendLog(ctx, log)
		}
	})

	// Print startup message
	if cfg.Watch.Oneshot {
		fmt.Printf("Importing all logs from %d file(s)", watcher.FileCount())
	} else {
		fmt.Printf("Watching %d file(s)", watcher.FileCount())
	}
	if !cfg.Watch.NoSend {
		fmt.Printf(" → sending to OTLP at %s", cfg.OTLP.Endpoint)
	}
	fmt.Println()
	if !cfg.Watch.Oneshot {
		fmt.Println("Press Ctrl+C to stop")
	}
	fmt.Println()

	// Oneshot mode: read all lines and exit
	if cfg.Watch.Oneshot {
		lineCount, err := watcher.ReadAll()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
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
