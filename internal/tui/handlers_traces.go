// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleTraceNamesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	// Handle common actions first (signal cycle, lookback, perspective, kibana)
	if newM, cmd, handled := m.handleCommonAction(action); handled {
		return newM, cmd
	}

	// Handle list navigation
	if isNavKey(key) {
		m.Traces.NamesCursor = listNav(m.Traces.NamesCursor, len(m.Traces.TransactionNames), key)
		return m, nil
	}

	switch action {
	case ActionBack:
		// Trace names is a base view - esc does nothing (user can press 'q' to quit)
		return m, nil
	case ActionSelect:
		// Select transaction name and show transactions
		if len(m.Traces.TransactionNames) > 0 && m.Traces.NamesCursor < len(m.Traces.TransactionNames) {
			m.Traces.SelectedTxName = m.Traces.TransactionNames[m.Traces.NamesCursor].Name
			m.Traces.ViewLevel = traceViewTransactions
			m.UI.Mode = viewLogs
			m.Logs.SelectedIndex = 0
			m.UI.Loading = true
			return m, m.fetchLogs()
		}
	case ActionRefresh:
		m.Traces.Loading = true
		return m, m.fetchTransactionNames()
	case ActionQuery:
		m.pushView(viewQuery)
		return m, nil
	case ActionSearch:
		m.pushView(viewSearch)
		m.Components.SearchInput.Focus()
		return m, textinput.Blink
	case ActionQuit:
		return m, tea.Quit
		// NOTE: ActionCycleLookback, ActionCycleSignal, ActionPerspective, ActionKibana
		// are now handled by handleCommonAction() above
	}

	return m, nil
}
