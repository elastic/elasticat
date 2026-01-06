// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

// Package config provides centralized configuration management for elasticat.
// It supports deterministic precedence (flags > env > defaults) using Viper,
// and fail-fast validation to prevent silent misconfiguration.
package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	ES          ESConfig     `mapstructure:"es"`
	OTLP        OTLPConfig   `mapstructure:"otlp"`
	Kibana      KibanaConfig `mapstructure:"kibana"`
	Watch       WatchConfig  `mapstructure:"watch"`
	TUI         TUIConfig    `mapstructure:"tui"`
	ProfileName string       `mapstructure:"-"` // Active profile name (not persisted)
}

// ESConfig holds Elasticsearch connection settings.
type ESConfig struct {
	URL         string        `mapstructure:"url"`          // Elasticsearch URL
	Index       string        `mapstructure:"index"`        // Index pattern
	Timeout     time.Duration `mapstructure:"timeout"`      // Query timeout
	PingTimeout time.Duration `mapstructure:"ping_timeout"` // Ping timeout
	APIKey      string        `mapstructure:"api_key"`      // API key for authentication
	Username    string        `mapstructure:"username"`     // Username for basic auth
	Password    string        `mapstructure:"password"`     // Password for basic auth
}

// KibanaConfig holds Kibana connection settings.
type KibanaConfig struct {
	URL string `mapstructure:"url"` // Kibana URL
}

// OTLPConfig holds OpenTelemetry Protocol settings.
type OTLPConfig struct {
	Endpoint string `mapstructure:"endpoint"` // OTLP HTTP endpoint
	Insecure bool   `mapstructure:"insecure"` // Use insecure connection
}

// WatchConfig holds file watching settings.
type WatchConfig struct {
	TailLines int    `mapstructure:"tail_lines"` // Number of lines to show initially
	NoColor   bool   `mapstructure:"no_color"`   // Disable colored output
	NoSend    bool   `mapstructure:"no_send"`    // Don't send to OTLP
	Oneshot   bool   `mapstructure:"oneshot"`    // Import all and exit
	Service   string `mapstructure:"service"`    // Override service name
}

// TUIConfig holds TUI timing and request settings.
type TUIConfig struct {
	TickInterval      time.Duration `mapstructure:"tick_interval"`
	LogsTimeout       time.Duration `mapstructure:"logs_timeout"`
	MetricsTimeout    time.Duration `mapstructure:"metrics_timeout"`
	TracesTimeout     time.Duration `mapstructure:"traces_timeout"`
	FieldCapsTimeout  time.Duration `mapstructure:"field_caps_timeout"`
	AutoDetectTimeout time.Duration `mapstructure:"auto_detect_timeout"`
}

// Default configuration values.
const (
	DefaultESURL             = "http://localhost:9200"
	DefaultIndex             = "logs-*"
	DefaultTimeout           = 30 * time.Second
	DefaultPingTimeout       = 5 * time.Second
	DefaultOTLPEndpoint      = "localhost:4318"
	DefaultTailLines         = 10
	DefaultTickInterval      = 2 * time.Second
	DefaultLogsTimeout       = 10 * time.Second
	DefaultMetricsTimeout    = 30 * time.Second
	DefaultTracesTimeout     = 30 * time.Second
	DefaultFieldCapsTimeout  = 10 * time.Second
	DefaultAutoDetectTimeout = 30 * time.Second
)

// profileFlag holds the --profile flag value, set by root command.
var profileFlag string

// SetProfileFlag sets the profile flag value (called from root command init).
func SetProfileFlag(name string) {
	profileFlag = name
}

// GetProfileFlag returns the current profile flag value.
func GetProfileFlag() string {
	return profileFlag
}

// ContextKey is used to store config in context.
type ContextKey struct{}

// FromContext retrieves Config from context.
func FromContext(ctx context.Context) (Config, bool) {
	cfg, ok := ctx.Value(ContextKey{}).(Config)
	return cfg, ok
}

// WithContext stores Config in context.
func WithContext(ctx context.Context, cfg Config) context.Context {
	return context.WithValue(ctx, ContextKey{}, cfg)
}

// Load builds a Config using Viper with precedence: flags > env > profile > defaults.
// It binds flags from the command (and its parents) and fails fast on invalid values.
func Load(cmd *cobra.Command) (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("ELASTICAT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 1. Set defaults
	setDefaults(v)

	// 2. Load and apply profile (between defaults and env)
	profileName, err := applyProfile(v)
	if err != nil {
		return Config{}, fmt.Errorf("apply profile: %w", err)
	}

	// 3. Bind flags (will override env and profile)
	if err := bindFlagsRecursive(v, cmd); err != nil {
		return Config{}, fmt.Errorf("bind flags: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.ProfileName = profileName

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyProfile loads the profile configuration and applies the active profile
// values to Viper. Returns the name of the active profile (empty if none).
func applyProfile(v *viper.Viper) (string, error) {
	profileCfg, err := LoadProfiles()
	if err != nil {
		// Log warning but don't fail - profiles are optional
		fmt.Fprintf(os.Stderr, "Warning: could not load profiles: %v\n", err)
		return "", nil
	}

	profile, name := profileCfg.GetActiveProfile(profileFlag)
	if profile == nil {
		return "", nil
	}

	// Resolve environment variable references in credentials
	resolved, err := profile.Resolve()
	if err != nil {
		return "", fmt.Errorf("profile %q: %w", name, err)
	}

	// Apply profile values to Viper (these become defaults that env/flags can override)
	if resolved.Elasticsearch.URL != "" {
		v.Set("es.url", resolved.Elasticsearch.URL)
	}
	if resolved.Elasticsearch.APIKey != "" {
		v.Set("es.api_key", resolved.Elasticsearch.APIKey)
	}
	if resolved.Elasticsearch.Username != "" {
		v.Set("es.username", resolved.Elasticsearch.Username)
	}
	if resolved.Elasticsearch.Password != "" {
		v.Set("es.password", resolved.Elasticsearch.Password)
	}
	if resolved.OTLP.Endpoint != "" {
		v.Set("otlp.endpoint", resolved.OTLP.Endpoint)
	}
	if resolved.OTLP.Insecure != nil {
		v.Set("otlp.insecure", *resolved.OTLP.Insecure)
	}
	if resolved.Kibana.URL != "" {
		v.Set("kibana.url", resolved.Kibana.URL)
	}

	return name, nil
}

// setDefaults registers default values with Viper.
func setDefaults(v *viper.Viper) {
	v.SetDefault("es.url", DefaultESURL)
	v.SetDefault("es.index", DefaultIndex)
	v.SetDefault("es.timeout", DefaultTimeout)
	v.SetDefault("es.ping_timeout", DefaultPingTimeout)
	v.SetDefault("es.api_key", "")
	v.SetDefault("es.username", "")
	v.SetDefault("es.password", "")

	v.SetDefault("otlp.endpoint", DefaultOTLPEndpoint)
	v.SetDefault("otlp.insecure", true)

	v.SetDefault("kibana.url", DefaultKibanaURL)

	v.SetDefault("watch.tail_lines", DefaultTailLines)
	v.SetDefault("watch.no_color", false)
	v.SetDefault("watch.no_send", false)
	v.SetDefault("watch.oneshot", false)
	v.SetDefault("watch.service", "")

	v.SetDefault("tui.tick_interval", DefaultTickInterval)
	v.SetDefault("tui.logs_timeout", DefaultLogsTimeout)
	v.SetDefault("tui.metrics_timeout", DefaultMetricsTimeout)
	v.SetDefault("tui.traces_timeout", DefaultTracesTimeout)
	v.SetDefault("tui.field_caps_timeout", DefaultFieldCapsTimeout)
	v.SetDefault("tui.auto_detect_timeout", DefaultAutoDetectTimeout)
}

// bindFlagsRecursive binds flags from cmd and all parents so Viper sees them.
func bindFlagsRecursive(v *viper.Viper, cmd *cobra.Command) error {
	if cmd == nil {
		return nil
	}
	if err := bindFlagSet(v, cmd.Flags()); err != nil {
		return err
	}
	if err := bindFlagSet(v, cmd.PersistentFlags()); err != nil {
		return err
	}
	return bindFlagsRecursive(v, cmd.Parent())
}

// bindFlagSet binds flags to Viper keys using explicit mappings to nested keys.
func bindFlagSet(v *viper.Viper, fs *pflag.FlagSet) error {
	if fs == nil {
		return nil
	}
	flagToKey := map[string]string{
		"es-url":              "es.url",
		"index":               "es.index",
		"ping-timeout":        "es.ping_timeout",
		"otlp":                "otlp.endpoint",
		"service":             "watch.service",
		"lines":               "watch.tail_lines",
		"no-color":            "watch.no_color",
		"no-send":             "watch.no_send",
		"oneshot":             "watch.oneshot",
		"tick-interval":       "tui.tick_interval",
		"logs-timeout":        "tui.logs_timeout",
		"metrics-timeout":     "tui.metrics_timeout",
		"traces-timeout":      "tui.traces_timeout",
		"field-caps-timeout":  "tui.field_caps_timeout",
		"auto-detect-timeout": "tui.auto_detect_timeout",
	}

	fs.VisitAll(func(f *pflag.Flag) {
		key, ok := flagToKey[f.Name]
		if !ok {
			// Fallback: replace "-" with "." to allow nested binding if names align
			key = strings.ReplaceAll(f.Name, "-", ".")
		}
		_ = v.BindPFlag(key, f)
	})
	return nil
}

// Validate enforces correctness and fails fast on invalid configuration.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ES.URL) == "" {
		return fmt.Errorf("es.url is required")
	}
	if strings.TrimSpace(c.ES.Index) == "" {
		return fmt.Errorf("es.index is required")
	}
	if c.ES.Timeout <= 0 {
		return fmt.Errorf("es.timeout must be > 0")
	}
	if c.ES.PingTimeout <= 0 {
		return fmt.Errorf("es.ping_timeout must be > 0")
	}
	if c.Watch.TailLines < 0 {
		return fmt.Errorf("watch.tail_lines must be >= 0")
	}
	if c.TUI.TickInterval <= 0 {
		return fmt.Errorf("tui.tick_interval must be > 0")
	}
	if c.TUI.LogsTimeout <= 0 {
		return fmt.Errorf("tui.logs_timeout must be > 0")
	}
	if c.TUI.MetricsTimeout <= 0 {
		return fmt.Errorf("tui.metrics_timeout must be > 0")
	}
	if c.TUI.TracesTimeout <= 0 {
		return fmt.Errorf("tui.traces_timeout must be > 0")
	}
	if c.TUI.FieldCapsTimeout <= 0 {
		return fmt.Errorf("tui.field_caps_timeout must be > 0")
	}
	if c.TUI.AutoDetectTimeout <= 0 {
		return fmt.Errorf("tui.auto_detect_timeout must be > 0")
	}
	return nil
}
