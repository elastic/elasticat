// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleQuitConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case quitConfirmYesKey:
		return m, tea.Quit
	case quitConfirmNoKey, "esc":
		m.popView()
		return m, nil
	}
	return m, nil
}

const (
	quitConfirmYesKey = "y"
	quitConfirmNoKey  = "n"
)
