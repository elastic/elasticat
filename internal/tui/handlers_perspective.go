// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handlePerspectiveListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle list navigation (adds pgup/pgdown, home/end support)
	if isNavKey(msg.String()) {
		m.perspectiveCursor = listNav(m.perspectiveCursor, len(m.perspectiveItems), msg.String())
		return m, nil
	}

	switch msg.String() {
	case "enter":
		// Cycle through filter states: unset → include → exclude → unset
		if len(m.perspectiveItems) > 0 {
			selected := m.perspectiveItems[m.perspectiveCursor]

			switch m.currentPerspective {
			case PerspectiveServices:
				if m.filterService != selected.Name {
					// Different item or no filter: set include filter
					m.filterService = selected.Name
					m.negateService = false
					m.statusMessage = fmt.Sprintf("Filtered to service: %s", selected.Name)
				} else if !m.negateService {
					// Same item, currently included: switch to exclude
					m.negateService = true
					m.statusMessage = fmt.Sprintf("Excluding service: %s", selected.Name)
				} else {
					// Same item, currently excluded: clear filter
					m.filterService = ""
					m.negateService = false
					m.statusMessage = fmt.Sprintf("Cleared service filter: %s", selected.Name)
				}
				m.userHasScrolled = false // Reset for tail -f behavior
			case PerspectiveResources:
				if m.filterResource != selected.Name {
					// Different item or no filter: set include filter
					m.filterResource = selected.Name
					m.negateResource = false
					m.statusMessage = fmt.Sprintf("Filtered to resource: %s", selected.Name)
				} else if !m.negateResource {
					// Same item, currently included: switch to exclude
					m.negateResource = true
					m.statusMessage = fmt.Sprintf("Excluding resource: %s", selected.Name)
				} else {
					// Same item, currently excluded: clear filter
					m.filterResource = ""
					m.negateResource = false
					m.statusMessage = fmt.Sprintf("Cleared resource filter: %s", selected.Name)
				}
				m.userHasScrolled = false // Reset for tail -f behavior
			}
			m.statusTime = time.Now()

			// Stay in perspective view - user can navigate back with 'esc' when ready
		}
		return m, nil
	case "p":
		return m, m.cyclePerspective()
	case "l":
		m.cycleLookback()
		m.perspectiveLoading = true
		return m, m.fetchPerspectiveData()
	case "r":
		// Refresh perspective data
		m.perspectiveLoading = true
		return m, m.fetchPerspectiveData()
	case "/":
		// Enter search mode (consistent with other list views)
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "esc":
		// Return to appropriate view based on signal type and drill-down level
		switch m.signalType {
		case signalMetrics:
			m.mode = viewMetricsDashboard
		case signalTraces:
			// Check trace view level to return to correct view
			switch m.traceViewLevel {
			case traceViewTransactions, traceViewSpans:
				// User was viewing transactions or spans (which use viewLogs)
				m.mode = viewLogs
			default:
				// User was at the top-level transaction names list
				m.mode = viewTraceNames
			}
		default:
			m.mode = viewLogs
		}
		return m, nil
	case "q":
		return m, tea.Quit
	}

	return m, nil
}
