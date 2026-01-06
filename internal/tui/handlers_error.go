// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleErrorModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "y":
		// Copy error to clipboard (consistent with 'y' in other views)
		if m.err != nil {
			m.copyToClipboard(m.err.Error(), "Error copied to clipboard!")
		}
		return m, nil

	case "q", "esc":
		// Close modal and return to previous view
		m.popView()
		m.err = nil
		return m, nil
	}

	// Handle viewport scrolling
	if viewportScroll(&m.errorViewport, key) {
		return m, nil
	}

	// Pass other keys to viewport for mouse wheel support
	var cmd tea.Cmd
	m.errorViewport, cmd = m.errorViewport.Update(msg)
	return m, cmd
}
