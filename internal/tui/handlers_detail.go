// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"time"

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
		m.UI.StatusMessage = ""
		return m, nil
	case ActionPrevItem:
		// Navigate to previous entry
		if m.Logs.SelectedIndex > 0 {
			m.Logs.SelectedIndex--
			m.updateDetailContent()
			// Fetch spans for traces signal type
			if m.Filters.Signal == signalTraces {
				return m, m.maybeFetchSpansForSelection()
			}
		}
		return m, nil
	case ActionNextItem:
		// Navigate to next entry
		if m.Logs.SelectedIndex < len(m.Logs.Entries)-1 {
			m.Logs.SelectedIndex++
			m.updateDetailContent()
			// Fetch spans for traces signal type
			if m.Filters.Signal == signalTraces {
				return m, m.maybeFetchSpansForSelection()
			}
		}
		return m, nil
	case ActionSelect:
		// Toggle between detail and JSON view (same stack level)
		if m.UI.Mode == viewDetail {
			m.UI.Mode = viewDetailJSON
			if len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
				m.setViewportContent(es.PrettyJSON(m.Logs.Entries[m.Logs.SelectedIndex].RawJSON))
				m.Components.Viewport.GotoTop()
			}
		} else {
			m.UI.Mode = viewDetail
			if len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
				m.setViewportContent(m.renderLogDetail(m.Logs.Entries[m.Logs.SelectedIndex]))
				m.Components.Viewport.GotoTop()
			}
		}
		return m, nil
	case ActionCopy:
		// Copy raw JSON to clipboard
		if len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
			m.copyToClipboard(es.PrettyJSON(m.Logs.Entries[m.Logs.SelectedIndex].RawJSON), "Copied JSON to clipboard!")
		}
		return m, nil
	case ActionCopyOriginal:
		// Copy log.record.original to clipboard (Y key)
		if len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
			log := m.Logs.Entries[m.Logs.SelectedIndex]
			if original := log.GetOriginal(); original != "" {
				m.copyToClipboard(original, "Copied original log to clipboard!")
			} else {
				m.UI.StatusMessage = "No log.record.original found"
				m.UI.StatusTime = time.Now()
			}
		}
		return m, nil
	case ActionKibana:
		// Prepare Kibana URL for trace and show creds modal (only for traces)
		if m.Filters.Signal == signalTraces && len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
			log := m.Logs.Entries[m.Logs.SelectedIndex]
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
		if m.UI.Mode == viewDetailJSON {
			// If we came from metric detail (via J key), pop back to it
			if m.peekViewStack() == viewMetricDetail {
				m.popView()
			} else {
				// Otherwise toggle to formatted detail view
				m.UI.Mode = viewDetail
				if len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
					m.setViewportContent(m.renderLogDetail(m.Logs.Entries[m.Logs.SelectedIndex]))
					m.Components.Viewport.GotoTop()
				}
			}
		} else {
			m.UI.Mode = viewDetailJSON
			if len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
				m.setViewportContent(es.PrettyJSON(m.Logs.Entries[m.Logs.SelectedIndex].RawJSON))
				m.Components.Viewport.GotoTop()
			}
		}
		return m, nil
	case ActionSpans:
		// Show spans for this trace (only for traces)
		if m.Filters.Signal == signalTraces && len(m.Logs.Entries) > 0 && m.Logs.SelectedIndex < len(m.Logs.Entries) {
			log := m.Logs.Entries[m.Logs.SelectedIndex]
			if log.TraceID != "" {
				m.Traces.SelectedTraceID = log.TraceID
				m.Traces.ViewLevel = traceViewSpans
				m.UI.Mode = viewLogs
				m.Logs.SelectedIndex = 0
				m.UI.Loading = true
				return m, m.fetchLogs()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.Components.Viewport, cmd = m.Components.Viewport.Update(msg)
	return m, cmd
}

// updateDetailContent refreshes the detail view content for the current selection
func (m *Model) updateDetailContent() {
	if len(m.Logs.Entries) == 0 || m.Logs.SelectedIndex >= len(m.Logs.Entries) {
		return
	}
	if m.UI.Mode == viewDetailJSON {
		m.setViewportContent(es.PrettyJSON(m.Logs.Entries[m.Logs.SelectedIndex].RawJSON))
	} else {
		m.setViewportContent(m.renderLogDetail(m.Logs.Entries[m.Logs.SelectedIndex]))
	}
	m.Components.Viewport.GotoTop()
}
