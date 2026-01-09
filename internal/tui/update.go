// Copyright 2026 Elasticsearch B.V. and contributors
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
// Otherwise, it sets m.UI.Err, shows the error modal, and returns (m, true).
func (m *Model) handleAsyncError(err error) (done bool) {
	if err == nil {
		return false
	}
	if isContextError(err) {
		return true
	}
	m.UI.Err = err
	m.showErrorModal()
	return true
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.tickCmd(),
		func() tea.Msg { return tea.EnableMouseCellMotion() },
	}

	// Chat mode doesn't need auto-detect or data fetching
	if m.Filters.Signal == signalChat {
		var cmd tea.Cmd
		m, cmd = m.enterChatView()
		cmds = append(cmds, cmd)
	} else {
		cmds = append(cmds, m.autoDetectLookback())
	}

	return tea.Batch(cmds...)
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

	case chatResponseMsg:
		return m.handleChatResponseMsg(msg)

	case OtelConfigOpenedMsg:
		if msg.Err != nil {
			m.UI.Err = msg.Err
			m.pushView(viewErrorModal)
			modalWidth := min(m.UI.Width-8, 80)
			m.Components.ErrorViewport.Width = modalWidth - 8
			m.Components.ErrorViewport.Height = min(m.UI.Height-15, 20)
			m.Components.ErrorViewport.SetContent(msg.Err.Error())
			m.Components.ErrorViewport.GotoTop()
			return m, nil
		}
		// Show the OTel config modal and start watching
		m.Otel.ConfigPath = msg.ConfigPath
		m.Otel.WatchingConfig = true
		m.Otel.ReloadError = nil
		m.pushView(viewOtelConfigModal)

		// If extracted, we need a restart - don't hot reload
		if msg.Extracted {
			m.Otel.WatchingConfig = false
			m.UI.StatusMessage = "Config extracted. Restart stack to use file mount."
			m.UI.StatusTime = time.Now()
			return m, nil
		}

		// Start watching for file changes
		return m, watchOtelConfig(msg.ConfigPath)

	case otelFileChangedMsg:
		// File changed - validate first, then reload if valid
		m.Otel.ValidationStatus = "Validating config..."
		m.Otel.ValidationValid = true // Clear any previous error styling
		return m, m.handleOtelFileChanged()

	case otelValidatedMsg:
		// Validation complete - update status and proceed if valid
		return m, m.handleOtelValidated(msg.Valid, msg.Message)

	case otelReloadedMsg:
		if msg.Err != nil {
			m.Otel.ReloadError = msg.Err
		} else {
			m.Otel.LastReload = msg.Time
			m.Otel.ReloadCount++
			m.Otel.ReloadError = nil
			// Clear validation status on successful reload
			m.Otel.ValidationStatus = ""
		}
		// Continue watching if still in the modal
		if m.UI.Mode == viewOtelConfigModal && m.Otel.WatchingConfig {
			return m, watchOtelConfig(m.Otel.ConfigPath)
		}
		return m, nil

	case otelWatcherErrorMsg:
		m.Otel.ReloadError = msg.Err
		m.Otel.WatchingConfig = false
		return m, nil

	case errMsg:
		return m.handleErrMsg(msg)
	}

	var cmds []tea.Cmd

	// Update components based on mode
	switch m.UI.Mode {
	case viewSearch:
		var cmd tea.Cmd
		m.Components.SearchInput, cmd = m.Components.SearchInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewIndex:
		var cmd tea.Cmd
		m.Components.IndexInput, cmd = m.Components.IndexInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewDetail:
		var cmd tea.Cmd
		m.Components.Viewport, cmd = m.Components.Viewport.Update(msg)
		cmds = append(cmds, cmd)
	case viewErrorModal:
		var cmd tea.Cmd
		m.Components.ErrorViewport, cmd = m.Components.ErrorViewport.Update(msg)
		cmds = append(cmds, cmd)
	case viewChat:
		var cmd tea.Cmd
		m.Chat.Input, cmd = m.Chat.Input.Update(msg)
		cmds = append(cmds, cmd)
		var vpCmd tea.Cmd
		m.Chat.Viewport, vpCmd = m.Chat.Viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

// showErrorModal configures the error viewport and switches to error modal view
func (m *Model) showErrorModal() {
	m.pushView(viewErrorModal)
	// Set up error viewport dimensions
	modalWidth := min(m.UI.Width-8, 80)
	m.Components.ErrorViewport.Width = modalWidth - 8           // Account for border + padding + margin
	m.Components.ErrorViewport.Height = min(m.UI.Height-15, 20) // Leave room for title/actions
	if m.UI.Err != nil {
		m.Components.ErrorViewport.SetContent(m.UI.Err.Error())
	}
	m.Components.ErrorViewport.GotoTop()
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.UI.Width = msg.Width
	m.UI.Height = msg.Height
	m.Components.Viewport.Width = msg.Width - 4
	// DetailStyle has border (2) + padding (2) = 4 lines overhead inside the box
	// getFullScreenHeight() returns the total height for the DetailStyle container
	// So interior content area = getFullScreenHeight() - 4
	m.Components.Viewport.Height = m.getFullScreenHeight() - 4
	m.Chat.Viewport.Width = msg.Width - 4
	m.Chat.Viewport.Height = msg.Height - 12 // Leave room for input and header

	// Re-wrap detail content after resize so long fields wrap correctly.
	if m.UI.Mode == viewDetail || m.UI.Mode == viewDetailJSON {
		(&m).updateDetailContent()
	}

	// Re-wrap metric detail content after resize
	if m.UI.Mode == viewMetricDetail {
		(&m).updateMetricDetailViewport()
	}

	// Re-wrap chat content after resize
	if m.UI.Mode == viewChat {
		(&m).updateChatViewport()
	}

	return m, nil
}

func (m Model) handleLogsMsg(msg logsMsg) (Model, tea.Cmd) {
	m.UI.Loading = false
	m.UI.LastRefresh = time.Now()
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.Logs.Entries = msg.logs
	m.Logs.Total = msg.total
	m.UI.Err = nil
	m.Query.LastJSON = msg.queryJSON
	m.Query.LastIndex = msg.index

	m = m.applyTailBehaviorAfterFetch()
	m = m.clampSelection()

	return m.maybeTriggerPostLoadFetches()
}

func (m Model) handleTickMsg() (Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.UI.AutoRefresh && m.UI.Mode == viewLogs {
		cmds = append(cmds, m.fetchLogs())
	}
	cmds = append(cmds, m.tickCmd())
	return m, tea.Batch(cmds...)
}

func (m Model) handleFieldCapsMsg(msg fieldCapsMsg) (Model, tea.Cmd) {
	m.Fields.Loading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.Fields.Available = msg.fields
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

	m.Filters.Lookback = msg.lookback
	m.UI.StatusMessage = fmt.Sprintf("Found %d entries in %s", msg.total, msg.lookback.String())
	m.UI.StatusTime = time.Now()

	return m.startInitialFetch()
}

// startInitialFetch kicks off the appropriate fetch based on signal type.
func (m Model) startInitialFetch() (Model, tea.Cmd) {
	switch m.Filters.Signal {
	case signalMetrics:
		if m.Metrics.ViewMode == metricsViewAggregated {
			m.Metrics.Loading = true
			return m, m.fetchAggregatedMetrics()
		}
	case signalTraces:
		if m.Traces.ViewLevel == traceViewNames {
			m.Traces.Loading = true
			return m, m.fetchTransactionNames()
		}
	}
	m.UI.Loading = true
	return m, m.fetchLogs()
}

func (m Model) handleMetricsAggMsg(msg metricsAggMsg) (Model, tea.Cmd) {
	m.Metrics.Loading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.Metrics.Aggregated = msg.result
	// Store the ES|QL query for Kibana integration
	if msg.result != nil && msg.result.Query != "" {
		m.Query.LastJSON = msg.result.Query
		m.Query.LastIndex = m.client.GetIndex()
	}
	m.UI.Err = nil
	return m, nil
}

func (m Model) handleMetricDetailDocsMsg(msg metricDetailDocsMsg) (Model, tea.Cmd) {
	m.Metrics.DetailDocsLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.Metrics.DetailDocs = msg.docs
	m.Metrics.DetailDocCursor = 0
	m.UI.Err = nil

	// Refresh viewport with updated content
	if m.UI.Mode == viewMetricDetail {
		m.updateMetricDetailViewport()
	}
	return m, nil
}

func (m Model) handleTransactionNamesMsg(msg transactionNamesMsg) (Model, tea.Cmd) {
	m.Traces.Loading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.Traces.TransactionNames = msg.names
	// Store the ES|QL query for display and chat context
	if msg.query != "" {
		m.Query.LastJSON = msg.query
		m.Query.LastIndex = m.client.GetIndex()
	}
	m.UI.Err = nil
	return m, nil
}

func (m Model) handleSpansMsg(msg spansMsg) (Model, tea.Cmd) {
	m.Traces.SpansLoading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.Traces.Spans = msg.spans
	m.UI.Err = nil
	return m, nil
}

func (m Model) handlePerspectiveDataMsg(msg perspectiveDataMsg) (Model, tea.Cmd) {
	m.Perspective.Loading = false
	if m.handleAsyncError(msg.err) {
		return m, nil
	}

	m.Perspective.Items = msg.items
	m.UI.Err = nil
	m.UI.StatusMessage = fmt.Sprintf("Loaded %d %s", len(msg.items), m.Perspective.Current.String())
	m.UI.StatusTime = time.Now()
	return m, nil
}

func (m Model) handleErrMsg(msg errMsg) (Model, tea.Cmd) {
	m.UI.Err = msg
	m.UI.Loading = false
	m.showErrorModal()
	return m, nil
}

// applyTailBehaviorAfterFetch auto-selects the newest log when the user has not scrolled.
func (m Model) applyTailBehaviorAfterFetch() Model {
	if m.Logs.UserHasScrolled || len(m.Logs.Entries) == 0 {
		return m
	}

	if m.UI.SortAscending {
		m.Logs.SelectedIndex = len(m.Logs.Entries) - 1
	} else {
		m.Logs.SelectedIndex = 0
	}
	return m
}

// clampSelection keeps the selection within bounds.
func (m Model) clampSelection() Model {
	if len(m.Logs.Entries) == 0 {
		m.Logs.SelectedIndex = 0
		return m
	}
	if m.Logs.SelectedIndex >= len(m.Logs.Entries) {
		m.Logs.SelectedIndex = len(m.Logs.Entries) - 1
	}
	if m.Logs.SelectedIndex < 0 {
		m.Logs.SelectedIndex = 0
	}
	return m
}

// maybeTriggerPostLoadFetches runs any follow-up fetches after logs load (e.g., spans).
func (m Model) maybeTriggerPostLoadFetches() (Model, tea.Cmd) {
	if m.Filters.Signal != signalTraces {
		m.Traces.LastFetchedTraceID = ""
		return m, nil
	}
	return m, m.maybeFetchSpansForSelection()
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
