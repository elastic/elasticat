// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/elastic/elasticat/internal/es"
)

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = viewLogs
		m.statusMessage = ""
		return m, nil
	case "left", "h":
		// Navigate to previous entry
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.updateDetailContent()
		}
		return m, nil
	case "right", "l":
		// Navigate to next entry
		if m.selectedIndex < len(m.logs)-1 {
			m.selectedIndex++
			m.updateDetailContent()
		}
		return m, nil
	case "enter":
		// Toggle between detail and JSON view
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
	case "j":
		// Toggle JSON view on/off
		if m.mode == viewDetailJSON {
			m.mode = viewDetail
			if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				m.setViewportContent(m.renderLogDetail(m.logs[m.selectedIndex]))
				m.viewport.GotoTop()
			}
		} else {
			m.mode = viewDetailJSON
			if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				m.setViewportContent(es.PrettyJSON(m.logs[m.selectedIndex].RawJSON))
				m.viewport.GotoTop()
			}
		}
		return m, nil
	case "y":
		// Copy raw JSON to clipboard
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			m.copyToClipboard(es.PrettyJSON(m.logs[m.selectedIndex].RawJSON), "Copied JSON to clipboard!")
		}
		return m, nil
	case "s":
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
