// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	if m.width == 0 {
		return "Loading..."
	}

	// Calculate heights using helper method
	logListHeight := m.getContentHeight(true)

	var b strings.Builder

	// Title header (top)
	b.WriteString(m.renderTitleHeader())
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// Main content based on mode
	switch m.mode {
	case viewLogs, viewSearch, viewIndex, viewQuery:
		// Log list (fills available space)
		b.WriteString(m.renderLogListWithHeight(logListHeight))
		b.WriteString("\n")
		// Compact detail (anchored to bottom, fixed height)
		b.WriteString(m.renderCompactDetail())
		if m.mode == viewSearch {
			b.WriteString("\n")
			b.WriteString(m.renderSearchInput())
		}
		if m.mode == viewIndex {
			b.WriteString("\n")
			b.WriteString(m.renderIndexInput())
		}
		if m.mode == viewQuery {
			b.WriteString("\n")
			b.WriteString(m.renderQueryOverlay())
		}
	case viewDetail, viewDetailJSON:
		b.WriteString(m.renderDetailView())
	case viewFields:
		b.WriteString(m.renderFieldSelector())
	case viewMetricsDashboard:
		b.WriteString(m.renderMetricsDashboard(logListHeight))
		b.WriteString("\n")
		b.WriteString(m.renderMetricsCompactDetail())
	case viewMetricDetail:
		b.WriteString(m.renderMetricDetail())
	case viewTraceNames:
		b.WriteString(m.renderTransactionNames(logListHeight))
	case viewPerspectiveList:
		b.WriteString(m.renderPerspectiveList(logListHeight))
		b.WriteString("\n")
		b.WriteString(m.renderCompactDetail())
	case viewErrorModal:
		// Use lipgloss.Place to properly overlay the modal in the center of the screen
		modal := m.renderErrorModal()
		overlay := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modal,
		)
		return overlay
	}

	// Help bar (bottom)
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return AppStyle.Render(b.String())
}
