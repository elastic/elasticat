// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	// Handle common actions first (signal cycle, lookback, perspective, kibana)
	if newM, cmd, handled := m.handleCommonAction(action); handled {
		return newM, cmd
	}

	// Handle navigation actions
	switch action {
	case ActionBack:
		// For traces, go back up the hierarchy
		if m.Filters.Signal == signalTraces {
			switch m.Traces.ViewLevel {
			case traceViewSpans:
				// Go back to transactions list
				m.Traces.ViewLevel = traceViewTransactions
				m.Traces.SelectedTraceID = ""
				m.Logs.SelectedIndex = 0
				m.UI.Loading = true
				return m, m.fetchLogs()
			case traceViewTransactions:
				// Go back to transaction names
				m.Traces.ViewLevel = traceViewNames
				m.Traces.SelectedTxName = ""
				m.UI.Mode = viewTraceNames
				m.Traces.Loading = true
				return m, m.fetchTransactionNames()
			}
		}
		// For metrics in documents view, go back to dashboard
		if m.Filters.Signal == signalMetrics && m.Metrics.ViewMode == metricsViewDocuments {
			m.Metrics.ViewMode = metricsViewAggregated
			m.UI.Mode = viewMetricsDashboard
			m.Metrics.Loading = true
			return m, m.fetchAggregatedMetrics()
		}
		// Logs is a base view - esc does nothing (user can press 'q' to quit)
		return m, nil
	case ActionScrollUp:
		if m.moveSelection(-1) {
			return m, m.maybeFetchSpansForSelection()
		}
	case ActionScrollDown:
		if m.moveSelection(1) {
			return m, m.maybeFetchSpansForSelection()
		}
	case ActionGoTop:
		if m.setSelectedIndex(0) {
			return m, m.maybeFetchSpansForSelection()
		}
	case ActionGoBottom:
		if m.setSelectedIndex(len(m.Logs.Entries) - 1) {
			return m, m.maybeFetchSpansForSelection()
		}
	case ActionPageUp:
		if m.moveSelection(-10) {
			return m, m.maybeFetchSpansForSelection()
		}
	case ActionPageDown:
		if m.moveSelection(10) {
			return m, m.maybeFetchSpansForSelection()
		}
	case ActionSearch:
		m.pushView(viewSearch)
		m.Components.SearchInput.Focus()
		return m, textinput.Blink
	case ActionSelect:
		if len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
			m.pushView(viewDetail)
			m.setViewportContent(m.renderLogDetail(m.Logs.Entries[m.Logs.SelectedIndex]))
			m.Components.Viewport.GotoTop()
		}
	case ActionRefresh:
		m.UI.Loading = true
		return m, m.fetchLogs()
	case ActionAutoRefresh:
		m.UI.AutoRefresh = !m.UI.AutoRefresh
	case ActionQuery:
		m.pushView(viewQuery)
		m.Query.Format = formatKibana
	case ActionFields:
		m.pushView(viewFields)
		m.Fields.Cursor = 0
		m.Fields.Search = ""
		m.Fields.SearchMode = false
		m.Fields.Loading = true
		return m, m.fetchFieldCaps()
	case ActionSort:
		m.UI.SortAscending = !m.UI.SortAscending
		m.UI.Loading = true
		return m, m.fetchLogs()
		// NOTE: ActionCycleLookback, ActionCycleSignal, ActionPerspective, ActionKibana
		// are now handled by handleCommonAction() above
	}

	// Handle view-specific keys that aren't common actions
	switch key {
	case "1":
		m.Filters.Level = "ERROR"
		m.Logs.UserHasScrolled = false // Reset for tail -f behavior
		m.UI.Loading = true
		return m, m.fetchLogs()
	case "2":
		m.Filters.Level = "WARN"
		m.Logs.UserHasScrolled = false // Reset for tail -f behavior
		m.UI.Loading = true
		return m, m.fetchLogs()
	case "3":
		m.Filters.Level = "INFO"
		m.Logs.UserHasScrolled = false // Reset for tail -f behavior
		m.UI.Loading = true
		return m, m.fetchLogs()
	case "4":
		m.Filters.Level = "DEBUG"
		m.Logs.UserHasScrolled = false // Reset for tail -f behavior
		m.UI.Loading = true
		return m, m.fetchLogs()
	case "0":
		m.Filters.Level = ""
		m.Logs.UserHasScrolled = false // Reset for tail -f behavior
		m.UI.Loading = true
		return m, m.fetchLogs()
	case "i":
		m.pushView(viewIndex)
		m.Components.IndexInput.SetValue(m.client.GetIndex())
		m.Components.IndexInput.Focus()
		return m, textinput.Blink
	case "t":
		switch m.UI.TimeDisplayMode {
		case timeDisplayClock:
			m.UI.TimeDisplayMode = timeDisplayRelative
		case timeDisplayRelative:
			m.UI.TimeDisplayMode = timeDisplayFull
		default:
			m.UI.TimeDisplayMode = timeDisplayClock
		}
	case "d":
		// Toggle between document view and aggregated view for metrics
		if m.Filters.Signal == signalMetrics {
			if m.Metrics.ViewMode == metricsViewAggregated {
				m.Metrics.ViewMode = metricsViewDocuments
				m.UI.Mode = viewLogs
				m.UI.Loading = true
				return m, m.fetchLogs()
			} else {
				m.Metrics.ViewMode = metricsViewAggregated
				m.UI.Mode = viewMetricsDashboard
				m.Metrics.Loading = true
				return m, m.fetchAggregatedMetrics()
			}
		}
	}

	return m, nil
}

func (m *Model) setSelectedIndex(newIdx int) bool {
	if len(m.Logs.Entries) == 0 {
		m.Logs.SelectedIndex = 0
		return false
	}

	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(m.Logs.Entries) {
		newIdx = len(m.Logs.Entries) - 1
	}
	if newIdx == m.Logs.SelectedIndex {
		return false
	}

	m.Logs.SelectedIndex = newIdx
	m.Logs.UserHasScrolled = true
	return true
}

func (m *Model) moveSelection(delta int) bool {
	return m.setSelectedIndex(m.Logs.SelectedIndex + delta)
}

// maybeFetchSpansForSelection triggers a spans fetch for traces, avoiding duplicate requests.
func (m *Model) maybeFetchSpansForSelection() tea.Cmd {
	if m.Filters.Signal != signalTraces {
		return nil
	}
	if len(m.Logs.Entries) == 0 || m.Logs.SelectedIndex < 0 || m.Logs.SelectedIndex >= len(m.Logs.Entries) {
		m.Traces.LastFetchedTraceID = ""
		return nil
	}

	traceID := m.Logs.Entries[m.Logs.SelectedIndex].TraceID
	if traceID == "" {
		m.Traces.LastFetchedTraceID = ""
		return nil
	}

	if !m.needsSpanFetch(traceID) {
		return nil
	}

	m.Traces.SpansLoading = true
	m.Traces.LastFetchedTraceID = traceID
	m.Traces.Spans = nil
	return m.fetchSpans(traceID)
}

// needsSpanFetch encapsulates the dedupe logic for span fetches.
func (m Model) needsSpanFetch(traceID string) bool {
	if traceID == "" {
		return false
	}
	if traceID == m.Traces.LastFetchedTraceID && (m.Traces.SpansLoading || len(m.Traces.Spans) > 0) {
		return false
	}
	return true
}
