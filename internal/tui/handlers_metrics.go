// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
)

func (m Model) handleMetricsDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	// Handle common actions first (signal cycle, lookback, perspective, kibana)
	if newM, cmd, handled := m.handleCommonAction(action); handled {
		return newM, cmd
	}

	// Calculate list length for navigation
	listLen := 0
	if m.Metrics.Aggregated != nil {
		listLen = len(m.Metrics.Aggregated.Metrics)
	}

	// Handle list navigation
	if isNavKey(key) {
		m.Metrics.Cursor = listNav(m.Metrics.Cursor, listLen, key)
		return m, nil
	}

	switch action {
	case ActionBack:
		// Metrics dashboard is a base view - esc does nothing (user can press 'q' to quit)
		return m, nil
	case ActionSelect:
		// Enter detail view for the selected metric
		if m.Metrics.Aggregated != nil && m.Metrics.Cursor < len(m.Metrics.Aggregated.Metrics) {
			m.pushView(viewMetricDetail)
			m.Metrics.DetailDocCursor = 0
			m.Metrics.DetailDocsLoading = true
			m.updateMetricDetailViewport() // Initialize viewport with current content
			return m, m.fetchMetricDetailDocs()
		}
	case ActionRefresh:
		m.Metrics.Loading = true
		return m, m.fetchAggregatedMetrics()
	case ActionSearch:
		m.pushView(viewSearch)
		m.Components.SearchInput.Focus()
		return m, textinput.Blink
	case ActionQuit:
		return m, tea.Quit
		// NOTE: ActionCycleLookback, ActionCycleSignal, ActionPerspective, ActionKibana
		// are now handled by handleCommonAction() above
	}

	// View-specific keys
	switch key {
	case "d":
		// Switch to document view
		m.Metrics.ViewMode = metricsViewDocuments
		m.UI.Mode = viewLogs
		m.UI.Loading = true
		return m, m.fetchLogs()
	}

	return m, nil
}

func (m Model) handleMetricDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	switch action {
	case ActionBack, ActionQuit:
		// Return to metrics dashboard
		m.popView()
		return m, nil
	case ActionPrevItem:
		// Previous metric (and re-fetch docs)
		if m.Metrics.Cursor > 0 {
			m.Metrics.Cursor--
			m.Metrics.DetailDocCursor = 0
			m.Metrics.DetailDocsLoading = true
			m.updateMetricDetailViewport()
			return m, m.fetchMetricDetailDocs()
		}
	case ActionNextItem:
		// Next metric (and re-fetch docs)
		if m.Metrics.Aggregated != nil && m.Metrics.Cursor < len(m.Metrics.Aggregated.Metrics)-1 {
			m.Metrics.Cursor++
			m.Metrics.DetailDocCursor = 0
			m.Metrics.DetailDocsLoading = true
			m.updateMetricDetailViewport()
			return m, m.fetchMetricDetailDocs()
		}
	case ActionPrevDoc:
		// Previous doc (N)
		if m.Metrics.DetailDocCursor > 0 {
			m.Metrics.DetailDocCursor--
			m.updateMetricDetailViewport()
		}
		return m, nil
	case ActionNextDoc:
		// Next doc (n)
		if m.Metrics.DetailDocCursor < len(m.Metrics.DetailDocs)-1 {
			m.Metrics.DetailDocCursor++
			m.updateMetricDetailViewport()
		}
		return m, nil
	case ActionCopy:
		// Copy current doc JSON to clipboard
		if len(m.Metrics.DetailDocs) > 0 && m.Metrics.DetailDocCursor < len(m.Metrics.DetailDocs) {
			m.copyToClipboard(es.PrettyJSON(m.Metrics.DetailDocs[m.Metrics.DetailDocCursor].RawJSON), "Copied JSON to clipboard!")
		}
		return m, nil
	case ActionRefresh:
		// Refresh
		m.Metrics.Loading = true
		m.Metrics.DetailDocsLoading = true
		return m, tea.Batch(m.fetchAggregatedMetrics(), m.fetchMetricDetailDocs())
	case ActionCycleLookback:
		// Change lookback - re-fetch metrics with new time range
		m.cycleLookback()
		m.Metrics.Loading = true
		m.Metrics.DetailDocsLoading = true
		return m, tea.Batch(m.fetchAggregatedMetrics(), m.fetchMetricDetailDocs())
	case ActionKibana:
		// Prepare Kibana URL for this specific metric and show creds modal
		if m.Metrics.Aggregated != nil && m.Metrics.Cursor < len(m.Metrics.Aggregated.Metrics) {
			metric := m.Metrics.Aggregated.Metrics[m.Metrics.Cursor]
			// metric.Type contains the time series type: "gauge", "counter", or "histogram"
			m.prepareMetricKibanaURL(metric.Name, metric.Type)
			m.showCredsModal()
		}
		return m, nil
	case ActionJSON:
		// View current doc as JSON (switch to detail JSON view)
		if len(m.Metrics.DetailDocs) > 0 && m.Metrics.DetailDocCursor < len(m.Metrics.DetailDocs) {
			// Temporarily put the doc in logs so detail view can render it
			m.Logs.Entries = m.Metrics.DetailDocs
			m.Logs.SelectedIndex = m.Metrics.DetailDocCursor
			m.pushView(viewDetailJSON)
			m.setViewportContent(es.PrettyJSON(m.Metrics.DetailDocs[m.Metrics.DetailDocCursor].RawJSON))
			m.Components.Viewport.GotoTop()
		}
		return m, nil
	}

	// Handle viewport scrolling for unhandled keys
	if viewportScroll(&m.Components.Viewport, key) {
		return m, nil
	}

	return m, nil
}

// updateMetricDetailViewport refreshes the viewport content for the current metric/doc
func (m *Model) updateMetricDetailViewport() {
	content := m.renderMetricDetailContent()
	m.setViewportContent(content)
	m.Components.Viewport.GotoTop()
}
