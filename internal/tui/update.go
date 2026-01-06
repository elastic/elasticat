// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"time"

	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleAsyncError is a helper that handles the common error pattern in async message handlers.
// If err is nil, it returns (m, false) so the caller can proceed with success handling.
// If err is a context error (canceled/timeout), it returns (m, true) to exit early.
// Otherwise, it sets m.err, shows the error modal, and returns (m, true).
func (m *Model) handleAsyncError(err error) (done bool) {
	if err == nil {
		return false
	}
	if isContextError(err) {
		return true
	}
	m.err = err
	m.showErrorModal()
	return true
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.autoDetectLookback(),
		m.tickCmd(),
		func() tea.Msg { return tea.EnableMouseCellMotion() },
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case logsMsg:
		return m.handleLogsMsg(msg)

	case tickMsg:
		return m.handleTickMsg()

	case fieldCapsMsg:
		return m.handleFieldCapsMsg(msg)

	case autoDetectMsg:
		return m.handleAutoDetectMsg(msg)

	case metricsAggMsg:
		return m.handleMetricsAggMsg(msg)

	case metricDetailDocsMsg:
		return m.handleMetricDetailDocsMsg(msg)

	case transactionNamesMsg:
		return m.handleTransactionNamesMsg(msg)

	case spansMsg:
		return m.handleSpansMsg(msg)

	case perspectiveDataMsg:
		return m.handlePerspectiveDataMsg(msg)

	case errMsg:
		return m.handleErrMsg(msg)
	}

	var cmds []tea.Cmd

	// Update components based on mode
	switch m.mode {
	case viewSearch:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewIndex:
		var cmd tea.Cmd
		m.indexInput, cmd = m.indexInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case viewErrorModal:
		var cmd tea.Cmd
		m.errorViewport, cmd = m.errorViewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// showErrorModal configures the error viewport and switches to error modal view
func (m *Model) showErrorModal() {
	m.pushView(viewErrorModal)
	// Set up error viewport dimensions
	modalWidth := min(m.width-8, 80)
	m.errorViewport.Width = modalWidth - 8        // Account for border + padding + margin
	m.errorViewport.Height = min(m.height-15, 20) // Leave room for title/actions
	if m.err != nil {
		m.errorViewport.SetContent(m.err.Error())
	}
	m.errorViewport.GotoTop()
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.viewport.Width = msg.Width - 4
	m.viewport.Height = msg.Height - 10

	// Re-wrap detail content after resize so long fields wrap correctly.
	if m.mode == viewDetail || m.mode == viewDetailJSON {
		(&m).updateDetailContent()
	}

	return m, nil
}

func (m Model) handleLogsMsg(msg logsMsg) (Model, tea.Cmd) {
	m.loading = false
	m.lastRefresh = time.Now()
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.logs = msg.logs
	m.total = msg.total
	m.err = nil
	m.lastQueryJSON = msg.queryJSON
	m.lastQueryIndex = msg.index

	m = m.applyTailBehaviorAfterFetch()
	m = m.clampSelection()

	return m.maybeTriggerPostLoadFetches()
}

func (m Model) handleTickMsg() (Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.autoRefresh && m.mode == viewLogs {
		cmds = append(cmds, m.fetchLogs())
	}
	cmds = append(cmds, m.tickCmd())
	return m, tea.Batch(cmds...)
}

func (m Model) handleFieldCapsMsg(msg fieldCapsMsg) (Model, tea.Cmd) {
	m.fieldsLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.availableFields = msg.fields
	return m, nil
}

func (m Model) handleAutoDetectMsg(msg autoDetectMsg) (Model, tea.Cmd) {
	// Auto-detect has special error handling: on failure, use current lookback
	if msg.err != nil {
		if isContextError(msg.err) {
			return m, nil
		}
		// Auto-detect failed, just use current lookback and fetch
		return m.startInitialFetch()
	}

	m.lookback = msg.lookback
	m.statusMessage = fmt.Sprintf("Found %d entries in %s", msg.total, msg.lookback.String())
	m.statusTime = time.Now()

	return m.startInitialFetch()
}

// startInitialFetch kicks off the appropriate fetch based on signal type.
func (m Model) startInitialFetch() (Model, tea.Cmd) {
	switch m.signalType {
	case signalMetrics:
		if m.metricsViewMode == metricsViewAggregated {
			m.metricsLoading = true
			return m, m.fetchAggregatedMetrics()
		}
	case signalTraces:
		if m.traceViewLevel == traceViewNames {
			m.tracesLoading = true
			return m, m.fetchTransactionNames()
		}
	}
	m.loading = true
	return m, m.fetchLogs()
}

func (m Model) handleMetricsAggMsg(msg metricsAggMsg) (Model, tea.Cmd) {
	m.metricsLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.aggregatedMetrics = msg.result
	// Store the ES|QL query for Kibana integration
	if msg.result != nil && msg.result.Query != "" {
		m.lastQueryJSON = msg.result.Query
		m.lastQueryIndex = m.client.GetIndex()
	}
	m.err = nil
	return m, nil
}

func (m Model) handleMetricDetailDocsMsg(msg metricDetailDocsMsg) (Model, tea.Cmd) {
	m.metricDetailDocsLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.metricDetailDocs = msg.docs
	m.metricDetailDocCursor = 0
	m.err = nil
	return m, nil
}

func (m Model) handleTransactionNamesMsg(msg transactionNamesMsg) (Model, tea.Cmd) {
	m.tracesLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.transactionNames = msg.names
	m.err = nil
	return m, nil
}

func (m Model) handleSpansMsg(msg spansMsg) (Model, tea.Cmd) {
	m.spansLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.spans = msg.spans
	m.err = nil
	return m, nil
}

func (m Model) handlePerspectiveDataMsg(msg perspectiveDataMsg) (Model, tea.Cmd) {
	m.perspectiveLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.perspectiveItems = msg.items
	m.err = nil
	m.statusMessage = fmt.Sprintf("Loaded %d %s", len(msg.items), m.currentPerspective.String())
	m.statusTime = time.Now()
	return m, nil
}

func (m Model) handleErrMsg(msg errMsg) (Model, tea.Cmd) {
	m.err = msg
	m.loading = false
	m.showErrorModal()
	return m, nil
}

// applyTailBehaviorAfterFetch auto-selects the newest log when the user has not scrolled.
func (m Model) applyTailBehaviorAfterFetch() Model {
	if m.userHasScrolled || len(m.logs) == 0 {
		return m
	}

	if m.sortAscending {
		m.selectedIndex = len(m.logs) - 1
	} else {
		m.selectedIndex = 0
	}
	return m
}

// clampSelection keeps the selection within bounds.
func (m Model) clampSelection() Model {
	if len(m.logs) == 0 {
		m.selectedIndex = 0
		return m
	}
	if m.selectedIndex >= len(m.logs) {
		m.selectedIndex = len(m.logs) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	return m
}

// maybeTriggerPostLoadFetches runs any follow-up fetches after logs load (e.g., spans).
func (m Model) maybeTriggerPostLoadFetches() (Model, tea.Cmd) {
	if m.signalType != signalTraces {
		m.lastFetchedTraceID = ""
		return m, nil
	}
	return m, m.maybeFetchSpansForSelection()
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
