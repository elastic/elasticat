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
	// TRANSACTION NAME (flex) | COUNT (8) | MIN(ms) (9) | AVG(ms) (9) | MAX(ms) (9) | TRACES (7) | SPANS (6) | ERR% (6) | LAST SEEN (10)
	countWidth := 8
	minWidth := 9
	avgWidth := 9
	maxWidth := 9
	tracesWidth := 7
	spansWidth := 6
	errWidth := 6
	lastSeenWidth := 10
	fixedWidth := countWidth + minWidth + avgWidth + maxWidth + tracesWidth + spansWidth + errWidth + lastSeenWidth + 8 // separators
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
			PadOrTruncate("ERR%", errWidth) + " " +
			PadOrTruncate("LAST SEEN", lastSeenWidth))

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

		// Format last seen
		lastSeenStr := "-"
		if !tx.LastSeen.IsZero() {
			lastSeenStr = formatRelativeTime(tx.LastSeen)
		}

		line := PadOrTruncate(tx.Name, nameWidth) + " " +
			PadOrTruncate(countStr, countWidth) + " " +
			PadOrTruncate(minStr, minWidth) + " " +
			PadOrTruncate(avgStr, avgWidth) + " " +
			PadOrTruncate(maxStr, maxWidth) + " " +
			PadOrTruncate(tracesStr, tracesWidth) + " " +
			PadOrTruncate(spansStr, spansWidth) + " " +
			PadOrTruncate(errStr, errWidth) + " " +
			PadOrTruncate(lastSeenStr, lastSeenWidth)

		if selected {
			lines = append(lines, SelectedLogStyle.Width(m.width-6).Render(line))
		} else {
			lines = append(lines, LogEntryStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.width - 4).Height(listHeight).Render(content)
}
