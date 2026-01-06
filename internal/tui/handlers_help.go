// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "q":
		m.popView()
		return m, nil
	}

	// Handle viewport scrolling
	if viewportScroll(&m.helpViewport, key) {
		return m, nil
	}

	// Pass other keys to viewport for mouse wheel support
	var cmd tea.Cmd
	m.helpViewport, cmd = m.helpViewport.Update(msg)
	return m, cmd
}
