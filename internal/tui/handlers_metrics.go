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

	// Calculate list length for navigation
	listLen := 0
	if m.aggregatedMetrics != nil {
		listLen = len(m.aggregatedMetrics.Metrics)
	}

	// Handle list navigation
	if isNavKey(key) {
		m.metricsCursor = listNav(m.metricsCursor, listLen, key)
		return m, nil
	}

	switch action {
	case ActionSelect:
		// Enter detail view for the selected metric
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
			m.pushView(viewMetricDetail)
			m.metricDetailDocCursor = 0
			m.metricDetailDocsLoading = true
			m.updateMetricDetailViewport() // Initialize viewport with current content
			return m, m.fetchMetricDetailDocs()
		}
	case ActionRefresh:
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	case ActionPerspective:
		return m, m.cyclePerspective()
	case ActionCycleLookback:
		m.cycleLookback()
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	case ActionCycleSignal:
		return m, m.cycleSignalType()
	case ActionSearch:
		m.pushView(viewSearch)
		m.searchInput.Focus()
		return m, textinput.Blink
	case ActionKibana:
		// Prepare Kibana URL and show creds modal (user presses enter to open browser)
		if m.prepareKibanaURL() {
			m.showCredsModal()
		}
		return m, nil
	case ActionQuit:
		return m, tea.Quit
	}

	// View-specific keys
	switch key {
	case "d":
		// Switch to document view
		m.metricsViewMode = metricsViewDocuments
		m.mode = viewLogs
		m.loading = true
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
		if m.metricsCursor > 0 {
			m.metricsCursor--
			m.metricDetailDocCursor = 0
			m.metricDetailDocsLoading = true
			m.updateMetricDetailViewport()
			return m, m.fetchMetricDetailDocs()
		}
	case ActionNextItem:
		// Next metric (and re-fetch docs)
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics)-1 {
			m.metricsCursor++
			m.metricDetailDocCursor = 0
			m.metricDetailDocsLoading = true
			m.updateMetricDetailViewport()
			return m, m.fetchMetricDetailDocs()
		}
	case ActionPrevDoc:
		// Previous doc (N)
		if m.metricDetailDocCursor > 0 {
			m.metricDetailDocCursor--
			m.updateMetricDetailViewport()
		}
		return m, nil
	case ActionNextDoc:
		// Next doc (n)
		if m.metricDetailDocCursor < len(m.metricDetailDocs)-1 {
			m.metricDetailDocCursor++
			m.updateMetricDetailViewport()
		}
		return m, nil
	case ActionCopy:
		// Copy current doc JSON to clipboard
		if len(m.metricDetailDocs) > 0 && m.metricDetailDocCursor < len(m.metricDetailDocs) {
			m.copyToClipboard(es.PrettyJSON(m.metricDetailDocs[m.metricDetailDocCursor].RawJSON), "Copied JSON to clipboard!")
		}
		return m, nil
	case ActionRefresh:
		// Refresh
		m.metricsLoading = true
		m.metricDetailDocsLoading = true
		return m, tea.Batch(m.fetchAggregatedMetrics(), m.fetchMetricDetailDocs())
	case ActionCycleLookback:
		// Change lookback - re-fetch metrics with new time range
		m.cycleLookback()
		m.metricsLoading = true
		m.metricDetailDocsLoading = true
		return m, tea.Batch(m.fetchAggregatedMetrics(), m.fetchMetricDetailDocs())
	case ActionKibana:
		// Prepare Kibana URL for this specific metric and show creds modal
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
			metric := m.aggregatedMetrics.Metrics[m.metricsCursor]
			// metric.Type contains the time series type: "gauge", "counter", or "histogram"
			m.prepareMetricKibanaURL(metric.Name, metric.Type)
			m.showCredsModal()
		}
		return m, nil
	case ActionJSON:
		// View current doc as JSON (switch to detail JSON view)
		if len(m.metricDetailDocs) > 0 && m.metricDetailDocCursor < len(m.metricDetailDocs) {
			// Temporarily put the doc in logs so detail view can render it
			m.logs = m.metricDetailDocs
			m.selectedIndex = m.metricDetailDocCursor
			m.pushView(viewDetailJSON)
			m.setViewportContent(es.PrettyJSON(m.metricDetailDocs[m.metricDetailDocCursor].RawJSON))
			m.viewport.GotoTop()
		}
		return m, nil
	}

	// Handle viewport scrolling for unhandled keys
	if viewportScroll(&m.viewport, key) {
		return m, nil
	}

	return m, nil
}

// updateMetricDetailViewport refreshes the viewport content for the current metric/doc
func (m *Model) updateMetricDetailViewport() {
	content := m.renderMetricDetailContent()
	m.setViewportContent(content)
	m.viewport.GotoTop()
}
