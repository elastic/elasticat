// Copyright 2026 Elasticsearch B.V. and contributors
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
		m.popView()
		m.UI.StatusMessage = ""
		return m, nil
	case "c":
		m.Query.Format = formatCurl
	case "k":
		m.Query.Format = formatKibana
	case "y":
		// Copy query to clipboard
		m.copyToClipboard(m.getQueryText(), "Copied to clipboard!")
	}
	return m, nil
}

// getQueryText returns the raw query text (without styling) for clipboard
func (m Model) getQueryText() string {
	index := m.Query.LastIndex

	if m.Query.Format == formatKibana {
		return fmt.Sprintf("GET %s/_search\n%s", index, m.Query.LastJSON)
	}

	// curl format
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(m.Query.LastJSON)); err != nil {
		return fmt.Sprintf("curl -X GET 'http://localhost:9200/%s/_search' \\\n  -H 'Content-Type: application/json' \\\n  -d '%s'",
			index, m.Query.LastJSON)
	}
	return fmt.Sprintf("curl -X GET 'http://localhost:9200/%s/_search' \\\n  -H 'Content-Type: application/json' \\\n  -d '%s'",
		index, compact.String())
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.Filters.Query = m.Components.SearchInput.Value()
		m.Logs.UserHasScrolled = false // Reset for tail -f behavior
		m.popView()
		m.Components.SearchInput.Blur()
		m.UI.Loading = true
		return m, m.fetchLogs()
	case "esc":
		m.popView()
		m.Components.SearchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.Components.SearchInput, cmd = m.Components.SearchInput.Update(msg)
	return m, cmd
}

func (m Model) handleIndexKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newIndex := m.Components.IndexInput.Value()
		if newIndex != "" {
			m.client.SetIndex(newIndex)
		}
		m.popView()
		m.Components.IndexInput.Blur()
		m.UI.Loading = true
		return m, m.fetchLogs()
	case "esc":
		m.popView()
		m.Components.IndexInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.Components.IndexInput, cmd = m.Components.IndexInput.Update(msg)
	return m, cmd
}
