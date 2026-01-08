// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderBase renders the main view for a given mode (excluding modal overlays) and appends the help bar.
func (m Model) renderBase(mode viewMode) string {
	var b strings.Builder

	// Calculate heights using helper method
	logListHeight := m.getContentHeight(true)

	// Title header (top)
	b.WriteString(m.renderTitleHeader())
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	switch mode {
	case viewLogs, viewSearch, viewIndex, viewQuery:
		b.WriteString(m.renderLogListWithHeight(logListHeight))
		b.WriteString("\n")
		b.WriteString(m.renderCompactDetail())
		if mode == viewSearch {
			b.WriteString("\n")
			b.WriteString(m.renderSearchInput())
		}
		if mode == viewIndex {
			b.WriteString("\n")
			b.WriteString(m.renderIndexInput())
		}
		if mode == viewQuery {
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
	case viewChat:
		b.WriteString(m.renderChatView(logListHeight))
	}

	// Help bar (bottom)
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return AppStyle.Render(b.String())
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
	if m.width == 0 {
		return "Loading..."
	}

	// Main content based on mode
	switch m.mode {
	case viewLogs, viewSearch, viewIndex, viewQuery:
		return m.renderBase(m.mode)
	case viewDetail, viewDetailJSON:
		return m.renderBase(m.mode)
	case viewFields:
		return m.renderBase(m.mode)
	case viewMetricsDashboard:
		return m.renderBase(m.mode)
	case viewMetricDetail:
		return m.renderBase(m.mode)
	case viewTraceNames:
		return m.renderBase(m.mode)
	case viewPerspectiveList:
		return m.renderBase(m.mode)
	case viewChat:
		return m.renderBase(m.mode)
	case viewErrorModal:
		// Use lipgloss.Place to properly overlay the modal in the center of the screen
		modal := m.renderErrorModal()
		overlay := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modal,
		)
		return overlay
	case viewQuitConfirm:
		// Use lipgloss.Place to properly overlay the modal in the center of the screen
		modal := m.renderQuitConfirmModal()
		overlay := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modal,
		)
		return overlay
	case viewHelp:
		// Render previous mode as background, then overlay help content
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.renderHelpOverlay(),
		)
		return overlayCenter(base, modal, m.width, m.height)
	case viewCredsModal:
		// Render previous mode as background, then overlay credentials modal
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.renderCredsModal(),
		)
		return overlayCenter(base, modal, m.width, m.height)
	case viewOtelConfigExplain:
		// Render previous mode as background, then overlay explanation modal
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.renderOtelConfigExplainModal(),
		)
		return overlayCenter(base, modal, m.width, m.height)
	case viewOtelConfigModal:
		// Render previous mode as background, then overlay OTel config modal
		base := m.renderBase(m.peekViewStack())
		modal := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.renderOtelConfigModal(),
		)
		return overlayCenter(base, modal, m.width, m.height)
	}
	return m.renderBase(m.mode)
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
