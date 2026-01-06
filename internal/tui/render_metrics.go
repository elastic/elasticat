// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderMetricsDashboard(listHeight int) string {
	if m.metricsLoading {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			LoadingStyle.Render("Loading metrics..."))
	}

	if m.err != nil {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	if m.aggregatedMetrics == nil || len(m.aggregatedMetrics.Metrics) == 0 {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			LoadingStyle.Render("No metrics found. Press 'd' for document view."))
	}

	// Calculate column widths
	// METRIC (flex) | SPARKLINE (20) | MIN (10) | MAX (10) | AVG (10) | LATEST (10)
	sparklineWidth := 20
	numWidth := 10
	fixedWidth := sparklineWidth + (numWidth * 4) + 6 // 6 for separators
	metricWidth := m.width - fixedWidth - 10          // padding
	if metricWidth < 20 {
		metricWidth = 20
	}

	// Header
	header := HeaderRowStyle.Render(
		PadOrTruncate("METRIC", metricWidth) + " " +
			PadOrTruncate("TREND", sparklineWidth) + " " +
			PadOrTruncate("MIN", numWidth) + " " +
			PadOrTruncate("MAX", numWidth) + " " +
			PadOrTruncate("AVG", numWidth) + " " +
			PadOrTruncate("LATEST", numWidth))

	// Calculate visible range using common helper
	metrics := m.aggregatedMetrics.Metrics
	startIdx, endIdx := calcVisibleRange(m.metricsCursor, len(metrics), listHeight)

	var lines []string
	lines = append(lines, header)

	for i := startIdx; i < endIdx; i++ {
		metric := metrics[i]
		selected := i == m.metricsCursor

		// Generate sparkline
		sparkline := generateSparkline(metric.Buckets, sparklineWidth)

		// Format numbers
		minStr := formatMetricValue(metric.Min)
		maxStr := formatMetricValue(metric.Max)
		avgStr := formatMetricValue(metric.Avg)
		latestStr := formatMetricValue(metric.Latest)

		// Build line
		line := PadOrTruncate(metric.ShortName, metricWidth) + " " +
			sparkline + " " +
			PadOrTruncate(minStr, numWidth) + " " +
			PadOrTruncate(maxStr, numWidth) + " " +
			PadOrTruncate(avgStr, numWidth) + " " +
			PadOrTruncate(latestStr, numWidth)

		if selected {
			lines = append(lines, SelectedLogStyle.Width(m.width-6).Render(line))
		} else {
			lines = append(lines, LogEntryStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.width - 4).Height(listHeight).Render(content)
}

func (m Model) renderMetricsCompactDetail() string {
	if m.aggregatedMetrics == nil || len(m.aggregatedMetrics.Metrics) == 0 {
		return CompactDetailStyle.Width(m.width - 4).Height(compactDetailHeight).Render(
			DetailMutedStyle.Render("No metric selected"))
	}

	if m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
		return CompactDetailStyle.Width(m.width - 4).Height(compactDetailHeight).Render(
			DetailMutedStyle.Render("No metric selected"))
	}

	metric := m.aggregatedMetrics.Metrics[m.metricsCursor]

	var b strings.Builder

	// First line: Full metric name and type
	b.WriteString(DetailKeyStyle.Render("Metric: "))
	b.WriteString(DetailValueStyle.Render(metric.Name))
	if metric.Type != "" {
		b.WriteString("  ")
		b.WriteString(DetailKeyStyle.Render("Type: "))
		b.WriteString(DetailValueStyle.Render(metric.Type))
	}
	b.WriteString("\n")

	// Second line: Stats
	b.WriteString(DetailKeyStyle.Render("Min: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.4f", metric.Min)))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Max: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.4f", metric.Max)))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Avg: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.4f", metric.Avg)))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Latest: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.4f", metric.Latest)))
	b.WriteString("\n")

	// Third line: Bucket info
	b.WriteString(DetailKeyStyle.Render("Buckets: "))
	b.WriteString(DetailMutedStyle.Render(fmt.Sprintf("%d @ %s intervals",
		len(metric.Buckets), m.aggregatedMetrics.BucketSize)))

	return CompactDetailStyle.Width(m.width - 4).Height(compactDetailHeight).Render(b.String())
}

// renderMetricDetail renders a split view: chart on top half, document details on bottom half
func (m Model) renderMetricDetail() string {
	contentHeight := m.getFullScreenHeight()

	if m.aggregatedMetrics == nil || m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
		return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(
			DetailMutedStyle.Render("No metric selected"))
	}

	metric := m.aggregatedMetrics.Metrics[m.metricsCursor]

	// Split the view: top half for chart, bottom half for docs
	headerLines := 6 // Header + stats + time range + spacing
	halfHeight := (contentHeight - headerLines) / 2
	chartHeight := halfHeight
	docsHeight := contentHeight - headerLines - chartHeight

	if chartHeight < 5 {
		chartHeight = 5
	}
	if docsHeight < 4 {
		docsHeight = 4
	}

	chartWidth := m.width - 10
	if chartWidth < 20 {
		chartWidth = 20
	}

	var b strings.Builder

	// Header: Metric name and type
	b.WriteString(DetailKeyStyle.Render("Metric: "))
	b.WriteString(DetailValueStyle.Render(metric.Name))
	if metric.Type != "" {
		b.WriteString("  ")
		b.WriteString(DetailMutedStyle.Render(fmt.Sprintf("(%s)", metric.Type)))
	}
	b.WriteString("\n")

	// Stats line
	b.WriteString(DetailKeyStyle.Render("Min: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.6f", metric.Min)))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Max: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.6f", metric.Max)))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Avg: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.6f", metric.Avg)))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Latest: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.6f", metric.Latest)))
	b.WriteString("\n")

	// Time range info
	if len(metric.Buckets) > 0 {
		b.WriteString(DetailKeyStyle.Render("Time Range: "))
		b.WriteString(DetailMutedStyle.Render(fmt.Sprintf(
			"%s → %s (%d buckets @ %s intervals)",
			metric.Buckets[0].Timestamp.Format("15:04:05"),
			metric.Buckets[len(metric.Buckets)-1].Timestamp.Format("15:04:05"),
			len(metric.Buckets),
			m.aggregatedMetrics.BucketSize)))
	}
	b.WriteString("\n\n")

	// Top half: Chart
	chart := m.renderLargeChart(metric.Buckets, metric.Min, metric.Max, chartWidth, chartHeight)
	b.WriteString(chart)
	b.WriteString("\n\n")

	// Bottom half: Document details
	b.WriteString(m.renderMetricDetailDocs(docsHeight))

	return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(b.String())
}

// renderMetricDetailDocs renders the document browser section in the metric detail view
func (m Model) renderMetricDetailDocs(height int) string {
	var b strings.Builder

	// Header with navigation hint
	docCount := len(m.metricDetailDocs)
	if m.metricDetailDocsLoading {
		b.WriteString(DetailKeyStyle.Render("Documents: "))
		b.WriteString(LoadingStyle.Render("Loading..."))
		return b.String()
	}

	if docCount == 0 {
		b.WriteString(DetailKeyStyle.Render("Documents: "))
		b.WriteString(DetailMutedStyle.Render("No documents found for this metric"))
		return b.String()
	}

	// Navigation header
	b.WriteString(DetailKeyStyle.Render("Documents: "))
	b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%d/%d", m.metricDetailDocCursor+1, docCount)))
	b.WriteString("  ")
	b.WriteString(DetailMutedStyle.Render("(a/d: prev/next)"))
	b.WriteString("\n")

	// Show current document
	if m.metricDetailDocCursor < docCount {
		doc := m.metricDetailDocs[m.metricDetailDocCursor]

		// Timestamp and service
		b.WriteString(DetailKeyStyle.Render("Time: "))
		b.WriteString(DetailValueStyle.Render(doc.Timestamp.Format("2006-01-02 15:04:05.000")))
		b.WriteString("  ")
		b.WriteString(DetailKeyStyle.Render("Service: "))
		service := doc.ServiceName
		if service == "" {
			service = "unknown"
		}
		b.WriteString(DetailValueStyle.Render(service))
		b.WriteString("\n")

		// Scope if available
		if scopeName, ok := doc.Scope["name"].(string); ok && scopeName != "" {
			b.WriteString(DetailKeyStyle.Render("Scope: "))
			b.WriteString(DetailValueStyle.Render(scopeName))
			b.WriteString("\n")
		}

		// Metrics data - show all metric values in this document
		if len(doc.Metrics) > 0 {
			b.WriteString(DetailKeyStyle.Render("Metrics: "))
			b.WriteString("\n")
			// Show each metric value (up to available height)
			lineCount := 0
			maxLines := height - 4 // Leave room for headers
			for key, val := range doc.Metrics {
				if lineCount >= maxLines {
					b.WriteString(DetailMutedStyle.Render("  ... more metrics"))
					break
				}
				b.WriteString("  ")
				b.WriteString(DetailKeyStyle.Render(key + ": "))
				b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%v", val)))
				b.WriteString("\n")
				lineCount++
			}
		}

		// Attributes if space allows
		if len(doc.Attributes) > 0 && height > 6 {
			b.WriteString(DetailKeyStyle.Render("Attrs: "))
			attrs := formatKVPreview(doc.Attributes, 3, 0)
			b.WriteString(DetailMutedStyle.Render(attrs))
		}
	}

	return b.String()
}

