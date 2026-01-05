// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
		// Previous metric
		if m.metricsCursor > 0 {
			m.metricsCursor--
		}
	case "right", "l":
		// Next metric
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics)-1 {
			m.metricsCursor++
		}
	case "r":
		// Refresh
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	}

	return m, nil
}
