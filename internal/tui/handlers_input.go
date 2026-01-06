// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleQueryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "Q":
		m.mode = viewLogs
		m.statusMessage = ""
		return m, nil
	case "c":
		m.queryFormat = formatCurl
	case "k":
		m.queryFormat = formatKibana
	case "y":
		// Copy query to clipboard
		m.copyToClipboard(m.getQueryText(), "Copied to clipboard!")
	}
	return m, nil
}

// getQueryText returns the raw query text (without styling) for clipboard
func (m Model) getQueryText() string {
	index := m.lastQueryIndex

	if m.queryFormat == formatKibana {
		return fmt.Sprintf("GET %s/_search\n%s", index, m.lastQueryJSON)
	}

	// curl format
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(m.lastQueryJSON)); err != nil {
		return fmt.Sprintf("curl -X GET 'http://localhost:9200/%s/_search' \\\n  -H 'Content-Type: application/json' \\\n  -d '%s'",
			index, m.lastQueryJSON)
	}
	return fmt.Sprintf("curl -X GET 'http://localhost:9200/%s/_search' \\\n  -H 'Content-Type: application/json' \\\n  -d '%s'",
		index, compact.String())
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searchQuery = m.searchInput.Value()
		m.userHasScrolled = false // Reset for tail -f behavior
		m.mode = viewLogs
		m.searchInput.Blur()
		m.loading = true
		return m, m.fetchLogs()
	case "esc":
		m.mode = viewLogs
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) handleIndexKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newIndex := m.indexInput.Value()
		if newIndex != "" {
			m.client.SetIndex(newIndex)
		}
		m.mode = viewLogs
		m.indexInput.Blur()
		m.loading = true
		return m, m.fetchLogs()
	case "esc":
		m.mode = viewLogs
		m.indexInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.indexInput, cmd = m.indexInput.Update(msg)
	return m, cmd
}
