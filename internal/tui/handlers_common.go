// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
	"golang.design/x/clipboard"
)

// listNav handles standard list navigation, returning the new cursor position.
// cursor: current position, listLen: total items, key: the pressed key.
// Returns -1 if the key is not a navigation key.
func listNav(cursor, listLen int, key string) int {
	action := GetAction(key)
	switch action {
	case ActionScrollUp:
		if cursor > 0 {
			return cursor - 1
		}
		return cursor
	case ActionScrollDown:
		if cursor < listLen-1 {
			return cursor + 1
		}
		return cursor
	case ActionGoTop:
		return 0
	case ActionGoBottom:
		if listLen > 0 {
			return listLen - 1
		}
		return 0
	case ActionPageUp:
		newCursor := cursor - 10
		if newCursor < 0 {
			return 0
		}
		return newCursor
	case ActionPageDown:
		newCursor := cursor + 10
		if listLen > 0 && newCursor >= listLen {
			return listLen - 1
		}
		if newCursor < 0 {
			return 0
		}
		return newCursor
	}
	return -1 // Not a navigation key
}

// isNavKey returns true if the key is a list navigation key
func isNavKey(key string) bool {
	return IsListNavAction(GetAction(key))
}

// viewportScroll handles standard viewport scrolling keys.
// Returns true if the key was handled, false otherwise.
// The viewport is modified in place.
func viewportScroll(vp *viewport.Model, key string) bool {
	action := GetAction(key)
	switch action {
	case ActionScrollDown:
		vp.ScrollDown(1)
		return true
	case ActionScrollUp:
		vp.ScrollUp(1)
		return true
	case ActionPageDown:
		vp.HalfPageDown()
		return true
	case ActionPageUp:
		vp.HalfPageUp()
		return true
	case ActionGoTop:
		vp.GotoTop()
		return true
	case ActionGoBottom:
		vp.GotoBottom()
		return true
	}
	return false
}

// pushView saves current mode to the view stack and transitions to a new view
func (m *Model) pushView(newMode viewMode) {
	m.UI.ViewStack = append(m.UI.ViewStack, ViewContext{Mode: m.UI.Mode})
	m.UI.Mode = newMode
}

// popView returns to the previous view from the stack, returns false if stack is empty
func (m *Model) popView() bool {
	if len(m.UI.ViewStack) == 0 {
		return false
	}
	n := len(m.UI.ViewStack) - 1
	m.UI.Mode = m.UI.ViewStack[n].Mode
	m.UI.ViewStack = m.UI.ViewStack[:n]
	return true
}

// peekViewStack returns the mode at the top of the stack without removing it
// Returns the current mode if stack is empty (for rendering background)
func (m *Model) peekViewStack() viewMode {
	if len(m.UI.ViewStack) == 0 {
		return m.UI.Mode
	}
	return m.UI.ViewStack[len(m.UI.ViewStack)-1].Mode
}

// clearViewStack resets navigation history (e.g., on signal change)
func (m *Model) clearViewStack() {
	m.UI.ViewStack = m.UI.ViewStack[:0]
}

// cycleSignalType switches to the next signal type and returns the appropriate command
func (m *Model) cycleSignalType() tea.Cmd {
	// Cycle: logs -> traces -> metrics -> chat -> logs
	switch m.Filters.Signal {
	case signalLogs:
		m.Filters.Signal = signalTraces
	case signalTraces:
		m.Filters.Signal = signalMetrics
	case signalMetrics:
		m.Filters.Signal = signalChat
	case signalChat:
		m.Filters.Signal = signalLogs
	}

	// Clear navigation history when switching signals
	m.clearViewStack()

	// Chat doesn't use an index pattern
	if m.Filters.Signal != signalChat {
		m.client.SetIndex(m.Filters.Signal.IndexPattern())
		m.Fields.Display = DefaultFields(m.Filters.Signal)
		m.Logs.Entries = []es.LogEntry{}
		m.Logs.SelectedIndex = 0
		m.UI.StatusMessage = "Auto-detecting time range..."
		m.UI.StatusTime = time.Now()
	}

	return m.enterSignalView()
}

// enterSignalView sets the appropriate mode and returns fetch command for current signal
func (m *Model) enterSignalView() tea.Cmd {
	switch m.Filters.Signal {
	case signalMetrics:
		m.UI.Mode = viewMetricsDashboard
		m.Metrics.Loading = true
		m.Metrics.Cursor = 0
		m.UI.Loading = false
		return tea.Batch(m.autoDetectLookback(), m.fetchAggregatedMetrics())
	case signalTraces:
		m.UI.Mode = viewTraceNames
		m.Traces.ViewLevel = traceViewNames
		m.Traces.Loading = true
		m.Traces.NamesCursor = 0
		m.Traces.SelectedTxName = ""
		m.Traces.SelectedTraceID = ""
		m.UI.Loading = false
		return tea.Batch(m.autoDetectLookback(), m.fetchTransactionNames())
	case signalChat:
		m.UI.Mode = viewChat
		m.UI.Loading = false
		_, cmd := m.enterChatView()
		return cmd
	default:
		m.UI.Mode = viewLogs
		m.UI.Loading = true
		return m.autoDetectLookback()
	}
}

// cyclePerspective switches perspective type and enters perspective list view
func (m *Model) cyclePerspective() tea.Cmd {
	switch m.Perspective.Current {
	case PerspectiveServices:
		m.Perspective.Current = PerspectiveResources
	case PerspectiveResources:
		m.Perspective.Current = PerspectiveServices
	}
	return m.enterPerspectiveView()
}

// enterPerspectiveView sets up the perspective list view
func (m *Model) enterPerspectiveView() tea.Cmd {
	m.pushView(viewPerspectiveList)
	m.Perspective.Cursor = 0
	m.Perspective.Items = []PerspectiveItem{}
	m.Perspective.Loading = true
	m.UI.StatusMessage = fmt.Sprintf("Loading %s...", m.Perspective.Current.String())
	m.UI.StatusTime = time.Now()
	return m.fetchPerspectiveData()
}

// cycleLookback advances to the next lookback duration
func (m *Model) cycleLookback() {
	for i, lb := range lookbackDurations {
		if lb == m.Filters.Lookback {
			m.Filters.Lookback = lookbackDurations[(i+1)%len(lookbackDurations)]
			return
		}
	}
}

// copyToClipboard copies text and sets status message
func (m *Model) copyToClipboard(text, successMsg string) {
	if err := clipboard.Init(); err != nil {
		m.UI.StatusMessage = "Clipboard error: " + err.Error()
	} else {
		clipboard.Write(clipboard.FmtText, []byte(text))
		m.UI.StatusMessage = successMsg
	}
	m.UI.StatusTime = time.Now()
}

// === View Category Helpers ===

// isBaseView returns true for signal root views that don't pop anywhere.
func (m Model) isBaseView() bool {
	switch m.UI.Mode {
	case viewLogs, viewMetricsDashboard, viewTraceNames, viewChat:
		return true
	}
	return false
}

// isModalView returns true for overlay views (error, help, quit, etc).
func (m Model) isModalView() bool {
	switch m.UI.Mode {
	case viewErrorModal, viewQuitConfirm, viewHelp, viewCredsModal,
		viewOtelConfigExplain, viewOtelConfigModal, viewOtelConfigUnavailable:
		return true
	}
	return false
}

// === Common Action Handling ===

// handleCommonAction handles actions shared across data views (logs, metrics, traces).
// Does NOT handle chat-specific behavior. Returns (model, cmd, handled).
// Callers should return immediately if handled is true.
func (m Model) handleCommonAction(action Action) (Model, tea.Cmd, bool) {
	switch action {
	case ActionCycleSignal:
		return m, m.cycleSignalType(), true
	case ActionCycleLookback:
		m.cycleLookback()
		return m, m.fetchCurrentViewData(), true
	case ActionPerspective:
		return m, m.cyclePerspective(), true
	case ActionKibana:
		if m.prepareKibanaURL() {
			m.showCredsModal()
		}
		return m, nil, true
	}
	return m, nil, false
}

// fetchCurrentViewData returns the appropriate fetch command for the current signal.
func (m *Model) fetchCurrentViewData() tea.Cmd {
	switch m.Filters.Signal {
	case signalLogs:
		m.UI.Loading = true
		return m.fetchLogs()
	case signalMetrics:
		m.Metrics.Loading = true
		return m.fetchAggregatedMetrics()
	case signalTraces:
		m.Traces.Loading = true
		return m.fetchTransactionNames()
	}
	return nil
}
