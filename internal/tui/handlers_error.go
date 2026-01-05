// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleErrorModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "y":
		// Copy error to clipboard (consistent with 'y' in other views)
		if m.err != nil {
			m.copyToClipboard(m.err.Error(), "Error copied to clipboard!")
		}
		return m, nil

	case "q", "esc":
		// Close modal and return to previous view
		m.mode = m.previousMode
		m.err = nil
		return m, nil

	case "j", "down":
		m.errorViewport.ScrollDown(1)
		return m, nil

	case "k", "up":
		m.errorViewport.ScrollUp(1)
		return m, nil

	case "d", "pgdown":
		m.errorViewport.HalfPageDown()
		return m, nil

	case "u", "pgup":
		m.errorViewport.HalfPageUp()
		return m, nil

	case "g", "home":
		m.errorViewport.GotoTop()
		return m, nil

	case "G", "end":
		m.errorViewport.GotoBottom()
		return m, nil
	}

	// Pass other keys to viewport for mouse wheel support
	m.errorViewport, cmd = m.errorViewport.Update(msg)
	return m, cmd
}
