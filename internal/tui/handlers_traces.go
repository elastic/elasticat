// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleTraceNamesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	// Handle list navigation
	if isNavKey(key) {
		m.traceNamesCursor = listNav(m.traceNamesCursor, len(m.transactionNames), key)
		return m, nil
	}

	switch action {
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
	case ActionPerspective:
		return m, m.cyclePerspective()
	case ActionCycleLookback:
		m.cycleLookback()
		m.tracesLoading = true
		return m, m.fetchTransactionNames()
	case ActionCycleSignal:
		return m, m.cycleSignalType()
	case ActionSearch:
		m.pushView(viewSearch)
		m.searchInput.Focus()
		return m, textinput.Blink
	case ActionQuit:
		return m, tea.Quit
	}

	return m, nil
}
