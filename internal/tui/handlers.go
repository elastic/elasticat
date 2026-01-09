// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/config"
)

// handleKey routes key events to mode-specific handlers
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	key := msg.String()
	action := GetAction(key)

	// System keys
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// Quit confirmation (unless in text input or already showing quit modal)
	if key == "q" && m.UI.Mode != viewQuitConfirm && !m.isTextInputActive() {
		m.pushView(viewQuitConfirm)
		return m, nil
	}

	// NOTE: esc is now handled by each view's handler directly.
	// All views must explicitly handle ActionBack for proper navigation.

	// Action-based global keys
	switch action {
	case ActionHelp:
		// Global help only when enabled and not in text-input modes
		if m.HelpEnabled() && !m.isTextInputActive() {
			m.pushView(viewHelp)
			m.renderHelpOverlay()
			return m, nil
		}
	case ActionChat:
		// Open chat from any view (except during text input or chat itself)
		if !m.isTextInputActive() && m.UI.Mode != viewChat {
			return m.enterChatView()
		}
	case ActionCreds:
		// Show credentials modal from any view (except during text input or already in creds modal)
		if !m.isTextInputActive() && m.UI.Mode != viewCredsModal {
			m.Creds.LastKibanaURL = "" // Clear any previous URL since this is direct access
			m.pushView(viewCredsModal)
			return m, nil
		}
	case ActionOtelConfig:
		// Show OTel config explanation modal from any view (except during text input)
		if !m.isTextInputActive() && m.UI.Mode != viewOtelConfigExplain && m.UI.Mode != viewOtelConfigModal && m.UI.Mode != viewOtelConfigUnavailable {
			// OTel config editing only works with the locally managed stack
			if m.profileName != config.StartLocalProfileName {
				m.pushView(viewOtelConfigUnavailable)
				return m, nil
			}
			m.pushView(viewOtelConfigExplain)
			return m, nil
		}
	case ActionSendToChat:
		// Send selected item to chat from any view (except during text input or chat itself)
		if !m.isTextInputActive() && m.UI.Mode != viewChat {
			return m.enterChatWithSelectedItem()
		}
	}

	// Mode-specific keys
	switch m.UI.Mode {
	case viewLogs:
		return m.handleLogsKey(msg)
	case viewSearch:
		return m.handleSearchKey(msg)
	case viewDetail, viewDetailJSON:
		return m.handleDetailKey(msg)
	case viewIndex:
		return m.handleIndexKey(msg)
	case viewQuery:
		return m.handleQueryKey(msg)
	case viewFields:
		return m.handleFieldsKey(msg)
	case viewMetricsDashboard:
		return m.handleMetricsDashboardKey(msg)
	case viewMetricDetail:
		return m.handleMetricDetailKey(msg)
	case viewTraceNames:
		return m.handleTraceNamesKey(msg)
	case viewPerspectiveList:
		return m.handlePerspectiveListKey(msg)
	case viewErrorModal:
		return m.handleErrorModalKey(msg)
	case viewQuitConfirm:
		return m.handleQuitConfirmKey(msg)
	case viewHelp:
		return m.handleHelpKey(msg)
	case viewChat:
		return m.handleChatKey(msg)
	case viewCredsModal:
		return m.handleCredsModalKey(msg)
	case viewOtelConfigExplain:
		return m.handleOtelConfigExplainKey(msg)
	case viewOtelConfigModal:
		return m.handleOtelConfigModalKey(msg)
	case viewOtelConfigUnavailable:
		return m.handleOtelConfigUnavailableKey(msg)
	}

	return m, nil
}

// isTextInputActive returns true when a text input is active, disabling global hotkeys like h.
func (m Model) isTextInputActive() bool {
	switch m.UI.Mode {
	case viewSearch, viewIndex, viewQuery:
		return true
	case viewFields:
		return m.Fields.SearchMode
	case viewChat:
		return m.Chat.InsertMode // Fixed: was m.Chat.Input.Focused()
	default:
		return false
	}
}

// handleMouse handles mouse events across all views
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel scrolling
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		switch m.UI.Mode {
		case viewLogs:
			// Scroll up in log list (2 items at a time for speed)
			if m.Logs.SelectedIndex > 0 {
				m.Logs.SelectedIndex -= 2
				if m.Logs.SelectedIndex < 0 {
					m.Logs.SelectedIndex = 0
				}
			}
		case viewDetail, viewDetailJSON:
			// Scroll up in detail viewport
			m.Components.Viewport.ScrollUp(3)
		case viewFields:
			// Scroll up in field selector
			if m.Fields.Cursor > 0 {
				m.Fields.Cursor -= 2
				if m.Fields.Cursor < 0 {
					m.Fields.Cursor = 0
				}
			}
		case viewMetricsDashboard:
			// Scroll up in metrics dashboard
			if m.Metrics.Cursor > 0 {
				m.Metrics.Cursor -= 2
				if m.Metrics.Cursor < 0 {
					m.Metrics.Cursor = 0
				}
			}
		case viewTraceNames:
			// Scroll up in trace names list
			if m.Traces.NamesCursor > 0 {
				m.Traces.NamesCursor -= 2
				if m.Traces.NamesCursor < 0 {
					m.Traces.NamesCursor = 0
				}
			}
		case viewPerspectiveList:
			// Scroll up in perspective list
			if m.Perspective.Cursor > 0 {
				m.Perspective.Cursor -= 2
				if m.Perspective.Cursor < 0 {
					m.Perspective.Cursor = 0
				}
			}
		case viewChat:
			// Scroll up in chat viewport
			m.Chat.Viewport.ScrollUp(3)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		switch m.UI.Mode {
		case viewLogs:
			// Scroll down in log list (2 items at a time for speed)
			if m.Logs.SelectedIndex < len(m.Logs.Entries)-1 {
				m.Logs.SelectedIndex += 2
				if m.Logs.SelectedIndex >= len(m.Logs.Entries) {
					m.Logs.SelectedIndex = len(m.Logs.Entries) - 1
				}
			}
		case viewDetail, viewDetailJSON:
			// Scroll down in detail viewport
			m.Components.Viewport.ScrollDown(3)
		case viewFields:
			// Scroll down in field selector
			sortedFields := m.getSortedFieldList()
			if m.Fields.Cursor < len(sortedFields)-1 {
				m.Fields.Cursor += 2
				if m.Fields.Cursor >= len(sortedFields) {
					m.Fields.Cursor = len(sortedFields) - 1
				}
			}
		case viewMetricsDashboard:
			// Scroll down in metrics dashboard
			if m.Metrics.Aggregated != nil && m.Metrics.Cursor < len(m.Metrics.Aggregated.Metrics)-1 {
				m.Metrics.Cursor += 2
				if m.Metrics.Cursor >= len(m.Metrics.Aggregated.Metrics) {
					m.Metrics.Cursor = len(m.Metrics.Aggregated.Metrics) - 1
				}
			}
		case viewTraceNames:
			// Scroll down in trace names list
			if m.Traces.NamesCursor < len(m.Traces.TransactionNames)-1 {
				m.Traces.NamesCursor += 2
				if m.Traces.NamesCursor >= len(m.Traces.TransactionNames) {
					m.Traces.NamesCursor = len(m.Traces.TransactionNames) - 1
				}
			}
		case viewPerspectiveList:
			// Scroll down in perspective list
			if m.Perspective.Cursor < len(m.Perspective.Items)-1 {
				m.Perspective.Cursor += 2
				if m.Perspective.Cursor >= len(m.Perspective.Items) {
					m.Perspective.Cursor = len(m.Perspective.Items) - 1
				}
			}
		case viewChat:
			// Scroll down in chat viewport
			m.Chat.Viewport.ScrollDown(3)
		}
		return m, nil
	}

	// Handle left clicks
	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionRelease {
		return m, nil
	}

	// Only handle clicks in log list mode on the first row (status bar)
	if m.UI.Mode == viewLogs && msg.Y == 0 {
		// Calculate the approximate position of the "Sort:" label in the status bar
		// The status bar contains: Signal, ES, Idx, Total, [Query], [Level], [Service], Sort, Auto
		sortStart, sortEnd := m.getSortLabelPosition()
		if msg.X >= sortStart && msg.X <= sortEnd {
			// Toggle sort order
			m.UI.SortAscending = !m.UI.SortAscending
			m.UI.Loading = true
			return m, m.fetchLogs()
		}
	}

	return m, nil
}

// getSortLabelPosition returns the approximate start and end X positions of the "Sort:" label
func (m Model) getSortLabelPosition() (start, end int) {
	// Build the status bar parts to calculate position
	// Note: This mirrors the logic in renderStatusBar but just calculates lengths
	pos := 1 // Start after padding

	// Signal: <type>
	pos += len("Signal: ") + len(m.Filters.Signal.String()) + 5 // + separator

	// ES: ok/err
	if m.UI.Err != nil {
		pos += len("ES: err") + 5
	} else {
		pos += len("ES: ok") + 5
	}

	// Idx: <index>
	pos += len("Idx: ") + len(m.client.GetIndex()) + 5 // +5 for separator

	// Total: <count>
	pos += len("Total: ") + len(fmt.Sprintf("%d", m.Logs.Total)) + 5

	// Optional Query filter
	if m.Filters.Query != "" {
		displayed := TruncateWithEllipsis(m.Filters.Query, 20)
		pos += len("Query: ") + len(displayed) + 5
	}

	// Optional Level filter
	if m.Filters.Level != "" {
		pos += len("Level: ") + len(m.Filters.Level) + 5
	}

	// Optional Service filter
	if m.Filters.Service != "" {
		pos += len("Service: ") + len(m.Filters.Service) + 5
	}

	// Lookback
	pos += len("Lookback: ") + len(m.Filters.Lookback.String()) + 5

	// Now we're at "Sort: "
	start = pos
	sortText := "newest→"
	if m.UI.SortAscending {
		sortText = "oldest→"
	}
	end = start + len("Sort: ") + len(sortText)

	return start, end
}

// Layout constants
const (
	statusBarHeight     = 1 // Usually one row (two when ES error)
	helpBarHeight       = 1
	compactDetailHeight = 5 // 3 lines of content + 2 for border
	layoutPadding       = 2 // Top/bottom padding from AppStyle
)
