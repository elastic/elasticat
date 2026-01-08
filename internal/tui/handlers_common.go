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
	m.viewStack = append(m.viewStack, ViewContext{Mode: m.mode})
	m.mode = newMode
}

// popView returns to the previous view from the stack, returns false if stack is empty
func (m *Model) popView() bool {
	if len(m.viewStack) == 0 {
		return false
	}
	n := len(m.viewStack) - 1
	m.mode = m.viewStack[n].Mode
	m.viewStack = m.viewStack[:n]
	return true
}

// peekViewStack returns the mode at the top of the stack without removing it
// Returns the current mode if stack is empty (for rendering background)
func (m *Model) peekViewStack() viewMode {
	if len(m.viewStack) == 0 {
		return m.mode
	}
	return m.viewStack[len(m.viewStack)-1].Mode
}

// clearViewStack resets navigation history (e.g., on signal change)
func (m *Model) clearViewStack() {
	m.viewStack = m.viewStack[:0]
}

// cycleSignalType switches to the next signal type and returns the appropriate command
func (m *Model) cycleSignalType() tea.Cmd {
	// Cycle: logs -> traces -> metrics -> chat -> logs
	switch m.signalType {
	case signalLogs:
		m.signalType = signalTraces
	case signalTraces:
		m.signalType = signalMetrics
	case signalMetrics:
		m.signalType = signalChat
	case signalChat:
		m.signalType = signalLogs
	}

	// Clear navigation history when switching signals
	m.clearViewStack()

	// Chat doesn't use an index pattern
	if m.signalType != signalChat {
		m.client.SetIndex(m.signalType.IndexPattern())
		m.displayFields = DefaultFields(m.signalType)
		m.logs = []es.LogEntry{}
		m.selectedIndex = 0
		m.statusMessage = "Auto-detecting time range..."
		m.statusTime = time.Now()
	}

	return m.enterSignalView()
}

// enterSignalView sets the appropriate mode and returns fetch command for current signal
func (m *Model) enterSignalView() tea.Cmd {
	switch m.signalType {
	case signalMetrics:
		m.mode = viewMetricsDashboard
		m.metricsLoading = true
		m.metricsCursor = 0
		m.loading = false
		return tea.Batch(m.autoDetectLookback(), m.fetchAggregatedMetrics())
	case signalTraces:
		m.mode = viewTraceNames
		m.traceViewLevel = traceViewNames
		m.tracesLoading = true
		m.traceNamesCursor = 0
		m.selectedTxName = ""
		m.selectedTraceID = ""
		m.loading = false
		return tea.Batch(m.autoDetectLookback(), m.fetchTransactionNames())
	case signalChat:
		m.mode = viewChat
		m.loading = false
		_, cmd := m.enterChatView()
		return cmd
	default:
		m.mode = viewLogs
		m.loading = true
		return m.autoDetectLookback()
	}
}

// cyclePerspective switches perspective type and enters perspective list view
func (m *Model) cyclePerspective() tea.Cmd {
	switch m.currentPerspective {
	case PerspectiveServices:
		m.currentPerspective = PerspectiveResources
	case PerspectiveResources:
		m.currentPerspective = PerspectiveServices
	}
	return m.enterPerspectiveView()
}

// enterPerspectiveView sets up the perspective list view
func (m *Model) enterPerspectiveView() tea.Cmd {
	m.pushView(viewPerspectiveList)
	m.perspectiveCursor = 0
	m.perspectiveItems = []PerspectiveItem{}
	m.perspectiveLoading = true
	m.statusMessage = fmt.Sprintf("Loading %s...", m.currentPerspective.String())
	m.statusTime = time.Now()
	return m.fetchPerspectiveData()
}

// cycleLookback advances to the next lookback duration
func (m *Model) cycleLookback() {
	for i, lb := range lookbackDurations {
		if lb == m.lookback {
			m.lookback = lookbackDurations[(i+1)%len(lookbackDurations)]
			return
		}
	}
}

// copyToClipboard copies text and sets status message
func (m *Model) copyToClipboard(text, successMsg string) {
	if err := clipboard.Init(); err != nil {
		m.statusMessage = "Clipboard error: " + err.Error()
	} else {
		clipboard.Write(clipboard.FmtText, []byte(text))
		m.statusMessage = successMsg
	}
	m.statusTime = time.Now()
}

// === View Category Helpers ===

// isBaseView returns true for signal root views that don't pop anywhere.
func (m Model) isBaseView() bool {
	switch m.mode {
	case viewLogs, viewMetricsDashboard, viewTraceNames, viewChat:
		return true
	}
	return false
}

// isModalView returns true for overlay views (error, help, quit, etc).
func (m Model) isModalView() bool {
	switch m.mode {
	case viewErrorModal, viewQuitConfirm, viewHelp, viewCredsModal,
		viewOtelConfigExplain, viewOtelConfigModal:
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
	switch m.signalType {
	case signalLogs:
		m.loading = true
		return m.fetchLogs()
	case signalMetrics:
		m.metricsLoading = true
		return m.fetchAggregatedMetrics()
	case signalTraces:
		m.tracesLoading = true
		return m.fetchTransactionNames()
	}
	return nil
}
