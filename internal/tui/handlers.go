// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleKey routes key events to mode-specific handlers
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		if m.mode == viewLogs {
			return m, tea.Quit
		}
		// Exit current mode
		m.mode = viewLogs
		m.searchInput.Blur()
		return m, nil

	case "esc":
		// Close error modal and return to previous view
		if m.mode == viewErrorModal {
			m.popView()
			m.err = nil // Clear error
			return m, nil
		}
		// Let metric and trace views handle their own escape
		if m.mode == viewMetricDetail || m.mode == viewMetricsDashboard || m.mode == viewTraceNames {
			break // Fall through to mode-specific handler
		}
		if m.mode != viewLogs {
			m.mode = viewLogs
			m.searchInput.Blur()
			return m, nil
		}
	case "h":
		// Global help only when enabled and not in text-input modes
		if m.HelpEnabled() && !m.isTextInputActive() {
			m.pushView(viewHelp)
			m.renderHelpOverlay()
			return m, nil
		}
	}

	// Mode-specific keys
	switch m.mode {
	case viewLogs:
		return m.handleLogsKey(msg)
	case viewSearch:
		return m.handleSearchKey(msg)
	case viewDetail, viewDetailJSON:
		return m.handleDetailKey(msg)
	case viewIndex:
		return m.handleIndexKey(msg)
	case viewQuery:
		return m.handleQueryKey(msg)
	case viewFields:
		return m.handleFieldsKey(msg)
	case viewMetricsDashboard:
		return m.handleMetricsDashboardKey(msg)
	case viewMetricDetail:
		return m.handleMetricDetailKey(msg)
	case viewTraceNames:
		return m.handleTraceNamesKey(msg)
	case viewPerspectiveList:
		return m.handlePerspectiveListKey(msg)
	case viewErrorModal:
		return m.handleErrorModalKey(msg)
	case viewHelp:
		return m.handleHelpKey(msg)
	}

	return m, nil
}

// isTextInputActive returns true when a text input is active, disabling global hotkeys like h.
func (m Model) isTextInputActive() bool {
	if m.mode == viewSearch || m.mode == viewIndex || m.mode == viewQuery {
		return true
	}
	// Fields search submode
	if m.mode == viewFields && m.fieldsSearchMode {
		return true
	}
	return false
}

// handleMouse handles mouse events across all views
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel scrolling
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		switch m.mode {
		case viewLogs:
			// Scroll up in log list (2 items at a time for speed)
			if m.selectedIndex > 0 {
				m.selectedIndex -= 2
				if m.selectedIndex < 0 {
					m.selectedIndex = 0
				}
			}
		case viewDetail, viewDetailJSON:
			// Scroll up in detail viewport
			m.viewport.ScrollUp(3)
		case viewFields:
			// Scroll up in field selector
			if m.fieldsCursor > 0 {
				m.fieldsCursor -= 2
				if m.fieldsCursor < 0 {
					m.fieldsCursor = 0
				}
			}
		case viewMetricsDashboard:
			// Scroll up in metrics dashboard
			if m.metricsCursor > 0 {
				m.metricsCursor -= 2
				if m.metricsCursor < 0 {
					m.metricsCursor = 0
				}
			}
		case viewTraceNames:
			// Scroll up in trace names list
			if m.traceNamesCursor > 0 {
				m.traceNamesCursor -= 2
				if m.traceNamesCursor < 0 {
					m.traceNamesCursor = 0
				}
			}
		case viewPerspectiveList:
			// Scroll up in perspective list
			if m.perspectiveCursor > 0 {
				m.perspectiveCursor -= 2
				if m.perspectiveCursor < 0 {
					m.perspectiveCursor = 0
				}
			}
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		switch m.mode {
		case viewLogs:
			// Scroll down in log list (2 items at a time for speed)
			if m.selectedIndex < len(m.logs)-1 {
				m.selectedIndex += 2
				if m.selectedIndex >= len(m.logs) {
					m.selectedIndex = len(m.logs) - 1
				}
			}
		case viewDetail, viewDetailJSON:
			// Scroll down in detail viewport
			m.viewport.ScrollDown(3)
		case viewFields:
			// Scroll down in field selector
			sortedFields := m.getSortedFieldList()
			if m.fieldsCursor < len(sortedFields)-1 {
				m.fieldsCursor += 2
				if m.fieldsCursor >= len(sortedFields) {
					m.fieldsCursor = len(sortedFields) - 1
				}
			}
		case viewMetricsDashboard:
			// Scroll down in metrics dashboard
			if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics)-1 {
				m.metricsCursor += 2
				if m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
					m.metricsCursor = len(m.aggregatedMetrics.Metrics) - 1
				}
			}
		case viewTraceNames:
			// Scroll down in trace names list
			if m.traceNamesCursor < len(m.transactionNames)-1 {
				m.traceNamesCursor += 2
				if m.traceNamesCursor >= len(m.transactionNames) {
					m.traceNamesCursor = len(m.transactionNames) - 1
				}
			}
		case viewPerspectiveList:
			// Scroll down in perspective list
			if m.perspectiveCursor < len(m.perspectiveItems)-1 {
				m.perspectiveCursor += 2
				if m.perspectiveCursor >= len(m.perspectiveItems) {
					m.perspectiveCursor = len(m.perspectiveItems) - 1
				}
			}
		}
		return m, nil
	}

	// Handle left clicks
	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionRelease {
		return m, nil
	}

	// Only handle clicks in log list mode on the first row (status bar)
	if m.mode == viewLogs && msg.Y == 0 {
		// Calculate the approximate position of the "Sort:" label in the status bar
		// The status bar contains: Signal, ES, Idx, Total, [Query], [Level], [Service], Sort, Auto
		sortStart, sortEnd := m.getSortLabelPosition()
		if msg.X >= sortStart && msg.X <= sortEnd {
			// Toggle sort order
			m.sortAscending = !m.sortAscending
			m.loading = true
			return m, m.fetchLogs()
		}
	}

	return m, nil
}

// getSortLabelPosition returns the approximate start and end X positions of the "Sort:" label
func (m Model) getSortLabelPosition() (start, end int) {
	// Build the status bar parts to calculate position
	// Note: This mirrors the logic in renderStatusBar but just calculates lengths
	pos := 1 // Start after padding

	// Signal: <type>
	pos += len("Signal: ") + len(m.signalType.String()) + 5 // + separator

	// ES: ok/err
	if m.err != nil {
		pos += len("ES: err") + 5
	} else {
		pos += len("ES: ok") + 5
	}

	// Idx: <index>
	pos += len("Idx: ") + len(m.client.GetIndex()) + 5 // +5 for separator

	// Total: <count>
	pos += len("Total: ") + len(fmt.Sprintf("%d", m.total)) + 5

	// Optional Query filter
	if m.searchQuery != "" {
		displayed := TruncateWithEllipsis(m.searchQuery, 20)
		pos += len("Query: ") + len(displayed) + 5
	}

	// Optional Level filter
	if m.levelFilter != "" {
		pos += len("Level: ") + len(m.levelFilter) + 5
	}

	// Optional Service filter
	if m.filterService != "" {
		pos += len("Service: ") + len(m.filterService) + 5
	}

	// Lookback
	pos += len("Lookback: ") + len(m.lookback.String()) + 5

	// Now we're at "Sort: "
	start = pos
	sortText := "newest→"
	if m.sortAscending {
		sortText = "oldest→"
	}
	end = start + len("Sort: ") + len(sortText)

	return start, end
}

// Layout constants
const (
	statusBarHeight     = 1 // Usually one row (two when ES error)
	helpBarHeight       = 1
	compactDetailHeight = 5 // 3 lines of content + 2 for border
	layoutPadding       = 2 // Top/bottom padding from AppStyle
)
