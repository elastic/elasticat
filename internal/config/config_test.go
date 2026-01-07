// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	// Root-level flags
	cmd.PersistentFlags().String("es-url", "", "")
	cmd.PersistentFlags().String("index", "", "")
	cmd.PersistentFlags().Duration("ping-timeout", 0, "")

	// Watch flags (simulate watch command flags)
	cmd.Flags().Int("lines", 0, "")
	cmd.Flags().Bool("no-color", false, "")
	cmd.Flags().Bool("no-send", false, "")
	cmd.Flags().Bool("oneshot", false, "")
	cmd.Flags().String("service", "", "")
	cmd.Flags().String("otlp", "", "")

	// TUI timing flags
	cmd.Flags().Duration("tick-interval", 0, "")
	cmd.Flags().Duration("logs-timeout", 0, "")
	cmd.Flags().Duration("metrics-timeout", 0, "")
	cmd.Flags().Duration("traces-timeout", 0, "")
	cmd.Flags().Duration("field-caps-timeout", 0, "")
	cmd.Flags().Duration("auto-detect-timeout", 0, "")

	return cmd
}

func TestLoad_Defaults(t *testing.T) {
	keys := []string{
		"ELASTICAT_ES_URL",
		"ELASTICAT_ES_INDEX",
		"ELASTICAT_ES_PING_TIMEOUT",
		"ELASTICAT_OTLP_ENDPOINT",
		"ELASTICAT_WATCH_TAIL_LINES",
		"ELASTICAT_WATCH_SERVICE",
		"ELASTICAT_TUI_TICK_INTERVAL",
		"ELASTICAT_TUI_LOGS_TIMEOUT",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}
	cmd := newTestCmd()
	cfg, err := Load(cmd)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ES.URL != DefaultESURL {
		t.Errorf("ES.URL = %q, want %q", cfg.ES.URL, DefaultESURL)
	}
	if cfg.TUI.TickInterval != DefaultTickInterval {
		t.Errorf("TUI.TickInterval = %v, want %v", cfg.TUI.TickInterval, DefaultTickInterval)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("ELASTICAT_ES_URL", "http://custom:9200")
	t.Setenv("ELASTICAT_ES_INDEX", "custom-*")
	t.Setenv("ELASTICAT_ES_PING_TIMEOUT", "7s")
	t.Setenv("ELASTICAT_OTLP_ENDPOINT", "custom:4318")
	t.Setenv("ELASTICAT_WATCH_TAIL_LINES", "50")
	t.Setenv("ELASTICAT_WATCH_SERVICE", "my-service")
	t.Setenv("ELASTICAT_TUI_TICK_INTERVAL", "3s")
	t.Setenv("ELASTICAT_TUI_LOGS_TIMEOUT", "11s")
	t.Setenv("ELASTICAT_TUI_METRICS_TIMEOUT", "44s")
	t.Setenv("ELASTICAT_TUI_TRACES_TIMEOUT", "45s")
	t.Setenv("ELASTICAT_TUI_FIELD_CAPS_TIMEOUT", "12s")
	t.Setenv("ELASTICAT_TUI_AUTO_DETECT_TIMEOUT", "31s")

	cmd := newTestCmd()
	cfg, err := Load(cmd)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ES.URL != "http://custom:9200" {
		t.Errorf("ES.URL = %q, want %q", cfg.ES.URL, "http://custom:9200")
	}
	if cfg.ES.Index != "custom-*" {
		t.Errorf("ES.Index = %q, want %q", cfg.ES.Index, "custom-*")
	}
	if cfg.ES.PingTimeout != 7*time.Second {
		t.Errorf("ES.PingTimeout = %v, want 7s", cfg.ES.PingTimeout)
	}
	if cfg.OTLP.Endpoint != "custom:4318" {
		t.Errorf("OTLP.Endpoint = %q, want %q", cfg.OTLP.Endpoint, "custom:4318")
	}
	if cfg.Watch.TailLines != 50 {
		t.Errorf("Watch.TailLines = %d, want 50", cfg.Watch.TailLines)
	}
	if cfg.Watch.Service != "my-service" {
		t.Errorf("Watch.Service = %q, want %q", cfg.Watch.Service, "my-service")
	}
	if cfg.TUI.TickInterval != 3*time.Second {
		t.Errorf("TUI.TickInterval = %v, want 3s", cfg.TUI.TickInterval)
	}
	if cfg.TUI.LogsTimeout != 11*time.Second {
		t.Errorf("TUI.LogsTimeout = %v, want 11s", cfg.TUI.LogsTimeout)
	}
	if cfg.TUI.MetricsTimeout != 44*time.Second {
		t.Errorf("TUI.MetricsTimeout = %v, want 44s", cfg.TUI.MetricsTimeout)
	}
	if cfg.TUI.TracesTimeout != 45*time.Second {
		t.Errorf("TUI.TracesTimeout = %v, want 45s", cfg.TUI.TracesTimeout)
	}
	if cfg.TUI.FieldCapsTimeout != 12*time.Second {
		t.Errorf("TUI.FieldCapsTimeout = %v, want 12s", cfg.TUI.FieldCapsTimeout)
	}
	if cfg.TUI.AutoDetectTimeout != 31*time.Second {
		t.Errorf("TUI.AutoDetectTimeout = %v, want 31s", cfg.TUI.AutoDetectTimeout)
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("ELASTICAT_ES_URL", "http://env:9200")

	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("es-url", "http://flag:9200")

	cfg, err := Load(cmd)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ES.URL != "http://flag:9200" {
		t.Errorf("ES.URL = %q, want flag value", cfg.ES.URL)
	}
}

func TestLoad_InvalidEnv_FailsFast(t *testing.T) {
	t.Setenv("ELASTICAT_TUI_LOGS_TIMEOUT", "abc")

	cmd := newTestCmd()
	if _, err := Load(cmd); err == nil {
		t.Fatalf("expected error for invalid duration, got nil")
	}
}
