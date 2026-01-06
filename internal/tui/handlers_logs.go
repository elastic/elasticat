// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// For traces, go back up the hierarchy
		if m.signalType == signalTraces {
			switch m.traceViewLevel {
			case traceViewSpans:
				// Go back to transactions list
				m.traceViewLevel = traceViewTransactions
				m.selectedTraceID = ""
				m.selectedIndex = 0
				m.loading = true
				return m, m.fetchLogs()
			case traceViewTransactions:
				// Go back to transaction names
				m.traceViewLevel = traceViewNames
				m.selectedTxName = ""
				m.mode = viewTraceNames
				m.tracesLoading = true
				return m, m.fetchTransactionNames()
			}
		}
	case "up", "k":
		if m.moveSelection(-1) {
			return m, m.maybeFetchSpansForSelection()
		}
	case "down", "j":
		if m.moveSelection(1) {
			return m, m.maybeFetchSpansForSelection()
		}
	case "home", "g":
		if m.setSelectedIndex(0) {
			return m, m.maybeFetchSpansForSelection()
		}
	case "end", "G":
		if m.setSelectedIndex(len(m.logs) - 1) {
			return m, m.maybeFetchSpansForSelection()
		}
	case "pgup":
		if m.moveSelection(-10) {
			return m, m.maybeFetchSpansForSelection()
		}
	case "pgdown":
		if m.moveSelection(10) {
			return m, m.maybeFetchSpansForSelection()
		}
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "enter":
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			m.mode = viewDetail
			m.setViewportContent(m.renderLogDetail(m.logs[m.selectedIndex]))
			m.viewport.GotoTop()
		}
	case "r":
		m.loading = true
		return m, m.fetchLogs()
	case "a":
		m.autoRefresh = !m.autoRefresh
	case "1":
		m.levelFilter = "ERROR"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "2":
		m.levelFilter = "WARN"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "3":
		m.levelFilter = "INFO"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "4":
		m.levelFilter = "DEBUG"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "0":
		m.levelFilter = ""
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "i":
		m.mode = viewIndex
		m.indexInput.SetValue(m.client.GetIndex())
		m.indexInput.Focus()
		return m, textinput.Blink
	case "t":
		switch m.timeDisplayMode {
		case timeDisplayClock:
			m.timeDisplayMode = timeDisplayRelative
		case timeDisplayRelative:
			m.timeDisplayMode = timeDisplayFull
		default:
			m.timeDisplayMode = timeDisplayClock
		}
	case "Q":
		m.mode = viewQuery
		m.queryFormat = formatKibana
	case "f":
		m.mode = viewFields
		m.fieldsCursor = 0
		m.fieldsSearch = ""
		m.fieldsSearchMode = false
		m.fieldsLoading = true
		return m, m.fetchFieldCaps()
	case "s":
		m.sortAscending = !m.sortAscending
		m.loading = true
		return m, m.fetchLogs()
	case "l":
		m.cycleLookback()
		m.loading = true
		return m, m.fetchLogs()
	case "m":
		return m, m.cycleSignalType()

	case "p":
		return m, m.cyclePerspective()

	case "K":
		// Open current query in Kibana Discover
		m.openInKibana()
		return m, nil

	case "d":
		// Toggle between document view and aggregated view for metrics
		if m.signalType == signalMetrics {
			if m.metricsViewMode == metricsViewAggregated {
				m.metricsViewMode = metricsViewDocuments
				m.mode = viewLogs
				m.loading = true
				return m, m.fetchLogs()
			} else {
				m.metricsViewMode = metricsViewAggregated
				m.mode = viewMetricsDashboard
				m.metricsLoading = true
				return m, m.fetchAggregatedMetrics()
			}
		}
	}

	return m, nil
}

func (m *Model) setSelectedIndex(newIdx int) bool {
	if len(m.logs) == 0 {
		m.selectedIndex = 0
		return false
	}

	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(m.logs) {
		newIdx = len(m.logs) - 1
	}
	if newIdx == m.selectedIndex {
		return false
	}

	m.selectedIndex = newIdx
	m.userHasScrolled = true
	return true
}

func (m *Model) moveSelection(delta int) bool {
	return m.setSelectedIndex(m.selectedIndex + delta)
}

// maybeFetchSpansForSelection triggers a spans fetch for traces, avoiding duplicate requests.
func (m *Model) maybeFetchSpansForSelection() tea.Cmd {
	if m.signalType != signalTraces {
		return nil
	}
	if len(m.logs) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.logs) {
		m.lastFetchedTraceID = ""
		return nil
	}

	traceID := m.logs[m.selectedIndex].TraceID
	if traceID == "" {
		m.lastFetchedTraceID = ""
		return nil
	}

	if !m.needsSpanFetch(traceID) {
		return nil
	}

	m.spansLoading = true
	m.lastFetchedTraceID = traceID
	m.spans = nil
	return m.fetchSpans(traceID)
}

// needsSpanFetch encapsulates the dedupe logic for span fetches.
func (m Model) needsSpanFetch(traceID string) bool {
	if traceID == "" {
		return false
	}
	if traceID == m.lastFetchedTraceID && (m.spansLoading || len(m.spans) > 0) {
		return false
	}
	return true
}
