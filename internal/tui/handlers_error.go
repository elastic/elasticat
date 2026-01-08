// Copyright 2026 Elasticsearch B.V. and contributors
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
		if m.UI.Err != nil {
			m.copyToClipboard(m.UI.Err.Error(), "Error copied to clipboard!")
		}
		return m, nil

	case "esc":
		// Close modal and return to previous view
		m.popView()
		m.UI.Err = nil
		return m, nil
	}

	// Handle viewport scrolling
	if viewportScroll(&m.Components.ErrorViewport, key) {
		return m, nil
	}

	// Pass other keys to viewport for mouse wheel support
	var cmd tea.Cmd
	m.Components.ErrorViewport, cmd = m.Components.ErrorViewport.Update(msg)
	return m, cmd
}
