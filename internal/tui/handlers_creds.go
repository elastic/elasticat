// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleCredsModalKey handles key events in the credentials modal
func (m Model) handleCredsModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	switch action {
	case ActionBack:
		// Dismiss modal and clear Creds.LastKibanaURL
		m.Creds.LastKibanaURL = ""
		m.popView()
		return m, nil
	case ActionSelect:
		// Open browser with the Kibana URL (keep modal open)
		if m.Creds.LastKibanaURL != "" {
			m.openLastKibanaURL()
		}
		return m, nil
	case ActionCopy:
		// Copy URL to clipboard
		if m.Creds.LastKibanaURL != "" {
			m.copyToClipboard(m.Creds.LastKibanaURL, "URL copied to clipboard!")
		}
		return m, nil
	}

	switch key {
	case "n":
		// Never show again this session
		m.Creds.HideModal = true
		m.Creds.LastKibanaURL = ""
		m.popView()
		m.UI.StatusMessage = "Use 'elasticat creds' to view credentials"
		m.UI.StatusTime = time.Now()
		return m, nil
	case "p":
		// Copy password to clipboard
		if m.esPassword != "" {
			m.copyToClipboard(m.esPassword, "Password copied to clipboard!")
		}
		return m, nil
	}

	return m, nil
}

// showCredsModal pushes the credentials modal onto the view stack.
// Does nothing if Creds.HideModal is true (user opted out for this session).
func (m *Model) showCredsModal() {
	if m.Creds.HideModal {
		// If user opted out, just open the browser directly
		if m.Creds.LastKibanaURL != "" {
			m.openLastKibanaURL()
		}
		return
	}
	m.pushView(viewCredsModal)
}
