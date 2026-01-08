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
	row1Parts = append(row1Parts, StatusKeyStyle.Render("Signal: ")+StatusValueStyle.Render(m.Filters.Signal.String()))

	// Current index
	row1Parts = append(row1Parts, StatusKeyStyle.Render("Idx: ")+StatusValueStyle.Render(m.client.GetIndex()))

	// Total logs
	row1Parts = append(row1Parts, StatusKeyStyle.Render("Total: ")+StatusValueStyle.Render(fmt.Sprintf("%d", m.Logs.Total)))

	// Filters (with visual indicator when active)
	if m.Filters.Query != "" {
		row1Parts = append(row1Parts, StatusKeyStyle.Render("Query: ")+StatusValueStyle.Render(TruncateWithEllipsis(m.Filters.Query, 20)))
	}
	if m.Filters.Level != "" {
		row1Parts = append(row1Parts, StatusKeyStyle.Render("Level: ")+StatusValueStyle.Render(m.Filters.Level))
	}
	if m.Filters.Service != "" {
		if m.Filters.NegateService {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true).Render("⛔ NOT Service: ")+StatusValueStyle.Render(m.Filters.Service))
		} else {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render("⚡ Service: ")+StatusValueStyle.Render(m.Filters.Service))
		}
	}
	if m.Filters.Resource != "" {
		if m.Filters.NegateResource {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true).Render("⛔ NOT Resource: ")+StatusValueStyle.Render(m.Filters.Resource))
		} else {
			row1Parts = append(row1Parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render("⚡ Resource: ")+StatusValueStyle.Render(m.Filters.Resource))
		}
	}

	// Loading indicator
	if m.UI.Loading {
		row1Parts = append(row1Parts, LoadingStyle.Render("loading..."))
	}

	row1 := strings.Join(row1Parts, "  │  ")

	// Row 2: Only ES status if there's an error
	var row2 string
	if m.UI.Err != nil {
		row2 = "\n" + ErrorStyle.Render("ES: err")
	}

	// Combine rows (row2 only if it has content)
	return StatusBarStyle.Width(m.UI.Width - 2).Render(row1 + row2)
}
