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
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.userHasScrolled = true // User manually scrolled
			// Fetch spans for traces when selection changes
			if m.signalType == signalTraces && len(m.logs) > 0 {
				traceID := m.logs[m.selectedIndex].TraceID
				if traceID != "" {
					m.spansLoading = true
					return m, m.fetchSpans(traceID)
				}
			}
		}
	case "down", "j":
		if m.selectedIndex < len(m.logs)-1 {
			m.selectedIndex++
			m.userHasScrolled = true // User manually scrolled
			// Fetch spans for traces when selection changes
			if m.signalType == signalTraces && len(m.logs) > 0 {
				traceID := m.logs[m.selectedIndex].TraceID
				if traceID != "" {
					m.spansLoading = true
					return m, m.fetchSpans(traceID)
				}
			}
		}
	case "home", "g":
		m.selectedIndex = 0
		m.userHasScrolled = true // User manually scrolled
	case "end", "G":
		if len(m.logs) > 0 {
			m.selectedIndex = len(m.logs) - 1
		}
		m.userHasScrolled = true // User manually scrolled
	case "pgup":
		m.selectedIndex -= 10
		if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
		m.userHasScrolled = true // User manually scrolled
	case "pgdown":
		m.selectedIndex += 10
		if m.selectedIndex >= len(m.logs) {
			m.selectedIndex = len(m.logs) - 1
		}
		m.userHasScrolled = true // User manually scrolled
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "enter":
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			m.mode = viewDetail
			m.viewport.SetContent(m.renderLogDetail(m.logs[m.selectedIndex]))
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
		m.relativeTime = !m.relativeTime
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
