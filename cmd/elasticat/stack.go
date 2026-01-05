// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/elastic/elasticat/internal/es"
	"github.com/spf13/cobra"
)

var (
	dockerDir  string
	withKibana bool
	withMCP    bool
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the ElastiCat stack (Elasticsearch + OTel Collector)",
	Long:  `Starts the Docker Compose stack including Elasticsearch and the OpenTelemetry Collector.`,
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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the ElastiCat stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus()
	},
}

func init() {
	upCmd.Flags().StringVar(&dockerDir, "dir", "", "Docker compose directory (default: auto-detect)")
	upCmd.Flags().BoolVar(&withKibana, "kibana", false, "Also start Kibana for advanced visualization")
	upCmd.Flags().BoolVar(&withMCP, "mcp", false, "Also start the Elasticsearch MCP server for AI assistant integration")

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
}

func findDockerDir() (string, error) {
	// Check if we're in the elasticat directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check ./docker
	dockerPath := filepath.Join(cwd, "docker")
	if _, err := os.Stat(filepath.Join(dockerPath, "docker-compose.yml")); err == nil {
		return dockerPath, nil
	}

	// Check if docker-compose.yml is in current dir
	if _, err := os.Stat(filepath.Join(cwd, "docker-compose.yml")); err == nil {
		return cwd, nil
	}

	// Check executable directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		dockerPath = filepath.Join(exeDir, "docker")
		if _, err := os.Stat(filepath.Join(dockerPath, "docker-compose.yml")); err == nil {
			return dockerPath, nil
		}
	}

	return "", fmt.Errorf("could not find docker-compose.yml. Use --dir to specify the path")
}

// getContainerRuntime returns "docker" or "podman" depending on what's available
// It also verifies that compose functionality is available
func getContainerRuntime() (string, error) {
	// Check for docker first
	if _, err := exec.LookPath("docker"); err == nil {
		// Verify docker compose is available
		cmd := exec.Command("docker", "compose", "version")
		if err := cmd.Run(); err == nil {
			return "docker", nil
		}
	}

	// Fall back to podman
	if _, err := exec.LookPath("podman"); err == nil {
		// Verify podman compose is available (either built-in or podman-compose)
		cmd := exec.Command("podman", "compose", "version")
		if err := cmd.Run(); err == nil {
			return "podman", nil
		}
		// podman exists but compose doesn't work
		return "", fmt.Errorf("podman found but 'podman compose' is not available.\n\nInstall it with:\n  Fedora/RHEL: sudo dnf install podman-compose\n  Ubuntu/Debian: sudo apt install podman-compose\n  pip: pip install podman-compose")
	}

	return "", fmt.Errorf("neither docker nor podman found in PATH.\n\nInstall one of:\n  Docker: https://docs.docker.com/get-docker/\n  Podman: https://podman.io/getting-started/installation")
}

func runUp() error {
	dir := dockerDir
	if dir == "" {
		var err error
		dir, err = findDockerDir()
		if err != nil {
			return err
		}
	}

	runtime, err := getContainerRuntime()
	if err != nil {
		return err
	}

	fmt.Println("Starting ElastiCat stack...")
	fmt.Printf("Using %s, compose directory: %s\n", runtime, dir)
	fmt.Println()

	args := []string{"compose"}
	if withKibana {
		args = append(args, "--profile", "kibana")
	}
	if withMCP {
		args = append(args, "--profile", "mcp")
	}
	args = append(args, "up", "-d")

	cmd := exec.Command(runtime, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start stack: %w", err)
	}

	fmt.Println()
	fmt.Println("Stack started successfully!")
	fmt.Println()
	fmt.Println("  Elasticsearch:  http://localhost:9200")
	fmt.Println("  OTel Collector: localhost:4317 (gRPC), localhost:4318 (HTTP)")
	if withKibana {
		fmt.Println("  Kibana:         http://localhost:5601")
	}
	if withMCP {
		fmt.Println("  MCP Server:     http://localhost:3000")
		fmt.Println()
		fmt.Println("To configure your AI assistant, add the MCP server endpoint to your config.")
	}
	fmt.Println()
	fmt.Println("Run 'elasticat ui' to open the log viewer.")

	return nil
}

func runDown() error {
	dir := dockerDir
	if dir == "" {
		var err error
		dir, err = findDockerDir()
		if err != nil {
			return err
		}
	}

	runtime, err := getContainerRuntime()
	if err != nil {
		return err
	}

	fmt.Println("Stopping ElastiCat stack...")

	cmd := exec.Command(runtime, "compose", "down")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop stack: %w", err)
	}

	fmt.Println("Stack stopped.")
	return nil
}

func runStatus() error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("ElastiCat Status")
	fmt.Println("================")
	fmt.Println()

	// Check Elasticsearch
	fmt.Printf("Elasticsearch (%s): ", esURL)
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

	// Check containers
	runtime, err := getContainerRuntime()
	if err != nil {
		fmt.Printf("Containers: %v\n", err)
		return nil
	}

	fmt.Printf("Containers (%s):\n", runtime)
	cmd := exec.Command(runtime, "ps", "--filter", "name=elasticat", "--format", "  {{.Names}}: {{.Status}}")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("  Could not check containers")
	}

	return nil
}
