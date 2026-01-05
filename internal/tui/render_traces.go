// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderTransactionNames(listHeight int) string {
	if m.tracesLoading {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			LoadingStyle.Render("Loading transaction names..."))
	}

	if m.err != nil {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	if len(m.transactionNames) == 0 {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			LoadingStyle.Render("No transactions found in the selected time range."))
	}

	// Calculate column widths
	// TRANSACTION NAME (flex) | COUNT (10) | MIN(ms) (10) | AVG(ms) (10) | MAX(ms) (10) | TRACES (8) | SPANS (8) | ERR% (8)
	countWidth := 10
	minWidth := 10
	avgWidth := 10
	maxWidth := 10
	tracesWidth := 8
	spansWidth := 8
	errWidth := 8
	fixedWidth := countWidth + minWidth + avgWidth + maxWidth + tracesWidth + spansWidth + errWidth + 7 // separators
	nameWidth := m.width - fixedWidth - 10
	if nameWidth < 20 {
		nameWidth = 20
	}

	// Header
	header := HeaderRowStyle.Render(
		PadOrTruncate("TRANSACTION NAME", nameWidth) + " " +
			PadOrTruncate("COUNT", countWidth) + " " +
			PadOrTruncate("MIN(ms)", minWidth) + " " +
			PadOrTruncate("AVG(ms)", avgWidth) + " " +
			PadOrTruncate("MAX(ms)", maxWidth) + " " +
			PadOrTruncate("TRACES", tracesWidth) + " " +
			PadOrTruncate("SPANS", spansWidth) + " " +
			PadOrTruncate("ERR%", errWidth))

	// Calculate visible range using common helper
	startIdx, endIdx := calcVisibleRange(m.traceNamesCursor, len(m.transactionNames), listHeight)

	var lines []string
	lines = append(lines, header)

	for i := startIdx; i < endIdx; i++ {
		tx := m.transactionNames[i]
		selected := i == m.traceNamesCursor

		// Format values
		countStr := fmt.Sprintf("%d", tx.Count)
		minStr := fmt.Sprintf("%.2f", tx.MinDuration)
		avgStr := fmt.Sprintf("%.2f", tx.AvgDuration)
		maxStr := fmt.Sprintf("%.2f", tx.MaxDuration)
		tracesStr := fmt.Sprintf("%d", tx.TraceCount)
		spansStr := fmt.Sprintf("%.1f", tx.AvgSpans)
		errStr := fmt.Sprintf("%.1f%%", tx.ErrorRate)

		line := PadOrTruncate(tx.Name, nameWidth) + " " +
			PadOrTruncate(countStr, countWidth) + " " +
			PadOrTruncate(minStr, minWidth) + " " +
			PadOrTruncate(avgStr, avgWidth) + " " +
			PadOrTruncate(maxStr, maxWidth) + " " +
			PadOrTruncate(tracesStr, tracesWidth) + " " +
			PadOrTruncate(spansStr, spansWidth) + " " +
			PadOrTruncate(errStr, errWidth)

		if selected {
			lines = append(lines, SelectedLogStyle.Width(m.width-6).Render(line))
		} else {
			lines = append(lines, LogEntryStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.width - 4).Height(listHeight).Render(content)
}

