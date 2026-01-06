// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
)

func (m Model) handleMetricsDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Calculate list length for navigation
	listLen := 0
	if m.aggregatedMetrics != nil {
		listLen = len(m.aggregatedMetrics.Metrics)
	}

	// Handle list navigation
	if isNavKey(msg.String()) {
		m.metricsCursor = listNav(m.metricsCursor, listLen, msg.String())
		return m, nil
	}

	switch msg.String() {
	case "enter":
		// Enter detail view for the selected metric
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
			m.mode = viewMetricDetail
			m.metricDetailDocCursor = 0
			m.metricDetailDocsLoading = true
			return m, m.fetchMetricDetailDocs()
		}
	case "r":
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	case "d":
		// Switch to document view
		m.metricsViewMode = metricsViewDocuments
		m.mode = viewLogs
		m.loading = true
		return m, m.fetchLogs()
	case "p":
		return m, m.cyclePerspective()
	case "l":
		m.cycleLookback()
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	case "m":
		return m, m.cycleSignalType()
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "K":
		// Open Kibana with a basic metrics query
		m.openInKibana()
		return m, nil
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleMetricDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace", "q":
		// Return to metrics dashboard
		m.mode = viewMetricsDashboard
	case "left":
		// Previous metric (and re-fetch docs)
		if m.metricsCursor > 0 {
			m.metricsCursor--
			m.metricDetailDocCursor = 0
			m.metricDetailDocsLoading = true
			return m, m.fetchMetricDetailDocs()
		}
	case "right":
		// Next metric (and re-fetch docs)
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics)-1 {
			m.metricsCursor++
			m.metricDetailDocCursor = 0
			m.metricDetailDocsLoading = true
			return m, m.fetchMetricDetailDocs()
		}
	case "a":
		// Previous doc
		if m.metricDetailDocCursor > 0 {
			m.metricDetailDocCursor--
		}
	case "d":
		// Next doc
		if m.metricDetailDocCursor < len(m.metricDetailDocs)-1 {
			m.metricDetailDocCursor++
		}
	case "j":
		// View current doc as JSON (switch to detail JSON view)
		if len(m.metricDetailDocs) > 0 && m.metricDetailDocCursor < len(m.metricDetailDocs) {
			// Temporarily put the doc in logs so detail view can render it
			m.logs = m.metricDetailDocs
			m.selectedIndex = m.metricDetailDocCursor
			m.previousMode = viewMetricDetail
			m.mode = viewDetailJSON
			m.setViewportContent(es.PrettyJSON(m.metricDetailDocs[m.metricDetailDocCursor].RawJSON))
			m.viewport.GotoTop()
		}
		return m, nil
	case "y":
		// Copy current doc JSON to clipboard
		if len(m.metricDetailDocs) > 0 && m.metricDetailDocCursor < len(m.metricDetailDocs) {
			m.copyToClipboard(es.PrettyJSON(m.metricDetailDocs[m.metricDetailDocCursor].RawJSON), "Copied JSON to clipboard!")
		}
		return m, nil
	case "r":
		// Refresh
		m.metricsLoading = true
		m.metricDetailDocsLoading = true
		return m, tea.Batch(m.fetchAggregatedMetrics(), m.fetchMetricDetailDocs())
	case "K":
		// Open Kibana with this specific metric
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
			metric := m.aggregatedMetrics.Metrics[m.metricsCursor]
			// metric.Type contains the time series type: "gauge", "counter", or "histogram"
			m.openMetricInKibana(metric.Name, metric.Type)
		}
		return m, nil
	}

	return m, nil
}
