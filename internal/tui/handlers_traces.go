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
		m.traceNamesCursor = listNav(m.traceNamesCursor, len(m.transactionNames), key)
		return m, nil
	}

	switch action {
	case ActionBack:
		// Trace names is a base view - esc does nothing (user can press 'q' to quit)
		return m, nil
	case ActionSelect:
		// Select transaction name and show transactions
		if len(m.transactionNames) > 0 && m.traceNamesCursor < len(m.transactionNames) {
			m.selectedTxName = m.transactionNames[m.traceNamesCursor].Name
			m.traceViewLevel = traceViewTransactions
			m.mode = viewLogs
			m.selectedIndex = 0
			m.loading = true
			return m, m.fetchLogs()
		}
	case ActionRefresh:
		m.tracesLoading = true
		return m, m.fetchTransactionNames()
	case ActionQuery:
		m.pushView(viewQuery)
		return m, nil
	case ActionSearch:
		m.pushView(viewSearch)
		m.searchInput.Focus()
		return m, textinput.Blink
	case ActionQuit:
		return m, tea.Quit
		// NOTE: ActionCycleLookback, ActionCycleSignal, ActionPerspective, ActionKibana
		// are now handled by handleCommonAction() above
	}

	return m, nil
}
