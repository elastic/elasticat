// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderBase renders the main view for a given mode (excluding modal overlays) and pins the help bar to the bottom.
func (m Model) renderBase(mode viewMode) string {
	var body strings.Builder

	help := m.renderHelpBar()
	helpHeight := lipgloss.Height(help)
	if helpHeight < 1 {
		helpHeight = 1
	}

	bodyHeight := m.UI.Height - helpHeight
	if bodyHeight < 0 {
		bodyHeight = 0
	}

	title := m.renderTitleHeader()
	status := m.renderStatusBar()

	// Title header (top)
	body.WriteString(title)
	body.WriteString("\n")

	// Status bar
	body.WriteString(status)
	body.WriteString("\n")

	// Remaining height for the view content (below title + status, above help)
	remainingHeight := bodyHeight - lipgloss.Height(title) - lipgloss.Height(status) - 2
	if remainingHeight < 0 {
		remainingHeight = 0
	}

	switch mode {
	case viewLogs, viewIndex, viewQuery:
		compact := m.renderCompactDetail()
		compactHeight := lipgloss.Height(compact)

		index := ""
		indexHeight := 0
		if mode == viewIndex {
			index = m.renderIndexInput()
			indexHeight = lipgloss.Height(index)
		}
		query := ""
		queryHeight := 0
		if mode == viewQuery {
			query = m.renderQueryOverlay()
			queryHeight = lipgloss.Height(query)
		}

		// Layout:
		// [log list]
		// \n
		// [compact detail]
		// (\n [index/query])?
		listHeight := remainingHeight - 1 - compactHeight - (boolToInt(mode == viewIndex || mode == viewQuery) * 1) - indexHeight - queryHeight
		if listHeight < 3 {
			listHeight = 3
		}

		body.WriteString(m.renderLogListWithHeight(listHeight))
		body.WriteString("\n")
		body.WriteString(compact)
		if mode == viewIndex {
			body.WriteString("\n")
			body.WriteString(index)
		}
		if mode == viewQuery {
			body.WriteString("\n")
			body.WriteString(query)
		}
	case viewDetail, viewDetailJSON:
		body.WriteString(m.renderDetailView())
	case viewFields:
		body.WriteString(m.renderFieldSelector())
	case viewMetricsDashboard:
		// Metrics dashboard shares the log-list height model (dashboard + compact)
		compact := m.renderMetricsCompactDetail()
		compactHeight := lipgloss.Height(compact)
		dashHeight := remainingHeight - 1 - compactHeight
		if dashHeight < 3 {
			dashHeight = 3
		}

		body.WriteString(m.renderMetricsDashboard(dashHeight))
		body.WriteString("\n")
		body.WriteString(compact)
	case viewMetricDetail:
		body.WriteString(m.renderMetricDetail())
	case viewTraceNames:
		body.WriteString(m.renderTransactionNames(remainingHeight))
	case viewPerspectiveList:
		body.WriteString(m.renderPerspectiveList(remainingHeight))
		body.WriteString("\n")
		body.WriteString(m.renderCompactDetail())
	case viewChat:
		body.WriteString(m.renderChatView(remainingHeight))
	}

	placedBody := lipgloss.Place(m.UI.Width, bodyHeight, lipgloss.Left, lipgloss.Top, body.String())
	full := lipgloss.JoinVertical(lipgloss.Left, placedBody, help)
	return AppStyle.Render(full)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// View is the main rendering entry point that routes to specific render functions.
//
// Render functions have been organized into separate files:
// - render_common.go: getContentHeight, getFullScreenHeight, calcVisibleRange
// - render_logs.go: renderLogList, renderLogListWithHeight, renderLogEntry, renderLogDetail
// - render_metrics.go: renderMetricsDashboard, renderMetricsCompactDetail, renderMetricDetail
// - render_traces.go: renderTransactionNames
// - render_perspective.go: renderPerspectiveList
// - render_detail.go: renderCompactDetail, renderDetailView, renderFieldSelector
// - render_overlay.go: renderQueryOverlay, renderErrorModal
//
// Component render functions are in:
// - header.go: renderTitleHeader
// - statusbar.go: renderStatusBar
// - helpbar.go: renderHelpBar
// - inputs.go: renderSearchInput, renderIndexInput
//
// Formatting functions are in:
// - formatting_time.go: formatRelativeTime
// - formatting_metrics.go: generateSparkline, formatMetricValue
// - formatting_charts.go: renderLargeChart
// - formatting_text.go: PadLeft, TruncateWithEllipsis, PadOrTruncate
func (m Model) View() string {
	if m.UI.Width == 0 {
		return "Loading..."
	}

	// Main content based on mode
	switch m.UI.Mode {
	case viewSearch:
		// Render previous mode as background, then overlay search bar at bottom
		base := m.renderBase(m.peekViewStack())
		return m.overlaySearchBar(base)
	case viewLogs, viewIndex, viewQuery:
		return m.renderBase(m.UI.Mode)
	case viewDetail, viewDetailJSON:
		return m.renderBase(m.UI.Mode)
	case viewFields:
		return m.renderBase(m.UI.Mode)
	case viewMetricsDashboard:
		return m.renderBase(m.UI.Mode)
	case viewMetricDetail:
		return m.renderBase(m.UI.Mode)
	case viewTraceNames:
		return m.renderBase(m.UI.Mode)
	case viewPerspectiveList:
		return m.renderBase(m.UI.Mode)
	case viewChat:
		return m.renderBase(m.UI.Mode)
	case viewErrorModal:
		// Use lipgloss.Place to properly overlay the modal in the center of the screen
		modal := m.renderErrorModal()
		overlay := lipgloss.Place(
			m.UI.Width, m.UI.Height,
			lipgloss.Center, lipgloss.Center,
			modal,
		)
		return overlay
	case viewQuitConfirm:
		// Use lipgloss.Place to properly overlay the modal in the center of the screen
		modal := m.renderQuitConfirmModal()
		overlay := lipgloss.Place(
			m.UI.Width, m.UI.Height,
			lipgloss.Center, lipgloss.Center,
			modal,
		)
		return overlay
	case viewHelp:
		// Render previous mode as background, then overlay help content
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.UI.Width, m.UI.Height,
			lipgloss.Center, lipgloss.Center,
			m.renderHelpOverlay(),
		)
		return overlayCenter(base, modal, m.UI.Width, m.UI.Height)
	case viewCredsModal:
		// Render previous mode as background, then overlay credentials modal
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.UI.Width, m.UI.Height,
			lipgloss.Center, lipgloss.Center,
			m.renderCredsModal(),
		)
		return overlayCenter(base, modal, m.UI.Width, m.UI.Height)
	case viewOtelConfigExplain:
		// Render previous mode as background, then overlay explanation modal
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.UI.Width, m.UI.Height,
			lipgloss.Center, lipgloss.Center,
			m.renderOtelConfigExplainModal(),
		)
		return overlayCenter(base, modal, m.UI.Width, m.UI.Height)
	case viewOtelConfigModal:
		// Render previous mode as background, then overlay OTel config modal
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.UI.Width, m.UI.Height,
			lipgloss.Center, lipgloss.Center,
			m.renderOtelConfigModal(),
		)
		return overlayCenter(base, modal, m.UI.Width, m.UI.Height)
	}
	return m.renderBase(m.UI.Mode)
}

// overlayCenter overlays 'top' onto 'base' by replacing centered lines, preserving background elsewhere.
func overlayCenter(base, top string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	topLines := strings.Split(top, "\n")

	// Ensure base has at least height lines
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	startY := max(0, (height-len(topLines))/2)

	for i, line := range topLines {
		y := startY + i
		if y >= len(baseLines) {
			break
		}
		baseLines[y] = line
	}

	return strings.Join(baseLines, "\n")
}

// overlaySearchBar overlays the search input bar at the bottom of the base view.
func (m Model) overlaySearchBar(base string) string {
	baseLines := strings.Split(base, "\n")

	// Render the search input
	search := m.renderSearchInput()

	// Find the position to insert the search bar (above the help bar, which is the last line)
	// The help bar is always at the bottom, so we replace the line just above it
	if len(baseLines) < 2 {
		return base + "\n" + search
	}

	// Replace the second-to-last line with the search bar
	// This positions it just above the help bar
	insertPos := len(baseLines) - 2
	if insertPos < 0 {
		insertPos = 0
	}

	baseLines[insertPos] = search

	return strings.Join(baseLines, "\n")
}
