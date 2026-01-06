// Copyright 2026 Elasticsearch B.V.
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

	index := m.lastQueryIndex

	// Header showing format and status
	var formatLabel string
	if m.queryFormat == formatKibana {
		formatLabel = "Kibana Dev Tools"
	} else {
		formatLabel = "curl"
	}

	header := fmt.Sprintf("Query (%s)", formatLabel)
	b.WriteString(QueryHeaderStyle.Render(header))

	// Show status message if recent (within 2 seconds)
	if m.statusMessage != "" && time.Since(m.statusTime) < 2*time.Second {
		b.WriteString("  ")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render(m.statusMessage))
	}
	b.WriteString("\n\n")

	if m.queryFormat == formatKibana {
		// Kibana Dev Tools format
		b.WriteString(QueryMethodStyle.Render("GET "))
		b.WriteString(QueryPathStyle.Render(index + "/_search"))
		b.WriteString("\n")
		b.WriteString(QueryBodyStyle.Render(m.lastQueryJSON))
	} else {
		// curl format
		b.WriteString(QueryBodyStyle.Render("curl -X GET 'http://localhost:9200/" + index + "/_search' \\\n"))
		b.WriteString(QueryBodyStyle.Render("  -H 'Content-Type: application/json' \\\n"))
		b.WriteString(QueryBodyStyle.Render("  -d '"))
		// Compact JSON for curl
		var compact bytes.Buffer
		if err := json.Compact(&compact, []byte(m.lastQueryJSON)); err == nil {
			b.WriteString(QueryBodyStyle.Render(compact.String()))
		} else {
			b.WriteString(QueryBodyStyle.Render(m.lastQueryJSON))
		}
		b.WriteString(QueryBodyStyle.Render("'"))
	}

	// Calculate height - use full screen height since this replaces the log list
	height := m.getFullScreenHeight()
	if height < 10 {
		height = 10
	}

	return QueryOverlayStyle.Width(m.width - 8).Height(height).Render(b.String())
}

func (m Model) renderErrorModal() string {
	// Modal dimensions
	modalWidth := min(m.width-8, 80)

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
	justCopied := m.statusMessage == "Error copied to clipboard!" &&
		time.Since(m.statusTime) < 2*time.Second

	// Scroll indicator
	scrollInfo := ""
	if m.errorViewport.TotalLineCount() > m.errorViewport.Height {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf(" (scroll: %d%%) ", int(m.errorViewport.ScrollPercent()*100)))
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
		Render(m.errorViewport.View())

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
	modalWidth := min(m.width-8, 80)

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
