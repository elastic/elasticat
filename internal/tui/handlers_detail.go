// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/elastic/elasticat/internal/es"
)

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	switch action {
	case ActionBack, ActionQuit:
		// Return to previous view via stack
		m.popView()
		m.statusMessage = ""
		return m, nil
	case ActionPrevItem:
		// Navigate to previous entry
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.updateDetailContent()
			// Fetch spans for traces signal type
			if m.signalType == signalTraces {
				return m, m.maybeFetchSpansForSelection()
			}
		}
		return m, nil
	case ActionNextItem:
		// Navigate to next entry
		if m.selectedIndex < len(m.logs)-1 {
			m.selectedIndex++
			m.updateDetailContent()
			// Fetch spans for traces signal type
			if m.signalType == signalTraces {
				return m, m.maybeFetchSpansForSelection()
			}
		}
		return m, nil
	case ActionSelect:
		// Toggle between detail and JSON view (same stack level)
		if m.mode == viewDetail {
			m.mode = viewDetailJSON
			if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				m.setViewportContent(es.PrettyJSON(m.logs[m.selectedIndex].RawJSON))
				m.viewport.GotoTop()
			}
		} else {
			m.mode = viewDetail
			if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				m.setViewportContent(m.renderLogDetail(m.logs[m.selectedIndex]))
				m.viewport.GotoTop()
			}
		}
		return m, nil
	case ActionCopy:
		// Copy raw JSON to clipboard
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			m.copyToClipboard(es.PrettyJSON(m.logs[m.selectedIndex].RawJSON), "Copied JSON to clipboard!")
		}
		return m, nil
	case ActionKibana:
		// Prepare Kibana URL for trace and show creds modal (only for traces)
		if m.signalType == signalTraces && len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			log := m.logs[m.selectedIndex]
			if log.TraceID != "" && m.prepareTraceKibanaURL(log.TraceID) {
				m.showCredsModal()
			}
		}
		return m, nil
	}

	// Handle JSON toggle (J) and spans (S)
	switch action {
	case ActionJSON:
		// Toggle JSON view on/off
		if m.mode == viewDetailJSON {
			// If we came from metric detail (via J key), pop back to it
			if m.peekViewStack() == viewMetricDetail {
				m.popView()
			} else {
				// Otherwise toggle to formatted detail view
				m.mode = viewDetail
				if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
					m.setViewportContent(m.renderLogDetail(m.logs[m.selectedIndex]))
					m.viewport.GotoTop()
				}
			}
		} else {
			m.mode = viewDetailJSON
			if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				m.setViewportContent(es.PrettyJSON(m.logs[m.selectedIndex].RawJSON))
				m.viewport.GotoTop()
			}
		}
		return m, nil
	case ActionSpans:
		// Show spans for this trace (only for traces)
		if m.signalType == signalTraces && len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			log := m.logs[m.selectedIndex]
			if log.TraceID != "" {
				m.selectedTraceID = log.TraceID
				m.traceViewLevel = traceViewSpans
				m.mode = viewLogs
				m.selectedIndex = 0
				m.loading = true
				return m, m.fetchLogs()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// updateDetailContent refreshes the detail view content for the current selection
func (m *Model) updateDetailContent() {
	if len(m.logs) == 0 || m.selectedIndex >= len(m.logs) {
		return
	}
	if m.mode == viewDetailJSON {
		m.setViewportContent(es.PrettyJSON(m.logs[m.selectedIndex].RawJSON))
	} else {
		m.setViewportContent(m.renderLogDetail(m.logs[m.selectedIndex]))
	}
	m.viewport.GotoTop()
}
