// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = m.previousMode
		return m, nil
	case "j", "down":
		m.helpViewport.ScrollDown(1)
		return m, nil
	case "k", "up":
		m.helpViewport.ScrollUp(1)
		return m, nil
	case "pgdown", "d":
		m.helpViewport.HalfPageDown()
		return m, nil
	case "pgup", "u":
		m.helpViewport.HalfPageUp()
		return m, nil
	case "g", "home":
		m.helpViewport.GotoTop()
		return m, nil
	case "G", "end":
		m.helpViewport.GotoBottom()
		return m, nil
	}

	var cmd tea.Cmd
	m.helpViewport, cmd = m.helpViewport.Update(msg)
	return m, cmd
}

