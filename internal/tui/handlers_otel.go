// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/fsnotify/fsnotify"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/config"
)

// handleOtelConfigExplainKey handles keys in the OTel config explanation modal.
func (m Model) handleOtelConfigExplainKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "o", "O":
		// User confirmed - proceed to open the editor
		m.popView() // Remove explain modal
		return m, m.handleOtelConfig()
	case "esc":
		// User cancelled
		m.popView()
		return m, nil
	}
	return m, nil
}

// handleOtelConfigUnavailableKey handles keys in the OTel config unavailable modal.
func (m Model) handleOtelConfigUnavailableKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "o", "O":
		m.popView()
		return m, nil
	}
	return m, nil
}

// handleOtelConfigModalKey handles keys in the OTel config watching modal.
func (m Model) handleOtelConfigModalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		// Copy config path to clipboard
		if m.Otel.ConfigPath != "" {
			m.copyToClipboard(m.Otel.ConfigPath, "Path copied to clipboard!")
		}
		return m, nil
	case "Y":
		// Copy error to clipboard (validation or reload error)
		if !m.Otel.ValidationValid && m.Otel.ValidationStatus != "" {
			m.copyToClipboard(m.Otel.ValidationStatus, "Error copied to clipboard!")
		} else if m.Otel.ReloadError != nil {
			m.copyToClipboard(m.Otel.ReloadError.Error(), "Error copied to clipboard!")
		}
		return m, nil
	case "esc":
		// Close modal and stop watching
		m.popView()
		m.Otel.WatchingConfig = false
		return m, nil
	}
	return m, nil
}

// OtelConfigReloadedMsg is sent when the OTel collector config is reloaded.
type OtelConfigReloadedMsg struct {
	Err error
}

// OtelConfigOpenedMsg is sent after attempting to open the OTel config.
type OtelConfigOpenedMsg struct {
	Err        error
	ConfigPath string
	Extracted  bool // true if config was freshly extracted
}

// handleOtelConfig handles the 'O' key to open OTel collector config.
// Returns a command that performs the operation asynchronously.
func (m *Model) handleOtelConfig() tea.Cmd {
	return func() (msg tea.Msg) {
		// Catch panics and convert to error
		defer func() {
			if r := recover(); r != nil {
				msg = OtelConfigOpenedMsg{
					Err: fmt.Errorf("panic in handleOtelConfig: %v", r),
				}
			}
		}()

		// Check if start-local stack is installed
		if !config.IsStartLocalInstalled() {
			return OtelConfigOpenedMsg{
				Err: fmt.Errorf("stack not installed. Run 'elasticat up' first"),
			}
		}

		needsRecreate := false

		// Check if docker-compose.yml is properly configured for file mount
		if !config.IsComposeUsingFilemount() {
			// Need to fix docker-compose.yml (and extract config if needed)
			if err := config.ExtractOtelConfig(); err != nil {
				return OtelConfigOpenedMsg{
					Err: fmt.Errorf("failed to configure file mount: %w", err),
				}
			}
			needsRecreate = true
		}

		// Check if running container has the file mount
		if !config.IsCollectorUsingFilemount() {
			needsRecreate = true
		}

		// Recreate collector if docker-compose or container mount was wrong
		if needsRecreate {
			if err := config.RecreateCollector(); err != nil {
				return OtelConfigOpenedMsg{
					Err: fmt.Errorf("failed to recreate collector: %w", err),
				}
			}
		}

		// Get the config path
		configPath, err := config.GetOtelConfigPath()
		if err != nil {
			return OtelConfigOpenedMsg{
				Err: fmt.Errorf("failed to get config path: %w", err),
			}
		}

		// Open in editor
		if err := openFileInEditor(configPath); err != nil {
			return OtelConfigOpenedMsg{
				Err: fmt.Errorf("failed to open editor: %w", err),
			}
		}

		return OtelConfigOpenedMsg{
			ConfigPath: configPath,
			Extracted:  false, // container already recreated if needed
		}
	}
}

// openFileInEditor opens the given file in the system's default editor.
// On macOS, this uses 'open' which opens in the default app for the file type.
// On Linux, this uses xdg-open.
// On Windows, this uses 'start'.
func openFileInEditor(filepath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", filepath)
	case "linux":
		cmd = exec.Command("xdg-open", filepath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", filepath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// reloadOtelCollector sends a SIGHUP to the collector container.
// Returns a command that emits OtelConfigReloadedMsg.
func reloadOtelCollector() tea.Cmd {
	return func() tea.Msg {
		err := config.ReloadOtelCollector()
		return OtelConfigReloadedMsg{Err: err}
	}
}

// watchOtelConfig watches the config file for changes and sends messages when modified.
// Returns a command that blocks and sends messages on file events.
func watchOtelConfig(configPath string) tea.Cmd {
	return func() tea.Msg {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return otelWatcherErrorMsg{Err: fmt.Errorf("create watcher: %w", err)}
		}
		defer watcher.Close()

		if err := watcher.Add(configPath); err != nil {
			return otelWatcherErrorMsg{Err: fmt.Errorf("watch file: %w", err)}
		}

		// Wait for a write event
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return otelWatcherErrorMsg{Err: fmt.Errorf("watcher closed")}
				}
				// Check for write or create events (some editors recreate the file)
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					return otelFileChangedMsg{Path: configPath}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return otelWatcherErrorMsg{Err: fmt.Errorf("watcher closed")}
				}
				return otelWatcherErrorMsg{Err: err}
			}
		}
	}
}

// otelValidatedMsg is sent after validating the OTel config.
type otelValidatedMsg struct {
	Valid   bool
	Message string
}

// handleOtelFileChanged handles a file change event by validating then reloading.
func (m *Model) handleOtelFileChanged() tea.Cmd {
	return func() tea.Msg {
		// First validate the config
		result := config.ValidateOtelConfig()
		return otelValidatedMsg{
			Valid:   result.Valid,
			Message: result.Message,
		}
	}
}

// handleOtelValidated handles validation result - if valid, proceeds to reload.
func (m *Model) handleOtelValidated(valid bool, message string) tea.Cmd {
	m.Otel.ValidationValid = valid
	m.Otel.ValidationStatus = message

	if !valid {
		// Config is invalid - don't reload, keep watching
		return watchOtelConfig(m.Otel.ConfigPath)
	}

	// Config is valid - proceed to reload
	return func() tea.Msg {
		err := config.ReloadOtelCollector()
		return otelReloadedMsg{
			Time: time.Now(),
			Err:  err,
		}
	}
}
