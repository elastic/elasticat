// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderQueryOverlay renders a floating window with the ES query
func (m Model) renderQueryOverlay() string {
	var b strings.Builder

	index := m.Query.LastIndex

	// Header showing format and status
	var formatLabel string
	if m.Query.Format == formatKibana {
		formatLabel = "Kibana Dev Tools"
	} else {
		formatLabel = "curl"
	}

	header := fmt.Sprintf("Query (%s)", formatLabel)
	b.WriteString(QueryHeaderStyle.Render(header))

	// Show status message if recent (within 2 seconds)
	justCopied := m.UI.StatusMessage == "Copied to clipboard!" && time.Since(m.UI.StatusTime) < 2*time.Second
	if justCopied {
		b.WriteString("  ")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render(m.UI.StatusMessage))
	}
	b.WriteString("\n\n")

	if m.Query.Format == formatKibana {
		// Kibana Dev Tools format
		b.WriteString(QueryMethodStyle.Render("GET "))
		b.WriteString(QueryPathStyle.Render(index + "/_search"))
		b.WriteString("\n")
		b.WriteString(QueryBodyStyle.Render(m.Query.LastJSON))
	} else {
		// curl format
		b.WriteString(QueryBodyStyle.Render("curl -X GET 'http://localhost:9200/" + index + "/_search' \\\n"))
		b.WriteString(QueryBodyStyle.Render("  -H 'Content-Type: application/json' \\\n"))
		b.WriteString(QueryBodyStyle.Render("  -d '"))
		// Compact JSON for curl
		var compact bytes.Buffer
		if err := json.Compact(&compact, []byte(m.Query.LastJSON)); err == nil {
			b.WriteString(QueryBodyStyle.Render(compact.String()))
		} else {
			b.WriteString(QueryBodyStyle.Render(m.Query.LastJSON))
		}
		b.WriteString(QueryBodyStyle.Render("'"))
	}

	b.WriteString("\n\n")

	// Key hints
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)

	var copyHint string
	if justCopied {
		copyHint = highlightStyle.Render(keysHint("copied ✓", "y"))
	} else {
		copyHint = highlightStyle.Render(keysHint("copy", "y"))
	}

	hints := lipgloss.JoinHorizontal(
		lipgloss.Left,
		copyHint,
		"  ",
		dimStyle.Render(keysHint("kibana", "k")),
		"  ",
		dimStyle.Render(keysHint("curl", "c")),
		"  ",
		dimStyle.Render(keysHint("close", "esc", "q")),
	)
	b.WriteString(hints)

	// Calculate height - use full screen height since this replaces the log list
	height := m.getFullScreenHeight()
	if height < 10 {
		height = 10
	}

	return QueryOverlayStyle.Width(m.UI.Width - 8).Height(height).Render(b.String())
}

func (m Model) renderErrorModal() string {
	// Modal dimensions
	modalWidth := min(m.UI.Width-8, 80)

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")). // Red border
		Padding(1, 2).
		Align(lipgloss.Left)

	// Error title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Render("⚠ Error")

	// Check if we just copied (statusMessage set within last 2 seconds)
	justCopied := m.UI.StatusMessage == "Error copied to clipboard!" &&
		time.Since(m.UI.StatusTime) < 2*time.Second

	// Scroll indicator
	scrollInfo := ""
	if m.Components.ErrorViewport.TotalLineCount() > m.Components.ErrorViewport.Height {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf(" (scroll: %d%%) ", int(m.Components.ErrorViewport.ScrollPercent()*100)))
	}

	// Action buttons
	var copyButton string
	if justCopied {
		copyButton = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Render(keysHint("Copy ✓ copied", "y"))
	} else {
		copyButton = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Render(actionHint(ActionCopy))
	}

	actions := lipgloss.JoinHorizontal(
		lipgloss.Left,
		copyButton,
		"  ",
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(keysHint("Scroll", "↑", "↓")),
		"  ",
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(actionHint(ActionBack)),
		"  ",
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(actionHint(ActionQuit)),
		scrollInfo,
	)

	// Get viewport content (wrap it in a style to constrain width)
	viewportContent := lipgloss.NewStyle().
		Width(modalWidth - 8). // Account for border (2) + padding (2*2) + margin (2)
		Render(m.Components.ErrorViewport.View())

	// Combine content with viewport for scrollable error message
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		viewportContent,
		"",
		actions,
	)

	// Render modal - lipgloss.Place() in View() handles centering
	return modalStyle.Render(content)
}

func (m Model) renderQuitConfirmModal() string {
	// Modal dimensions
	modalWidth := min(m.UI.Width-8, 80)

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Align(lipgloss.Left)

	title := lipgloss.NewStyle().
		Bold(true).
		Render("Quit?")

	body := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render("Are you sure you want to quit?")

	actions := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("Yes", quitConfirmYesKey)),
		"  ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(keyHint([]string{quitConfirmNoKey, "esc"}, "No")),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		body,
		"",
		actions,
	)
	return modalStyle.Render(content)
}

func (m Model) renderOtelConfigExplainModal() string {
	// Modal dimensions
	modalWidth := min(m.UI.Width-8, 65)

	// Modal box style - use cyan border
	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("45")). // Cyan border
		Padding(1, 2).
		Align(lipgloss.Left)

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("45")).
		Render("⚙ Open OTel Collector Config")

	var b strings.Builder

	// Explanation text
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))

	b.WriteString(dimStyle.Render("This will open the OpenTelemetry Collector configuration"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("file in your default editor."))
	b.WriteString("\n\n")

	b.WriteString(highlightStyle.Render("What happens:"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  • The config file will open in your editor"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  • Elasticat will watch the file for changes"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  • "))
	b.WriteString(yellowStyle.Render("Saving validates & hot-reloads the collector"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  • Invalid configs will show errors, not reload"))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("You can modify processors, exporters, and pipelines"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("to customize how telemetry is processed."))

	// Actions
	actions := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("open editor", "enter", "o")),
		"  ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(keysHint("cancel", "esc")),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		b.String(),
		"",
		actions,
	)

	return modalStyle.Render(content)
}

func (m Model) renderOtelConfigModal() string {
	// Modal dimensions
	modalWidth := min(m.UI.Width-8, 70)

	// Modal box style - use a nice cyan border for config
	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("45")). // Cyan border
		Padding(1, 2).
		Align(lipgloss.Left)

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("45")).
		Render("⚙ OTel Collector Config")

	var b strings.Builder

	// Label/value styles
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true) // Green
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)  // Red
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Show config path (never truncate - user needs to copy it)
	b.WriteString(labelStyle.Render("Config file:"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render("  " + m.Otel.ConfigPath))
	b.WriteString("\n\n")

	// Watching status
	b.WriteString(labelStyle.Render("Status:"))
	b.WriteString("\n")
	if m.Otel.WatchingConfig {
		b.WriteString(successStyle.Render("  ● Watching for changes"))
	} else {
		b.WriteString(dimStyle.Render("  ○ Not watching"))
	}
	b.WriteString("\n\n")

	// Validation status (if any)
	if m.Otel.ValidationStatus != "" {
		b.WriteString(labelStyle.Render("Validation:"))
		b.WriteString("\n")
		if m.Otel.ValidationValid {
			if m.Otel.ValidationStatus == "Validating config..." {
				b.WriteString(dimStyle.Render("  ⏳ " + m.Otel.ValidationStatus))
			} else {
				b.WriteString(successStyle.Render("  ✓ Config is valid"))
			}
		} else {
			// Show error with wrapped text
			b.WriteString(errorStyle.Render("  ✗ Invalid config:"))
			b.WriteString("\n")
			// Wrap long error messages
			errMsg := m.Otel.ValidationStatus
			if len(errMsg) > modalWidth-8 {
				// Split into multiple lines
				for len(errMsg) > 0 {
					lineLen := min(modalWidth-10, len(errMsg))
					b.WriteString(errorStyle.Render("    " + errMsg[:lineLen]))
					errMsg = errMsg[lineLen:]
					if len(errMsg) > 0 {
						b.WriteString("\n")
					}
				}
			} else {
				b.WriteString(errorStyle.Render("    " + errMsg))
			}
		}
		b.WriteString("\n\n")
	}

	// Last reload info
	b.WriteString(labelStyle.Render("Last reload:"))
	b.WriteString("\n")
	if m.Otel.ReloadError != nil {
		b.WriteString(errorStyle.Render("  ✗ Error: " + m.Otel.ReloadError.Error()))
	} else if !m.Otel.LastReload.IsZero() {
		reloadTime := m.Otel.LastReload.Format("15:04:05")
		b.WriteString(successStyle.Render(fmt.Sprintf("  ✓ Reloaded at %s", reloadTime)))
		if m.Otel.ReloadCount > 1 {
			b.WriteString(dimStyle.Render(fmt.Sprintf(" (%d times)", m.Otel.ReloadCount)))
		}
	} else {
		b.WriteString(dimStyle.Render("  (none yet)"))
	}
	b.WriteString("\n\n")

	// Instructions
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
	if m.Otel.WatchingConfig {
		b.WriteString(instructionStyle.Render("Save to validate & hot-reload. Invalid configs won't reload."))
	} else {
		b.WriteString(instructionStyle.Render("Run 'elasticat down && up' to apply extracted config."))
	}

	// Check if we just copied
	justCopiedPath := m.UI.StatusMessage == "Path copied to clipboard!" &&
		time.Since(m.UI.StatusTime) < 2*time.Second
	justCopiedError := m.UI.StatusMessage == "Error copied to clipboard!" &&
		time.Since(m.UI.StatusTime) < 2*time.Second

	// Build action hints
	var actionParts []string

	// Copy path hint
	if justCopiedPath {
		actionParts = append(actionParts, lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("path ✓", "y")))
	} else {
		actionParts = append(actionParts, lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("copy path", "y")))
	}

	// Copy error hint (only if there's an error)
	hasError := (!m.Otel.ValidationValid && m.Otel.ValidationStatus != "" && m.Otel.ValidationStatus != "Validating config...") || m.Otel.ReloadError != nil
	if hasError {
		if justCopiedError {
			actionParts = append(actionParts, lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("error ✓", "Y")))
		} else {
			actionParts = append(actionParts, lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("copy error", "Y")))
		}
	}

	// Dismiss hint
	actionParts = append(actionParts, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(keysHint("dismiss", "esc")))

	actions := lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(actionParts, "  "))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		b.String(),
		"",
		actions,
	)

	return modalStyle.Render(content)
}

func (m Model) renderCredsModal() string {
	// Modal dimensions
	modalWidth := min(m.UI.Width-8, 70)

	// Modal box style - use a nice blue/cyan border for credentials
	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")). // Blue border
		Padding(1, 2).
		Align(lipgloss.Left)

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Render("Open Kibana")

	var b strings.Builder

	// Label style
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true) // Yellow

	// Instruction message at top
	if m.Creds.LastKibanaURL != "" {
		b.WriteString(instructionStyle.Render("Press enter to open Kibana, then log in with the credentials below."))
		b.WriteString("\n\n")
	}

	// Credentials: elastic / password (show first since it's most important)
	b.WriteString(labelStyle.Render("Credentials:"))
	b.WriteString("\n")
	if m.esUsername != "" && m.esPassword != "" {
		b.WriteString(valueStyle.Render("  " + m.esUsername + " / " + m.esPassword))
	} else {
		b.WriteString(dimStyle.Render("  (not configured)"))
	}
	b.WriteString("\n\n")

	// If we have a lastKibanaURL, show it
	if m.Creds.LastKibanaURL != "" {
		b.WriteString(labelStyle.Render("Kibana URL:"))
		b.WriteString("\n")
		// Truncate long URLs
		displayURL := m.Creds.LastKibanaURL
		if len(displayURL) > modalWidth-6 {
			displayURL = displayURL[:modalWidth-9] + "..."
		}
		b.WriteString(valueStyle.Render("  " + displayURL))
		b.WriteString("\n\n")
	}

	// Elasticsearch URL
	b.WriteString(labelStyle.Render("Elasticsearch:"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render("  http://localhost:9200"))
	b.WriteString("\n\n")

	// OTLP endpoints
	b.WriteString(labelStyle.Render("OTLP:"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render("  localhost:4317"))
	b.WriteString(dimStyle.Render(" (gRPC)"))
	b.WriteString(valueStyle.Render(" / :4318"))
	b.WriteString(dimStyle.Render(" (HTTP)"))

	// Check if we just copied (statusMessage set within last 2 seconds)
	justCopiedURL := m.UI.StatusMessage == "URL copied to clipboard!" &&
		time.Since(m.UI.StatusTime) < 2*time.Second
	justCopiedPass := m.UI.StatusMessage == "Password copied to clipboard!" &&
		time.Since(m.UI.StatusTime) < 2*time.Second

	// Actions - include enter/y/p when we have a URL
	var actions string
	if m.Creds.LastKibanaURL != "" {
		var copyURLHint string
		if justCopiedURL {
			copyURLHint = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("url ✓", "y"))
		} else {
			copyURLHint = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("copy url", "y"))
		}
		var copyPassHint string
		if justCopiedPass {
			copyPassHint = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("pass ✓", "p"))
		} else {
			copyPassHint = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("copy pass", "p"))
		}
		actions = lipgloss.JoinHorizontal(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("open browser", "enter")),
			"  ",
			copyURLHint,
			"  ",
			copyPassHint,
			"  ",
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(keysHint("dismiss", "esc")),
			"  ",
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(keysHint("never show", "n")),
		)
	} else {
		var copyPassHint string
		if justCopiedPass {
			copyPassHint = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("pass ✓", "p"))
		} else {
			copyPassHint = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(keysHint("copy pass", "p"))
		}
		actions = lipgloss.JoinHorizontal(
			lipgloss.Left,
			copyPassHint,
			"  ",
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(keysHint("dismiss", "esc")),
			"  ",
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(keysHint("never show again", "n")),
		)
	}

	// Tip about CLI command
	tip := dimStyle.Render("Tip: Use 'elasticat creds' from terminal")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		b.String(),
		"",
		actions,
		"",
		tip,
	)

	return modalStyle.Render(content)
}
