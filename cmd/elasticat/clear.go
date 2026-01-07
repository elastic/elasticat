// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/elastic/elasticat/internal/config"
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
		return runClear(cmd.Context())
	},
}

func init() {
	clearCmd.Flags().BoolVarP(&clearForce, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(clearCmd)
}

func runClear(parentCtx context.Context) error {
	cfg, ok := config.FromContext(parentCtx)
	if !ok {
		return fmt.Errorf("configuration not loaded")
	}

	if !isLocalESURL(cfg.ES.URL) {
		return fmt.Errorf("refusing to run `elasticat clear` against non-local Elasticsearch (%q); `clear` is only allowed when --es-url points to localhost", cfg.ES.URL)
	}

	ctx, cancel := context.WithTimeout(parentCtx, 60*time.Second)
	defer cancel()

	// Create a client just for ping check (use logs index)
	pingClient, err := es.NewFromConfig(cfg.ES.URL, cfg.ES.Index, cfg.ES.APIKey, cfg.ES.Username, cfg.ES.Password)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Check connection first
	if err := pingClient.Ping(ctx); err != nil {
		return fmt.Errorf("cannot connect to Elasticsearch: %w\nIs the stack running? Try 'elasticat up'", err)
	}

	// Prompt for confirmation unless --force
	if !clearForce {
		ok, err := confirmY(os.Stdin, os.Stdout, "This will delete ALL collected telemetry (logs, metrics, traces).\nType 'y' to continue: ")
		if err != nil {
			return err
		}
		if !ok {
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
		{"logs", cfg.ES.Index},
		{"metrics", "metrics-*"},
		{"traces", "traces-*"},
	}

	var totalDeleted int64
	for _, sig := range signals {
		client, err := es.NewFromConfig(cfg.ES.URL, sig.index, cfg.ES.APIKey, cfg.ES.Username, cfg.ES.Password)
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

func confirmY(in io.Reader, out io.Writer, prompt string) (bool, error) {
	// Intentionally strict: only a single 'y'/'Y' confirms.
	// Anything else (including empty input, EOF, or "yes") aborts.
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return false, err
	}

	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	line = strings.TrimSpace(line)
	return line == "y" || line == "Y", nil
}

func isLocalESURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}

	// Common flag/env form: "localhost:9200" or "127.0.0.1:9200" (no scheme).
	// Avoid url.Parse quirks where "host:port" is treated as "scheme:opaque".
	if !strings.Contains(raw, "://") {
		hostport := strings.SplitN(raw, "/", 2)[0]
		// Drop userinfo if present.
		if _, after, ok := strings.Cut(hostport, "@"); ok {
			hostport = after
		}
		host := hostport
		if h, _, err := net.SplitHostPort(hostport); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if host == "localhost" {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	}

	u, err := url.Parse(raw)
	if err != nil {
		return false
	}

	host := u.Hostname()
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
