// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/elastic/elasticat/internal/config"
	"github.com/elastic/elasticat/internal/es"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the ElastiCat stack (Elasticsearch + Kibana + EDOT Collector)",
	Long: `Starts the Elastic stack using the official start-local script.

This installs Elasticsearch, Kibana, and the EDOT (Elastic Distribution of
OpenTelemetry) collector for local development. Credentials are automatically
saved to the 'elastic-start-local' profile.

Learn more at https://github.com/elastic/start-local`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUp()
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the ElastiCat stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDown()
	},
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Stop the ElastiCat stack and remove all data",
	Long:  `Stops the stack and removes all containers, networks, and volumes including Elasticsearch data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDestroy()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the ElastiCat stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus(cmd.Context())
	},
}

var credsCmd = &cobra.Command{
	Use:   "creds",
	Short: "Display the Kibana/Elasticsearch credentials",
	Long:  `Displays the username, password, and API key for the local Elastic stack.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreds()
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(credsCmd)
}

// createElasticatSpace creates the 'elasticat' Kibana space with Observability solution view.
// This provides a focused UI for log viewing and observability.
func createElasticatSpace() error {
	env, err := config.LoadStartLocalEnv()
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	kibanaURL := fmt.Sprintf("http://localhost:%s", env.KibanaPort)
	if env.KibanaPort == "" {
		kibanaURL = "http://localhost:5601"
	}

	// Wait for Kibana to be ready (up to 60 seconds)
	fmt.Print("Waiting for Kibana to be ready...")
	client := &http.Client{Timeout: 5 * time.Second}
	statusURL := kibanaURL + "/api/status"

	for i := 0; i < 12; i++ {
		req, _ := http.NewRequest("GET", statusURL, nil)
		req.SetBasicAuth("elastic", env.ESPassword)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				fmt.Println(" ready!")
				break
			}
		}
		if i == 11 {
			fmt.Println(" timeout (continuing anyway)")
			return nil // Don't fail the whole operation
		}
		fmt.Print(".")
		time.Sleep(5 * time.Second)
	}

	// Create the elasticat space
	spaceJSON := `{"id":"elasticat","name":"ElastiCat","description":"Observability space for ElastiCat","color":"#F06292","initials":"EC","disabledFeatures":[],"solution":"oblt"}`

	req, err := http.NewRequest("POST", kibanaURL+"/api/spaces/space", bytes.NewBufferString(spaceJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("kbn-xsrf", "true")
	req.SetBasicAuth("elastic", env.ESPassword)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("create space request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		fmt.Println("Created 'elasticat' Kibana space with Observability solution view")
		fmt.Printf("  Access it at: %s/s/elasticat\n", kibanaURL)
	} else if resp.StatusCode == 409 {
		// Space already exists, that's fine
		fmt.Println("Kibana 'elasticat' space already exists")
	} else {
		// Don't fail, just warn
		fmt.Printf("Warning: could not create Kibana space (status %d)\n", resp.StatusCode)
	}

	return nil
}

// ensureStartLocalProfile creates the elastic-start-local profile if it doesn't exist
// and sets it as the current profile.
func ensureStartLocalProfile() error {
	cfg, err := config.LoadProfiles()
	if err != nil {
		return fmt.Errorf("load profiles: %w", err)
	}

	// Check if profile already exists
	_, err = cfg.GetProfile(config.StartLocalProfileName)
	if err != nil {
		// Profile doesn't exist, create it
		cfg.SetProfile(config.StartLocalProfileName, config.Profile{
			Source: config.ProfileSourceStartLocal,
		})
	}

	// Set as current profile
	cfg.CurrentProfile = config.StartLocalProfileName

	if err := config.SaveProfiles(cfg); err != nil {
		return fmt.Errorf("save profiles: %w", err)
	}

	return nil
}

func runUp() error {
	startLocalDir, err := config.GetStartLocalDir()
	if err != nil {
		return fmt.Errorf("get start-local directory: %w", err)
	}

	if config.IsStartLocalInstalled() {
		// Stack already installed, just start it
		fmt.Println("Starting ElastiCat stack...")
		fmt.Println()

		startScript := filepath.Join(startLocalDir, "start.sh")
		cmd := exec.Command("sh", startScript)
		cmd.Dir = startLocalDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start stack: %w", err)
		}
	} else {
		// First-time installation
		fmt.Println("Installing ElastiCat stack using Elastic start-local...")
		fmt.Println()

		// Ensure parent directory exists (~/.elasticat/)
		parentDir := filepath.Dir(startLocalDir)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", parentDir, err)
		}

		// Run the start-local installer with EDOT
		// ES_LOCAL_DIR should be just the folder name, not the full path
		// start-local creates the folder in the current working directory
		cmd := exec.Command("sh", "-c", "curl -fsSL https://elastic.co/start-local | sh -s -- --edot")
		cmd.Dir = parentDir
		cmd.Env = append(os.Environ(), fmt.Sprintf("ES_LOCAL_DIR=%s", config.StartLocalDirName))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install stack: %w", err)
		}
	}

	// Ensure profile exists and is set as current
	if err := ensureStartLocalProfile(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save profile: %v\n", err)
	} else {
		fmt.Println()
		fmt.Printf("Profile '%s' created and set as current.\n", config.StartLocalProfileName)
	}

	// Create the elasticat Kibana space with Observability solution view
	fmt.Println()
	if err := createElasticatSpace(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create Kibana space: %v\n", err)
	}

	fmt.Println()
	fmt.Println("Run 'elasticat ui' to open the log viewer.")

	return nil
}

func runDown() error {
	if !config.IsStartLocalInstalled() {
		return fmt.Errorf("stack not installed. Run 'elasticat up' first")
	}

	startLocalDir, err := config.GetStartLocalDir()
	if err != nil {
		return fmt.Errorf("get start-local directory: %w", err)
	}

	fmt.Println("Stopping ElastiCat stack...")

	stopScript := filepath.Join(startLocalDir, "stop.sh")
	cmd := exec.Command("sh", stopScript)
	cmd.Dir = startLocalDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop stack: %w", err)
	}

	fmt.Println("Stack stopped.")
	return nil
}

func runDestroy() error {
	if !config.IsStartLocalInstalled() {
		return fmt.Errorf("stack not installed. Nothing to destroy")
	}

	startLocalDir, err := config.GetStartLocalDir()
	if err != nil {
		return fmt.Errorf("get start-local directory: %w", err)
	}

	fmt.Println("Destroying ElastiCat stack (removing containers, networks, and volumes)...")

	uninstallScript := filepath.Join(startLocalDir, "uninstall.sh")
	cmd := exec.Command("sh", uninstallScript)
	cmd.Dir = startLocalDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall stack: %w", err)
	}

	// Optionally remove the profile
	cfg, err := config.LoadProfiles()
	if err == nil {
		if err := cfg.DeleteProfile(config.StartLocalProfileName); err == nil {
			if err := config.SaveProfiles(cfg); err == nil {
				fmt.Printf("Profile '%s' removed.\n", config.StartLocalProfileName)
			}
		}
	}

	fmt.Println("Stack destroyed. All data has been removed.")
	return nil
}

func runCreds() error {
	if !config.IsStartLocalInstalled() {
		return fmt.Errorf("stack not installed. Run 'elasticat up' first")
	}

	env, err := config.LoadStartLocalEnv()
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	kibanaPort := env.KibanaPort
	if kibanaPort == "" {
		kibanaPort = "5601"
	}

	fmt.Println("ElastiCat Stack Credentials")
	fmt.Println("===========================")
	fmt.Println()
	fmt.Println("Kibana:")
	fmt.Printf("  URL:      http://localhost:%s\n", kibanaPort)
	fmt.Printf("  Space:    http://localhost:%s/s/elasticat\n", kibanaPort)
	fmt.Println("  Username: elastic")
	fmt.Printf("  Password: %s\n", env.ESPassword)
	fmt.Println()
	fmt.Println("Elasticsearch:")
	fmt.Printf("  URL:      %s\n", env.ESURL)
	fmt.Println("  Username: elastic")
	fmt.Printf("  Password: %s\n", env.ESPassword)
	fmt.Printf("  API Key:  %s\n", env.ESAPIKey)
	fmt.Println()
	fmt.Println("OTLP Endpoints:")
	fmt.Println("  gRPC: http://localhost:4317")
	fmt.Println("  HTTP: http://localhost:4318")

	return nil
}

func runStatus(parentCtx context.Context) error {
	cfg, ok := config.FromContext(parentCtx)
	if !ok {
		return fmt.Errorf("configuration not loaded")
	}

	client, err := es.NewFromConfig(cfg.ES.URL, cfg.ES.Index, cfg.ES.APIKey, cfg.ES.Username, cfg.ES.Password)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(parentCtx, cfg.ES.PingTimeout)
	defer cancel()

	fmt.Println("ElastiCat Status")
	fmt.Println("================")
	fmt.Println()

	// Show installation status
	fmt.Print("Installation: ")
	if config.IsStartLocalInstalled() {
		fmt.Println("INSTALLED")
		startLocalDir, _ := config.GetStartLocalDir()
		fmt.Printf("  Directory: %s\n", startLocalDir)
	} else {
		fmt.Println("NOT INSTALLED")
		fmt.Println("  Run 'elasticat up' to install the stack")
	}
	fmt.Println()

	// Check Elasticsearch
	fmt.Printf("Elasticsearch (%s): ", cfg.ES.URL)
	if err := client.Ping(ctx); err != nil {
		fmt.Println("NOT CONNECTED")
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Println("CONNECTED")

		// Get log count
		result, err := client.Tail(ctx, es.TailOptions{Size: 1})
		if err == nil {
			fmt.Printf("  Total logs indexed: %d\n", result.Total)
		}
	}

	fmt.Println()

	// Check containers using docker
	runtime := "docker"
	if _, err := exec.LookPath("docker"); err != nil {
		if _, err := exec.LookPath("podman"); err == nil {
			runtime = "podman"
		} else {
			fmt.Println("Containers: docker/podman not found")
			return nil
		}
	}

	fmt.Printf("Containers (%s):\n", runtime)
	// start-local uses container names like es-local-dev, kibana-local-dev, edot-collector
	cmd := exec.Command(runtime, "ps", "--filter", "name=es-local", "--filter", "name=kibana-local", "--filter", "name=edot-collector", "--format", "  {{.Names}}: {{.Status}}")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("  Could not check containers")
	}

	return nil
}
