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
	key := msg.String()
	action := GetAction(key)

	// Handle list navigation (adds pgup/pgdown, home/end support)
	if isNavKey(key) {
		m.perspectiveCursor = listNav(m.perspectiveCursor, len(m.perspectiveItems), key)
		return m, nil
	}

	switch action {
	case ActionSelect:
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
	case ActionPerspective:
		return m, m.cyclePerspective()
	case ActionCycleLookback:
		m.cycleLookback()
		m.perspectiveLoading = true
		return m, m.fetchPerspectiveData()
	case ActionRefresh:
		// Refresh perspective data
		m.perspectiveLoading = true
		return m, m.fetchPerspectiveData()
	case ActionSearch:
		// Enter search mode (consistent with other list views)
		m.pushView(viewSearch)
		m.searchInput.Focus()
		return m, textinput.Blink
	case ActionBack:
		// Return to previous view via stack
		m.popView()
		return m, nil
	case ActionQuit:
		return m, tea.Quit
	}

	return m, nil
}
