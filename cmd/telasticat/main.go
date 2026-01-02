package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/andrewvc/turboelasticat/internal/es"
	"github.com/andrewvc/turboelasticat/internal/otlp"
	"github.com/andrewvc/turboelasticat/internal/tui"
	"github.com/andrewvc/turboelasticat/internal/watch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	// Flags
	esURL       string
	esIndex     string
	dockerDir   string
	withKibana  bool
	withMCP     bool
	serviceFlag string
	levelFlag   string
	followFlag  bool
	jsonFlag    bool

	// Watch command flags
	watchLines   int
	watchNoColor bool
	watchOTLP    string
	watchNoSend  bool
	watchOneshot bool

	// Clear command flags
	clearForce bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "telasticat",
	Short: "AI-powered local development log viewer",
	Long: `TurboElasticat - View and search your local development logs with the power of Elasticsearch.

Start the stack with 'telasticat up', then view logs with 'telasticat logs'.
Your AI assistant can query logs via the Elasticsearch MCP server.`,
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the TurboElasticat stack (Elasticsearch + OTel Collector)",
	Long:  `Starts the Docker Compose stack including Elasticsearch and the OpenTelemetry Collector.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUp()
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the TurboElasticat stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDown()
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Open the interactive log viewer TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI(tui.SignalLogs)
	},
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Open the interactive metrics viewer TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI(tui.SignalMetrics)
	},
}

var tracesCmd = &cobra.Command{
	Use:   "traces",
	Short: "Open the interactive traces viewer TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI(tui.SignalTraces)
	},
}

var tailCmd = &cobra.Command{
	Use:   "tail [service]",
	Short: "Tail logs in real-time (non-interactive)",
	RunE: func(cmd *cobra.Command, args []string) error {
		service := ""
		if len(args) > 0 {
			service = args[0]
		}
		return runTail(service)
	},
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search logs with ES query syntax",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		return runSearch(query)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the TurboElasticat stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus()
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch <file>...",
	Short: "Watch log files like tail -F and send to Elasticsearch",
	Long: `Watch one or more log files in real-time, displaying them with colors
and automatically sending them to Elasticsearch via OTLP.

Examples:
  telasticat watch server.log
  telasticat watch server.log server-err.log
  telasticat watch ./logs/*.log
  telasticat watch -n 50 server.log     # Show last 50 lines
  telasticat watch --no-send server.log # Display only, don't send to ES`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWatch(args)
	},
}

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all collected logs from Elasticsearch",
	Long: `Clears all logs from Elasticsearch. Useful during development
when you want a fresh start.

Examples:
  telasticat clear          # Prompts for confirmation
  telasticat clear --force  # Skip confirmation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClear()
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&esURL, "es-url", "http://localhost:9200", "Elasticsearch URL")
	rootCmd.PersistentFlags().StringVar(&esIndex, "index", "logs-*", "Elasticsearch index/data stream pattern (e.g., 'logs-*', 'logs-myapp-*')")

	// Up command flags
	upCmd.Flags().StringVar(&dockerDir, "dir", "", "Docker compose directory (default: auto-detect)")
	upCmd.Flags().BoolVar(&withKibana, "kibana", false, "Also start Kibana for advanced visualization")
	upCmd.Flags().BoolVar(&withMCP, "mcp", false, "Also start the Elasticsearch MCP server for AI assistant integration")

	// Tail/Search flags
	tailCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Filter by service name")
	tailCmd.Flags().StringVarP(&levelFlag, "level", "l", "", "Filter by log level (ERROR, WARN, INFO, DEBUG)")
	tailCmd.Flags().BoolVarP(&followFlag, "follow", "f", true, "Follow logs in real-time")

	searchCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Filter by service name")
	searchCmd.Flags().StringVarP(&levelFlag, "level", "l", "", "Filter by log level")
	searchCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")

	// Watch command flags
	watchCmd.Flags().IntVarP(&watchLines, "lines", "n", 10, "Number of lines to show from end of file")
	watchCmd.Flags().BoolVar(&watchNoColor, "no-color", false, "Disable colored output")
	watchCmd.Flags().StringVarP(&serviceFlag, "service", "s", "", "Override service name (otherwise derived from filename)")
	watchCmd.Flags().StringVar(&watchOTLP, "otlp", "localhost:4318", "OTLP HTTP endpoint")
	watchCmd.Flags().BoolVar(&watchNoSend, "no-send", false, "Don't send logs to Elasticsearch, display only")
	watchCmd.Flags().BoolVar(&watchOneshot, "oneshot", false, "Import all logs and exit (don't follow)")

	// Clear command flags
	clearCmd.Flags().BoolVarP(&clearForce, "force", "f", false, "Skip confirmation prompt")

	// Add commands
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(tracesCmd)
	rootCmd.AddCommand(tailCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(clearCmd)
}

func findDockerDir() (string, error) {
	// Check if we're in the turbodevlog directory
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

	fmt.Println("Starting TurboElasticat stack...")
	fmt.Printf("Using %s, compose directory: %s\n", runtime, dir)
	fmt.Println()

	args := []string{"compose", "up", "-d"}
	if withKibana {
		args = append(args, "--profile", "kibana")
	}
	if withMCP {
		args = append(args, "--profile", "mcp")
	}

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
	fmt.Println("Run 'telasticat logs' to open the log viewer.")

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

	fmt.Println("Stopping TurboElasticat stack...")

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

func runTUI(signal tui.SignalType) error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	// Check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		fmt.Println("Warning: Could not connect to Elasticsearch. Is the stack running?")
		fmt.Println("Run 'telasticat up' to start the stack.")
		fmt.Println()
	}

	model := tui.NewModel(client, signal)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

func runTail(service string) error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx := context.Background()

	// Use service from flag if provided
	if serviceFlag != "" {
		service = serviceFlag
	}

	fmt.Printf("Tailing logs")
	if service != "" {
		fmt.Printf(" for service: %s", service)
	}
	if levelFlag != "" {
		fmt.Printf(" (level: %s)", levelFlag)
	}
	fmt.Println("... (Ctrl+C to stop)")
	fmt.Println()

	var lastTimestamp time.Time

	for {
		opts := es.TailOptions{
			Size:    50,
			Service: service,
			Level:   levelFlag,
			Since:   lastTimestamp,
		}

		result, err := client.Tail(ctx, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching logs: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Print logs in reverse order (oldest first)
		for i := len(result.Logs) - 1; i >= 0; i-- {
			log := result.Logs[i]
			if log.Timestamp.After(lastTimestamp) {
				printLogLine(log)
				lastTimestamp = log.Timestamp
			}
		}

		if !followFlag {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func runSearch(query string) error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.Search(ctx, query, es.SearchOptions{
		Size:    100,
		Service: serviceFlag,
		Level:   levelFlag,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	fmt.Printf("Found %d logs matching '%s'\n\n", result.Total, query)

	for _, log := range result.Logs {
		printLogLine(log)
	}

	return nil
}

func runStatus() error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("TurboElasticat Status")
	fmt.Println("=====================")
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
	cmd := exec.Command(runtime, "ps", "--filter", "name=turboelasticat", "--format", "  {{.Names}}: {{.Status}}")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("  Could not check containers")
	}

	return nil
}

func printLogLine(log es.LogEntry) {
	ts := log.Timestamp.Format("15:04:05.000")
	level := log.GetLevel()
	service := log.ServiceName
	if service == "" && log.ContainerID != "" {
		service = log.ContainerID[:min(12, len(log.ContainerID))]
	}
	if service == "" {
		service = "unknown"
	}

	msg := log.GetMessage()

	// Color the level
	levelColor := "\033[0m"
	switch level {
	case "ERROR", "FATAL", "error", "fatal":
		levelColor = "\033[31m" // Red
	case "WARN", "WARNING", "warn", "warning":
		levelColor = "\033[33m" // Yellow
	case "INFO", "info":
		levelColor = "\033[32m" // Green
	case "DEBUG", "debug":
		levelColor = "\033[36m" // Cyan
	}

	fmt.Printf("%s %s%-5s\033[0m [%s] %s\n", ts, levelColor, level, service, msg)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func runClear() error {
	client, err := es.New([]string{esURL}, esIndex)
	if err != nil {
		return fmt.Errorf("failed to create ES client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check connection first
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("cannot connect to Elasticsearch: %w\nIs the stack running? Try 'telasticat up'", err)
	}

	// Prompt for confirmation unless --force
	if !clearForce {
		fmt.Print("This will delete ALL collected logs. Are you sure? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("Clearing logs...")

	deleted, err := client.Clear(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear logs: %w", err)
	}

	fmt.Printf("Deleted %d logs.\n", deleted)
	return nil
}
