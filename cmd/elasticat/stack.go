// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/elastic/elasticat/internal/config"
	"github.com/elastic/elasticat/internal/es"
	"github.com/spf13/cobra"
)

var (
	dockerDir  string
	withKibana bool
	withMCP    bool
	noKibana   bool
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the ElastiCat stack (Elasticsearch + OTel Collector)",
	Long:  `Starts the Docker Compose stack including Elasticsearch and the OpenTelemetry Collector.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Kibana is enabled by default; allow users to explicitly disable it.
		if noKibana {
			withKibana = false
		}
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
		return runStatus(cmd.Context())
	},
}

func init() {
	upCmd.Flags().StringVar(&dockerDir, "dir", "", "Docker compose directory (default: auto-detect)")
	upCmd.Flags().BoolVar(&withKibana, "kibana", true, "Start Kibana for advanced visualization (default: true)")
	upCmd.Flags().BoolVar(&noKibana, "no-kibana", false, "Disable Kibana (overrides --kibana)")
	upCmd.Flags().BoolVar(&withMCP, "mcp", false, "Also start the Elasticsearch MCP server for AI assistant integration")

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
}

type dockerDirSearch struct {
	getwd              func() (string, error)
	executable         func() (string, error)
	stat               func(string) (os.FileInfo, error)
	elasticatDataDirFn func() (string, error)
}

func defaultDockerDirSearch() dockerDirSearch {
	return dockerDirSearch{
		getwd:              os.Getwd,
		executable:         os.Executable,
		stat:               os.Stat,
		elasticatDataDirFn: defaultElasticatDataDir,
	}
}

func defaultElasticatDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "elasticat"), nil
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "elasticat"), nil
		}
		return filepath.Join(home, "AppData", "Roaming", "elasticat"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "elasticat"), nil
		}
		return filepath.Join(home, ".local", "share", "elasticat"), nil
	}
}

func findDockerDir() (string, error) {
	return findDockerDirWith(defaultDockerDirSearch())
}

func findDockerDirWith(s dockerDirSearch) (string, error) {
	if s.getwd == nil {
		s.getwd = os.Getwd
	}
	if s.executable == nil {
		s.executable = os.Executable
	}
	if s.stat == nil {
		s.stat = os.Stat
	}
	if s.elasticatDataDirFn == nil {
		s.elasticatDataDirFn = defaultElasticatDataDir
	}

	// Check if we're in the elasticat directory
	cwd, err := s.getwd()
	if err != nil {
		return "", err
	}

	// Check ./docker
	dockerPath := filepath.Join(cwd, "docker")
	if _, err := s.stat(filepath.Join(dockerPath, "docker-compose.yml")); err == nil {
		return dockerPath, nil
	}

	// Check if docker-compose.yml is in current dir
	if _, err := s.stat(filepath.Join(cwd, "docker-compose.yml")); err == nil {
		return cwd, nil
	}

	// Check user data directory (release installs put docker/ here)
	if dataDir, err := s.elasticatDataDirFn(); err == nil && dataDir != "" {
		dataDockerPath := filepath.Join(dataDir, "docker")
		if _, err := s.stat(filepath.Join(dataDockerPath, "docker-compose.yml")); err == nil {
			return dataDockerPath, nil
		}
	}

	// Check executable directory
	exePath, err := s.executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		dockerPath = filepath.Join(exeDir, "docker")
		if _, err := s.stat(filepath.Join(dockerPath, "docker-compose.yml")); err == nil {
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
