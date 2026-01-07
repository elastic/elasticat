// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar renders the status bar showing current state and filters
func (m Model) renderStatusBar() string {
	// Row 1: Signal, Index, Total, Filters, Loading
	var row1Parts []string

	// Signal type
	row1Parts = append(row1Parts, StatusKeyStyle.Render("Signal: ")+StatusValueStyle.Render(m.signalType.String()))

	// Current index
	row1Parts = append(row1Parts, StatusKeyStyle.Render("Idx: ")+StatusValueStyle.Render(m.client.GetIndex()))

	// Total logs
	row1Parts = append(row1Parts, StatusKeyStyle.Render("Total: ")+StatusValueStyle.Render(fmt.Sprintf("%d", m.total)))

	// Filters (with visual indicator when active)
	if m.searchQuery != "" {
		row1Parts = append(row1Parts, StatusKeyStyle.Render("Query: ")+StatusValueStyle.Render(TruncateWithEllipsis(m.searchQuery, 20)))
	}
	if m.levelFilter != "" {
		row1Parts = append(row1Parts, StatusKeyStyle.Render("Level: ")+StatusValueStyle.Render(m.levelFilter))
	}
	if m.filterService != "" {
		if m.negateService {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true).Render("⛔ NOT Service: ")+StatusValueStyle.Render(m.filterService))
		} else {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render("⚡ Service: ")+StatusValueStyle.Render(m.filterService))
		}
	}
	if m.filterResource != "" {
		if m.negateResource {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true).Render("⛔ NOT Resource: ")+StatusValueStyle.Render(m.filterResource))
		} else {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render("⚡ Resource: ")+StatusValueStyle.Render(m.filterResource))
		}
	}

	// Loading indicator
	if m.loading {
		row1Parts = append(row1Parts, LoadingStyle.Render("loading..."))
	}

	row1 := strings.Join(row1Parts, "  │  ")

	// Row 2: Only ES status if there's an error
	var row2 string
	if m.err != nil {
		row2 = "\n" + ErrorStyle.Render("ES: err")
	}

	// Combine rows (row2 only if it has content)
	return StatusBarStyle.Width(m.width - 2).Render(row1 + row2)
}
