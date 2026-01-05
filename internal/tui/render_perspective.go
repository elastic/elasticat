// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderPerspectiveList(listHeight int) string {
	if m.perspectiveLoading {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			LoadingStyle.Render(fmt.Sprintf("Loading %s...", strings.ToLower(m.currentPerspective.String()))))
	}

	if m.err != nil {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	if len(m.perspectiveItems) == 0 {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			LoadingStyle.Render(fmt.Sprintf("No %s found in the selected time range.", strings.ToLower(m.currentPerspective.String()))))
	}

	// Calculate column widths
	// NAME (flex) | LOGS (10) | TRACES (10) | METRICS (10)
	logsWidth := 10
	tracesWidth := 10
	metricsWidth := 10
	fixedWidth := logsWidth + tracesWidth + metricsWidth + 4 // separators
	nameWidth := m.width - fixedWidth - 10
	if nameWidth < 20 {
		nameWidth = 20
	}

	// Header
	headerLabel := strings.ToUpper(m.currentPerspective.String()[:len(m.currentPerspective.String())-1]) // Remove trailing 's'
	header := HeaderRowStyle.Render(
		PadOrTruncate(headerLabel, nameWidth) + " " +
			PadOrTruncate("LOGS", logsWidth) + " " +
			PadOrTruncate("TRACES", tracesWidth) + " " +
			PadOrTruncate("METRICS", metricsWidth))

	// Calculate visible range using common helper
	startIdx, endIdx := calcVisibleRange(m.perspectiveCursor, len(m.perspectiveItems), listHeight)

	var lines []string
	lines = append(lines, header)

	for i := startIdx; i < endIdx; i++ {
		item := m.perspectiveItems[i]
		selected := i == m.perspectiveCursor

		// Check if this item is currently active as a filter (include or exclude)
		isIncluded := false
		isExcluded := false
		if m.currentPerspective == PerspectiveServices && m.filterService == item.Name {
			if m.negateService {
				isExcluded = true
			} else {
				isIncluded = true
			}
		} else if m.currentPerspective == PerspectiveResources && m.filterResource == item.Name {
			if m.negateResource {
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
			lines = append(lines, SelectedLogStyle.Width(m.width-6).Render(line))
		} else {
			lines = append(lines, LogEntryStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.width - 4).Height(listHeight).Render(content)
}

