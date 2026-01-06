// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
	"golang.design/x/clipboard"
)

// listNav handles standard list navigation, returning the new cursor position.
// cursor: current position, listLen: total items, key: the pressed key.
// Returns -1 if the key is not a navigation key.
func listNav(cursor, listLen int, key string) int {
	switch key {
	case "up", "k":
		if cursor > 0 {
			return cursor - 1
		}
		return cursor
	case "down", "j":
		if cursor < listLen-1 {
			return cursor + 1
		}
		return cursor
	case "home", "g":
		return 0
	case "end", "G":
		if listLen > 0 {
			return listLen - 1
		}
		return 0
	case "pgup":
		newCursor := cursor - 10
		if newCursor < 0 {
			return 0
		}
		return newCursor
	case "pgdown":
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
	switch key {
	case "up", "k", "down", "j", "home", "g", "end", "G", "pgup", "pgdown":
		return true
	}
	return false
}

// cycleSignalType switches to the next signal type and returns the appropriate command
func (m *Model) cycleSignalType() tea.Cmd {
	// Cycle: logs -> traces -> metrics -> logs
	switch m.signalType {
	case signalLogs:
		m.signalType = signalTraces
	case signalTraces:
		m.signalType = signalMetrics
	case signalMetrics:
		m.signalType = signalLogs
	}

	m.client.SetIndex(m.signalType.IndexPattern())
	m.displayFields = DefaultFields(m.signalType)
	m.logs = []es.LogEntry{}
	m.selectedIndex = 0
	m.statusMessage = "Auto-detecting time range..."
	m.statusTime = time.Now()

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
	m.mode = viewPerspectiveList
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
