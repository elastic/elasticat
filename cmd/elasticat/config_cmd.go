// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/elastic/elasticat/internal/config"
	"github.com/spf13/cobra"
)

// Flags for set-profile command
var (
	setProfileESURL       string
	setProfileESAPIKey    string
	setProfileESUsername  string
	setProfileESPassword  string
	setProfileOTLP        string
	setProfileOTLPInsec   bool
	setProfileKibanaURL   string
	setProfileKibanaSpace string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage elasticat configuration and profiles",
	Long: `Manage elasticat configuration profiles.

Profiles allow you to define multiple Elasticsearch/Kibana/OTLP configurations
and switch between them easily (similar to kubectl contexts).

Configuration is stored in ~/.config/elasticat/config.yaml`,
}

var useProfileCmd = &cobra.Command{
	Use:   "use-profile <name>",
	Short: "Set the current profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadProfiles()
		if err != nil {
			return fmt.Errorf("load profiles: %w", err)
		}

		// Verify profile exists
		if _, err := cfg.GetProfile(name); err != nil {
			return fmt.Errorf("profile %q does not exist", name)
		}

		cfg.CurrentProfile = name
		if err := config.SaveProfiles(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Switched to profile %q\n", name)
		return nil
	},
}

var setProfileCmd = &cobra.Command{
	Use:   "set-profile <name>",
	Short: "Create or update a profile",
	Long: `Create or update a named profile with connection settings.

Examples:
  # Create a local development profile
  elasticat config set-profile local --es-url http://localhost:9200

  # Create a staging profile with API key (using env var reference)
  elasticat config set-profile staging \
    --es-url https://staging.es.example.com:9243 \
    --es-api-key '${STAGING_ES_API_KEY}' \
    --kibana-url https://staging.kb.example.com

  # Create a profile with basic auth
  elasticat config set-profile dev \
    --es-url https://dev.es.example.com:9243 \
    --es-username elastic \
    --es-password changeme

Credentials can be stored as:
  - Environment variable references: ${MY_SECRET} (recommended)
  - Plain text values (warning will be shown)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadProfiles()
		if err != nil {
			return fmt.Errorf("load profiles: %w", err)
		}

		// Get existing profile or create new one
		profile, _ := cfg.GetProfile(name)

		// Update with provided flags
		if setProfileESURL != "" {
			profile.Elasticsearch.URL = setProfileESURL
		}
		if setProfileESAPIKey != "" {
			profile.Elasticsearch.APIKey = setProfileESAPIKey
		}
		if setProfileESUsername != "" {
			profile.Elasticsearch.Username = setProfileESUsername
		}
		if setProfileESPassword != "" {
			profile.Elasticsearch.Password = setProfileESPassword
		}
		if setProfileOTLP != "" {
			profile.OTLP.Endpoint = setProfileOTLP
		}
		if cmd.Flags().Changed("otlp-insecure") {
			profile.OTLP.Insecure = &setProfileOTLPInsec
		}
		if setProfileKibanaURL != "" {
			profile.Kibana.URL = setProfileKibanaURL
		}
		if setProfileKibanaSpace != "" {
			profile.Kibana.Space = setProfileKibanaSpace
		}

		cfg.SetProfile(name, profile)

		if err := config.SaveProfiles(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		// Warn if plain text credentials were stored
		if profile.HasPlainTextCredentials() {
			fmt.Fprintln(cmd.ErrOrStderr(), config.PlainTextCredentialWarning())
			fmt.Fprintln(cmd.ErrOrStderr())
		}

		fmt.Printf("Profile %q saved\n", name)
		return nil
	},
}

var getProfilesCmd = &cobra.Command{
	Use:     "get-profiles",
	Aliases: []string{"list-profiles", "profiles"},
	Short:   "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadProfiles()
		if err != nil {
			return fmt.Errorf("load profiles: %w", err)
		}

		names := cfg.ListProfiles()
		if len(names) == 0 {
			fmt.Println("No profiles configured.")
			fmt.Println("Create one with: elasticat config set-profile <name> --es-url <url>")
			return nil
		}

		sort.Strings(names)

		fmt.Println("PROFILES:")
		for _, name := range names {
			marker := "  "
			if name == cfg.CurrentProfile {
				marker = "* "
			}
			profile, _ := cfg.GetProfile(name)
			fmt.Printf("%s%-20s  %s\n", marker, name, profile.Elasticsearch.URL)
		}

		if cfg.CurrentProfile != "" {
			fmt.Printf("\n* = current profile\n")
		}

		return nil
	},
}

var currentProfileCmd = &cobra.Command{
	Use:   "current-profile",
	Short: "Show the current profile name",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadProfiles()
		if err != nil {
			return fmt.Errorf("load profiles: %w", err)
		}

		if cfg.CurrentProfile == "" {
			fmt.Println("No profile selected (using defaults)")
			return nil
		}

		fmt.Println(cfg.CurrentProfile)
		return nil
	},
}

var deleteProfileCmd = &cobra.Command{
	Use:   "delete-profile <name>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.LoadProfiles()
		if err != nil {
			return fmt.Errorf("load profiles: %w", err)
		}

		if err := cfg.DeleteProfile(name); err != nil {
			return err
		}

		if err := config.SaveProfiles(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Profile %q deleted\n", name)
		return nil
	},
}

var viewConfigCmd = &cobra.Command{
	Use:   "view",
	Short: "Show the full configuration (credentials masked)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadProfiles()
		if err != nil {
			return fmt.Errorf("load profiles: %w", err)
		}

		if len(cfg.Profiles) == 0 && cfg.CurrentProfile == "" {
			fmt.Println("No configuration found.")
			fmt.Println("Create a profile with: elasticat config set-profile <name> --es-url <url>")
			return nil
		}

		// Print the masked config
		fmt.Println(cfg.String())
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the configuration file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.GetConfigPath()
		if err != nil {
			return fmt.Errorf("get config path: %w", err)
		}
		fmt.Println(path)
		return nil
	},
}

func init() {
	// set-profile flags
	setProfileCmd.Flags().StringVar(&setProfileESURL, "es-url", "", "Elasticsearch URL")
	setProfileCmd.Flags().StringVar(&setProfileESAPIKey, "es-api-key", "", "Elasticsearch API key (supports ${ENV_VAR} syntax)")
	setProfileCmd.Flags().StringVar(&setProfileESUsername, "es-username", "", "Elasticsearch username")
	setProfileCmd.Flags().StringVar(&setProfileESPassword, "es-password", "", "Elasticsearch password (supports ${ENV_VAR} syntax)")
	setProfileCmd.Flags().StringVar(&setProfileOTLP, "otlp", "", "OTLP endpoint")
	setProfileCmd.Flags().BoolVar(&setProfileOTLPInsec, "otlp-insecure", true, "Use insecure OTLP connection")
	setProfileCmd.Flags().StringVar(&setProfileKibanaURL, "kibana-url", "", "Kibana URL")
	setProfileCmd.Flags().StringVar(&setProfileKibanaSpace, "kibana-space", "", "Kibana space (e.g., 'elasticat')")

	// Add subcommands
	configCmd.AddCommand(useProfileCmd)
	configCmd.AddCommand(setProfileCmd)
	configCmd.AddCommand(getProfilesCmd)
	configCmd.AddCommand(currentProfileCmd)
	configCmd.AddCommand(deleteProfileCmd)
	configCmd.AddCommand(viewConfigCmd)
	configCmd.AddCommand(configPathCmd)

	rootCmd.AddCommand(configCmd)
}

// formatProfileSummary returns a brief summary of a profile's settings.
func formatProfileSummary(p config.Profile) string {
	var parts []string
	if p.Elasticsearch.URL != "" {
		parts = append(parts, fmt.Sprintf("es=%s", p.Elasticsearch.URL))
	}
	if p.OTLP.Endpoint != "" {
		parts = append(parts, fmt.Sprintf("otlp=%s", p.OTLP.Endpoint))
	}
	if p.Kibana.URL != "" {
		kibanaStr := p.Kibana.URL
		if p.Kibana.Space != "" {
			kibanaStr = fmt.Sprintf("%s (space: %s)", p.Kibana.URL, p.Kibana.Space)
		}
		parts = append(parts, fmt.Sprintf("kibana=%s", kibanaStr))
	}
	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, ", ")
}
