// Copyright 2026 Elasticsearch B.V. and contributors
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
		m.Perspective.Cursor = listNav(m.Perspective.Cursor, len(m.Perspective.Items), key)
		return m, nil
	}

	switch action {
	case ActionSelect:
		// Cycle through filter states: unset → include → exclude → unset
		if len(m.Perspective.Items) > 0 {
			selected := m.Perspective.Items[m.Perspective.Cursor]

			switch m.Perspective.Current {
			case PerspectiveServices:
				if m.Filters.Service != selected.Name {
					// Different item or no filter: set include filter
					m.Filters.Service = selected.Name
					m.Filters.NegateService = false
					m.UI.StatusMessage = fmt.Sprintf("Filtered to service: %s", selected.Name)
				} else if !m.Filters.NegateService {
					// Same item, currently included: switch to exclude
					m.Filters.NegateService = true
					m.UI.StatusMessage = fmt.Sprintf("Excluding service: %s", selected.Name)
				} else {
					// Same item, currently excluded: clear filter
					m.Filters.Service = ""
					m.Filters.NegateService = false
					m.UI.StatusMessage = fmt.Sprintf("Cleared service filter: %s", selected.Name)
				}
				m.Logs.UserHasScrolled = false // Reset for tail -f behavior
			case PerspectiveResources:
				if m.Filters.Resource != selected.Name {
					// Different item or no filter: set include filter
					m.Filters.Resource = selected.Name
					m.Filters.NegateResource = false
					m.UI.StatusMessage = fmt.Sprintf("Filtered to resource: %s", selected.Name)
				} else if !m.Filters.NegateResource {
					// Same item, currently included: switch to exclude
					m.Filters.NegateResource = true
					m.UI.StatusMessage = fmt.Sprintf("Excluding resource: %s", selected.Name)
				} else {
					// Same item, currently excluded: clear filter
					m.Filters.Resource = ""
					m.Filters.NegateResource = false
					m.UI.StatusMessage = fmt.Sprintf("Cleared resource filter: %s", selected.Name)
				}
				m.Logs.UserHasScrolled = false // Reset for tail -f behavior
			}
			m.UI.StatusTime = time.Now()

			// Stay in perspective view - user can navigate back with 'esc' when ready
		}
		return m, nil
	case ActionPerspective:
		return m, m.cyclePerspective()
	case ActionCycleLookback:
		m.cycleLookback()
		m.Perspective.Loading = true
		return m, m.fetchPerspectiveData()
	case ActionRefresh:
		// Refresh perspective data
		m.Perspective.Loading = true
		return m, m.fetchPerspectiveData()
	case ActionSearch:
		// Enter search mode (consistent with other list views)
		m.pushView(viewSearch)
		m.Components.SearchInput.Focus()
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
