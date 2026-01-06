// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleTraceNamesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle list navigation
	if isNavKey(msg.String()) {
		m.traceNamesCursor = listNav(m.traceNamesCursor, len(m.transactionNames), msg.String())
		return m, nil
	}

	switch msg.String() {
	case "enter":
		// Select transaction name and show transactions
		if len(m.transactionNames) > 0 && m.traceNamesCursor < len(m.transactionNames) {
			m.selectedTxName = m.transactionNames[m.traceNamesCursor].Name
			m.traceViewLevel = traceViewTransactions
			m.mode = viewLogs
			m.selectedIndex = 0
			m.loading = true
			return m, m.fetchLogs()
		}
	case "r":
		m.tracesLoading = true
		return m, m.fetchTransactionNames()
	case "p":
		return m, m.cyclePerspective()
	case "l":
		m.cycleLookback()
		m.tracesLoading = true
		return m, m.fetchTransactionNames()
	case "m":
		return m, m.cycleSignalType()
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "q":
		return m, tea.Quit
	}

	return m, nil
}
