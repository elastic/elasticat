// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderPerspectiveList(listHeight int) string {
	if m.Perspective.Loading {
		return LogListStyle.Width(m.UI.Width - 4).Height(listHeight).Render(
			LoadingStyle.Render(fmt.Sprintf("Loading %s...", strings.ToLower(m.Perspective.Current.String()))))
	}

	if m.UI.Err != nil {
		return LogListStyle.Width(m.UI.Width - 4).Height(listHeight).Render(
			ErrorStyle.Render(fmt.Sprintf("Error: %v", m.UI.Err)))
	}

	if len(m.Perspective.Items) == 0 {
		return LogListStyle.Width(m.UI.Width - 4).Height(listHeight).Render(
			LoadingStyle.Render(fmt.Sprintf("No %s found in the selected time range.", strings.ToLower(m.Perspective.Current.String()))))
	}

	// Calculate column widths
	// NAME (flex) | LOGS (10) | TRACES (10) | METRICS (10)
	logsWidth := 10
	tracesWidth := 10
	metricsWidth := 10
	fixedWidth := logsWidth + tracesWidth + metricsWidth + 4 // separators
	nameWidth := m.UI.Width - fixedWidth - 10
	if nameWidth < 20 {
		nameWidth = 20
	}

	// Header
	headerLabel := strings.ToUpper(m.Perspective.Current.String()[:len(m.Perspective.Current.String())-1]) // Remove trailing 's'
	header := HeaderRowStyle.Render(
		PadOrTruncate(headerLabel, nameWidth) + " " +
			PadOrTruncate("LOGS", logsWidth) + " " +
			PadOrTruncate("TRACES", tracesWidth) + " " +
			PadOrTruncate("METRICS", metricsWidth))

	// Calculate visible range using common helper
	startIdx, endIdx := calcVisibleRange(m.Perspective.Cursor, len(m.Perspective.Items), listHeight)

	var lines []string
	lines = append(lines, header)

	for i := startIdx; i < endIdx; i++ {
		item := m.Perspective.Items[i]
		selected := i == m.Perspective.Cursor

		// Check if this item is currently active as a filter (include or exclude)
		isIncluded := false
		isExcluded := false
		if m.Perspective.Current == PerspectiveServices && m.Filters.Service == item.Name {
			if m.Filters.NegateService {
				isExcluded = true
			} else {
				isIncluded = true
			}
		} else if m.Perspective.Current == PerspectiveResources && m.Filters.Resource == item.Name {
			if m.Filters.NegateResource {
				isExcluded = true
			} else {
				isIncluded = true
			}
		}

		// Format values
		logsStr := fmt.Sprintf("%d", item.LogCount)
		tracesStr := fmt.Sprintf("%d", item.TraceCount)
		metricsStr := fmt.Sprintf("%d", item.MetricCount)

		// Add active marker: ✓ for include, - for exclude
		nameDisplay := item.Name
		if isIncluded {
			nameDisplay = "✓ " + item.Name
		} else if isExcluded {
			nameDisplay = "- " + item.Name
		}

		line := PadOrTruncate(nameDisplay, nameWidth) + " " +
			PadOrTruncate(logsStr, logsWidth) + " " +
			PadOrTruncate(tracesStr, tracesWidth) + " " +
			PadOrTruncate(metricsStr, metricsWidth)

		if selected {
			lines = append(lines, SelectedLogStyle.Width(m.UI.Width-6).Render(line))
		} else {
			lines = append(lines, LogEntryStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.UI.Width - 4).Height(listHeight).Render(content)
}
