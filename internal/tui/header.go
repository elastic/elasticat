// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderTitleHeader renders the top header with title and operational info
func (m Model) renderTitleHeader() string {
	title := "\\ =ↀ_ↀ= 𝓒𝓪𝓽𝓼𝓔𝔂𝓮 =ↀ_ↀ= /"

	// Add perspective indicator when in perspective view
	if m.UI.Mode == viewPerspectiveList {
		title = "\\ =ↀ_ↀ= 𝓒𝓪𝓽𝓼𝓔𝔂𝓮 [" + m.Perspective.Current.String() + "] =ↀ_ↀ= /"
	}

	// Build operational info for right side
	var infoParts []string
	infoParts = append(infoParts, "Lookback: "+m.Filters.Lookback.String())

	if m.UI.SortAscending {
		infoParts = append(infoParts, "Sort: oldest→")
	} else {
		infoParts = append(infoParts, "Sort: newest→")
	}

	if m.UI.AutoRefresh {
		infoParts = append(infoParts, "Auto: ON")
	} else {
		infoParts = append(infoParts, "Auto: OFF")
	}

	// Make entire operational info white
	rightInfo := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Render("[ " + strings.Join(infoParts, " │ ") + " ]")

	// Calculate how many characters we need for the line to fill the width
	// Account for padding in the style (2 chars)
	// Use lipgloss.Width to get actual rendered width (ignoring ANSI codes)
	availableWidth := m.UI.Width - 2
	titleLen := lipgloss.Width(title)
	rightInfoLen := lipgloss.Width(rightInfo)

	// Check if everything fits
	if titleLen+rightInfoLen >= availableWidth {
		// Not enough space, just show title with line
		lineChars := availableWidth - titleLen
		if lineChars < 0 {
			lineChars = 0
		}
		line := strings.Repeat("═", lineChars)
		return TitleHeaderStyle.Width(m.UI.Width).Render(title + line)
	}

	// Fill the middle with box drawing characters
	lineChars := availableWidth - titleLen - rightInfoLen
	line := strings.Repeat("═", lineChars)

	fullHeader := title + line + rightInfo
	return TitleHeaderStyle.Width(m.UI.Width).Render(fullHeader)
}
