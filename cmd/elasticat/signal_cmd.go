// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/index"
	"github.com/elastic/elasticat/internal/tui"
	"github.com/spf13/cobra"
)

const (
	defaultLimit     = 50
	defaultRefreshMs = 1000
)

var (
	followFlag       bool
	refreshMs        int
	limitFlag        int
	signalJSONOutput bool
)

// Shared flag registration with a customizable default follow value.
func registerSignalFlagsWithDefaults(cmd *cobra.Command, followDefault bool) {
	cmd.Flags().BoolVarP(&followFlag, "follow", "f", followDefault, "Tail new documents (polling)")
	cmd.Flags().IntVar(&refreshMs, "refresh", defaultRefreshMs, "Follow refresh interval in milliseconds")
	cmd.Flags().IntVar(&limitFlag, "limit", defaultLimit, "Documents fetched per request")
	cmd.Flags().BoolVar(&signalJSONOutput, "json", false, "Output raw JSON documents (NDJSON)")
}

func registerSignalFlags(cmd *cobra.Command) {
	registerSignalFlagsWithDefaults(cmd, false)
}

type signalKind int

const (
	signalKindLogs signalKind = iota
	signalKindMetrics
	signalKindTraces
)

type signalRunConfig struct {
	kind            signalKind
	serviceOverride string
	fieldsOverride  []string
	defaultFollow   bool
}

func (k signalKind) signalType() tui.SignalType {
	switch k {
	case signalKindMetrics:
		return tui.SignalMetrics
	case signalKindTraces:
		return tui.SignalTraces
	default:
		return tui.SignalLogs
	}
}

func (k signalKind) defaultIndex() string {
	switch k {
	case signalKindMetrics:
		return index.Metrics
	case signalKindTraces:
		return index.Traces
	default:
		return index.Logs
	}
}

func (k signalKind) processorEvent() string {
	if k == signalKindTraces {
		return "transaction"
	}
	return ""
}

func runSignalCommand(cmd *cobra.Command, kind signalKind, args []string) error {
	cfg := signalRunConfig{
		kind:          kind,
		defaultFollow: false,
	}
	return runSignalCommandWithConfig(cmd, cfg, args)
}

func runSignalCommandWithConfig(cmd *cobra.Command, cfg signalRunConfig, args []string) error {
	preArgs, fields := fieldsForRun(cmd, args)
	cfg.fieldsOverride = fields
	// For tail, the first positional (pre-dash) arg is an optional service override.
	if len(preArgs) > 0 {
		cfg.serviceOverride = preArgs[0]
	}
	effective := effectiveIndex(cmd, cfg.kind.defaultIndex())
	useFollow := followFlag
	if !cmd.Flags().Changed("follow") {
		// If user did not set --follow, honor the command's default preference.
		useFollow = cfg.defaultFollow
	}
	if useFollow {
		return runSignalFollow(cfg, effective)
	}
	return runSignalOnce(cfg, effective)
}

func effectiveIndex(cmd *cobra.Command, defaultIndex string) string {
	// `--index` is a persistent flag on rootCmd; Cobra/pflag flag lookup can be subtle
	// across command flagsets, so check all relevant ones.
	if cmd.Flags().Changed("index") || cmd.InheritedFlags().Changed("index") {
		return esIndex
	}
	if rootCmd != nil && rootCmd.PersistentFlags().Changed("index") {
		return esIndex
	}
	return defaultIndex
}

func serviceForRun(serviceOverride string) string {
	if serviceFlag != "" {
		return serviceFlag
	}
	if serviceOverride != "" {
		return serviceOverride
	}
	return ""
}

func fieldsForRun(cmd *cobra.Command, args []string) (preDashArgs []string, fields []string) {
	at := cmd.ArgsLenAtDash()
	if at >= 0 {
		preDashArgs = args[:at]
		fields = args[at:]
		return
	}
	return args, nil
}

func runSignalOnce(cfg signalRunConfig, indexPattern string) error {
	client, err := es.New([]string{esURL}, indexPattern)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	service := serviceForRun(cfg.serviceOverride)
	opts := baseTailOptions(cfg.kind, service)
	entries, err := fetchTailEntries(ctx, client, opts)
	if err != nil {
		return err
	}

	renderer := newTableRenderer(cfg.kind, cfg.fieldsOverride)
	return renderEntries(entries, renderer, true)
}

func runSignalFollow(cfg signalRunConfig, indexPattern string) error {
	client, err := es.New([]string{esURL}, indexPattern)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	renderer := newTableRenderer(cfg.kind, cfg.fieldsOverride)

	notifyCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initial load (non-follow sort order for latest docs)
	service := serviceForRun(cfg.serviceOverride)
	opts := baseTailOptions(cfg.kind, service)
	ctx, cancel := context.WithTimeout(notifyCtx, 10*time.Second)
	initial, err := fetchTailEntries(ctx, client, opts)
	cancel()
	if err != nil {
		return err
	}

	if err := renderEntries(initial, renderer, true); err != nil {
		return err
	}

	lastTimestamp := maxTimestamp(initial, time.Time{})
	if lastTimestamp.IsZero() {
		lastTimestamp = time.Now()
	}

	refresh := refreshMs
	if refresh <= 0 {
		refresh = defaultRefreshMs
	}
	interval := time.Duration(refresh) * time.Millisecond
	if interval <= 0 {
		interval = time.Millisecond * defaultRefreshMs
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-notifyCtx.Done():
			return nil
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(notifyCtx, 10*time.Second)
			opts := baseTailOptions(cfg.kind, service)
			opts.SortAsc = true
			if !lastTimestamp.IsZero() {
				opts.Since = lastTimestamp
			}
			entries, err := fetchTailEntries(ctx, client, opts)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "refresh error: %v\n", err)
				continue
			}

			newEntries := filterNewEntries(entries, lastTimestamp)
			if len(newEntries) == 0 {
				continue
			}

			if err := renderEntries(newEntries, renderer, false); err != nil {
				return err
			}

			lastTimestamp = maxTimestamp(newEntries, lastTimestamp)
		}
	}
}

func baseTailOptions(kind signalKind, service string) es.TailOptions {
	limit := limitFlag
	if limit <= 0 {
		limit = defaultLimit
	}
	return es.TailOptions{
		Size:           limit,
		Service:        service,
		Level:          levelFlag,
		ProcessorEvent: kind.processorEvent(),
		SortAsc:        false,
	}
}

func fetchTailEntries(ctx context.Context, client *es.Client, opts es.TailOptions) ([]es.LogEntry, error) {
	result, err := client.Tail(ctx, opts)
	if err != nil {
		return nil, err
	}
	return result.Logs, nil
}

func renderEntries(entries []es.LogEntry, renderer *tableRenderer, showHeader bool) error {
	if signalJSONOutput {
		printEntriesJSON(entries)
		return nil
	}
	if showHeader {
		renderer.RenderHeader()
	}
	renderer.RenderRows(entries)
	return nil
}

func printEntriesJSON(entries []es.LogEntry) {
	for _, entry := range entries {
		if entry.RawJSON != "" {
			fmt.Println(entry.RawJSON)
			continue
		}
		data, err := json.Marshal(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal entry: %v\n", err)
			continue
		}
		fmt.Println(string(data))
	}
}

func filterNewEntries(entries []es.LogEntry, after time.Time) []es.LogEntry {
	var result []es.LogEntry
	for _, entry := range entries {
		if entry.Timestamp.After(after) {
			result = append(result, entry)
		}
	}
	return result
}

func maxTimestamp(entries []es.LogEntry, current time.Time) time.Time {
	for _, entry := range entries {
		if entry.Timestamp.After(current) {
			current = entry.Timestamp
		}
	}
	return current
}
