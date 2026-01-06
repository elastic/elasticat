// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/elastic/elasticat/internal/es"
)

func (m Model) renderLogListWithHeight(listHeight int) string {
	if m.err != nil {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	if len(m.logs) == 0 {
		return LogListStyle.Width(m.width - 4).Height(listHeight).Render(
			LoadingStyle.Render("No logs found. Waiting for data..."))
	}

	// Calculate flexible column width first (needed for both header and content)
	fixedWidth := 0
	for _, field := range m.displayFields {
		width := field.Width
		if field.Name == "@timestamp" {
			width = m.timestampWidth(width)
		}
		if width > 0 {
			fixedWidth += width + 1 // +1 for space between columns
		}
	}
	flexWidth := m.width - fixedWidth - 10
	if flexWidth < 20 {
		flexWidth = 20
	}

	// Build dynamic column header from displayFields
	var headerParts []string
	for _, field := range m.displayFields {
		label := field.Label
		// Special handling for timestamp label
		if field.Name == "@timestamp" {
			switch m.timeDisplayMode {
			case timeDisplayRelative:
				label = "AGE"
			case timeDisplayFull:
				label = "DATETIME"
			default:
				label = "TIME"
			}
		}
		width := field.Width
		if field.Name == "@timestamp" {
			width = m.timestampWidth(width)
		}
		if width == 0 {
			width = flexWidth
		}
		headerParts = append(headerParts, PadOrTruncate(label, width))
	}
	header := HeaderRowStyle.Render(strings.Join(headerParts, " "))

	// Calculate visible range based on provided height
	// Subtract 3 for header row and borders
	visibleHeight := listHeight - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	startIdx := m.selectedIndex - visibleHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(m.logs) {
		endIdx = len(m.logs)
		startIdx = endIdx - visibleHeight
		if startIdx < 0 {
			startIdx = 0
		}
	}

	var lines []string
	lines = append(lines, header)
	for i := startIdx; i < endIdx; i++ {
		log := m.logs[i]
		line := m.renderLogEntry(log, i == m.selectedIndex)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.width - 4).Height(listHeight).Render(content)
}

// timestampWidth returns the width to use for the timestamp column based on mode.
func (m Model) timestampWidth(base int) int {
	if base <= 0 {
		base = 8
	}
	switch m.timeDisplayMode {
	case timeDisplayFull:
		if base < 19 {
			return 19
		}
		return base
	default:
		// Clock or relative can use the base width
		return base
	}
}

func (m Model) renderLogEntry(log es.LogEntry, selected bool) string {
	// Calculate fixed column widths to determine flexible column width
	fixedWidth := 0
	flexibleFieldIdx := -1
	for i, field := range m.displayFields {
		width := field.Width
		if field.Name == "@timestamp" {
			width = m.timestampWidth(width)
		}
		if width > 0 {
			fixedWidth += width + 1 // +1 for space between columns
		} else {
			flexibleFieldIdx = i
		}
	}

	// Available width for flexible field (usually MESSAGE)
	flexWidth := m.width - fixedWidth - 10 // Account for borders and padding
	if flexWidth < 20 {
		flexWidth = 20
	}

	// Get highlighter for search matches
	hl := m.Highlighter()

	// When selected, use selection colors for all columns
	getStyle := func(baseStyle lipgloss.Style) lipgloss.Style {
		if selected {
			return SelectedCellStyle
		}
		return baseStyle
	}

	var parts []string
	for i, field := range m.displayFields {
		var value string
		var styled string
		width := field.Width
		if field.Name == "@timestamp" {
			width = m.timestampWidth(width)
		}
		if width == 0 {
			if i == flexibleFieldIdx {
				width = flexWidth
			} else {
				width = 15 // Default width for unspecified fields
			}
		}

		switch field.Name {
		case "@timestamp":
			switch m.timeDisplayMode {
			case timeDisplayRelative:
				value = formatRelativeTime(log.Timestamp)
			case timeDisplayFull:
				value = formatFullTime(log.Timestamp)
			default:
				value = formatClockTime(log.Timestamp)
			}
			// Timestamp is not searchable, just pad and style with MaxWidth to prevent scrolling
			styled = getStyle(TimestampStyle).MaxWidth(width).Render(PadOrTruncate(value, width))

		case "severity_text", "level":
			level := log.GetLevel()
			levelDisplay := strings.ToUpper(level)
			if len(levelDisplay) > 5 {
				levelDisplay = levelDisplay[:5]
			}
			// Keep level style colors even when selected for visibility
			if selected {
				styled = hl.Apply(levelDisplay, width, LevelStyle(level))
			} else {
				styled = hl.Apply(levelDisplay, width, LevelStyle(level))
			}

		case "_resource":
			resource := log.GetResource()
			if resource == "" {
				resource = "-"
			}
			styled = hl.Apply(resource, width, getStyle(ResourceStyle))

		case "service.name", "resource.attributes.service.name":
			service := log.ServiceName
			if service == "" {
				service = "unknown"
			}
			if log.ContainerID != "" && service == "unknown" {
				service = log.ContainerID[:min(12, len(log.ContainerID))]
			}
			styled = hl.Apply(service, width, getStyle(ServiceStyle))

		case "body.text", "body", "message":
			msg := log.GetMessage()
			msg = strings.ReplaceAll(msg, "\n", " ")
			styled = hl.Apply(msg, width, getStyle(MessageStyle))

		default:
			// Generic field extraction
			value = log.GetFieldValue(field.Name)
			if value == "" {
				value = "-"
			}
			value = strings.ReplaceAll(value, "\n", " ")
			styled = hl.Apply(value, width, getStyle(DetailValueStyle))
		}

		parts = append(parts, styled)
	}

	line := strings.Join(parts, " ")

	if selected {
		return SelectedLogStyle.Width(m.width - 6).Render(line)
	}
	return LogEntryStyle.Render(line)
}

// renderSpanWaterfall renders spans as a timeline waterfall visualization.
// Each span is shown with its name and a proportional bar showing its timing
// relative to the earliest span in the trace.
func renderSpanWaterfall(spans []es.LogEntry, availableWidth int) string {
	if len(spans) == 0 {
		return ""
	}

	// Unicode box-drawing characters for the waterfall bars
	const (
		barLeft  = "├"
		barLine  = "─"
		barRight = "┤"
	)

	// Column widths
	// Output format: "  %-20s: [bar]" = 2 (indent) + 20 (name) + 2 (": ") = 24 fixed chars
	nameWidth := 20
	fixedChars := 2 + nameWidth + 2                 // "  " + name + ": "
	barAreaWidth := availableWidth - fixedChars - 2 // -2 extra margin for safety
	if barAreaWidth < 20 {
		barAreaWidth = 20
	}

	// Find timeline bounds: earliest start and latest end
	var minStart, maxEnd time.Time
	for i, span := range spans {
		spanStart := span.Timestamp
		spanEnd := spanStart.Add(time.Duration(span.Duration))

		if i == 0 || spanStart.Before(minStart) {
			minStart = spanStart
		}
		if i == 0 || spanEnd.After(maxEnd) {
			maxEnd = spanStart.Add(time.Duration(span.Duration))
		}
	}

	// Total timeline duration in nanoseconds
	totalDuration := maxEnd.Sub(minStart).Nanoseconds()
	if totalDuration <= 0 {
		totalDuration = 1 // Avoid division by zero
	}

	var b strings.Builder

	for _, span := range spans {
		// Get span name
		name := span.Name
		if name == "" {
			name = span.GetMessage()
		}
		if name == "" {
			name = "unnamed"
		}

		// Truncate name if needed
		if len(name) > nameWidth {
			name = name[:nameWidth-3] + "..."
		}

		// Calculate bar position and width
		spanStart := span.Timestamp
		offsetNs := spanStart.Sub(minStart).Nanoseconds()
		durationNs := span.Duration
		if durationNs <= 0 {
			durationNs = 1 // Minimum duration for display
		}

		// Scale to available width
		startPos := int(float64(offsetNs) / float64(totalDuration) * float64(barAreaWidth))
		barWidth := int(float64(durationNs) / float64(totalDuration) * float64(barAreaWidth))

		// Format duration string
		ms := float64(durationNs) / 1_000_000.0
		var durationStr string
		if ms < 1 {
			durationStr = fmt.Sprintf("%.2fms", ms)
		} else if ms < 100 {
			durationStr = fmt.Sprintf("%.1fms", ms)
		} else {
			durationStr = fmt.Sprintf("%.0fms", ms)
		}

		// Minimum bar width to fit: ├(duration)┤
		minBarWidth := len(durationStr) + 2 // +2 for ├ and ┤
		if barWidth < minBarWidth {
			barWidth = minBarWidth
		}

		// Calculate max allowed bar width at this position
		maxBarWidth := barAreaWidth - startPos
		if maxBarWidth < minBarWidth {
			// Shift start position left to fit minimum bar
			startPos = barAreaWidth - minBarWidth
			if startPos < 0 {
				startPos = 0
			}
			maxBarWidth = barAreaWidth - startPos
		}

		// Clamp bar width to max allowed
		if barWidth > maxBarWidth {
			barWidth = maxBarWidth
		}

		// Build the bar: ├───(duration)───┤
		// Inner width is what's between ├ and ┤
		innerWidth := barWidth - 2
		if innerWidth < 0 {
			innerWidth = 0
		}

		// Center the duration string within the bar (or truncate if needed)
		var bar string
		if innerWidth >= len(durationStr) {
			totalPadding := innerWidth - len(durationStr)
			leftPad := totalPadding / 2
			rightPad := totalPadding - leftPad
			bar = barLeft + strings.Repeat(barLine, leftPad) + durationStr + strings.Repeat(barLine, rightPad) + barRight
		} else {
			// Bar too small for full duration, just show minimal bar
			bar = barLeft + durationStr[:innerWidth] + barRight
		}

		// Build the full line with leading spaces for offset
		leadingSpaces := strings.Repeat(" ", startPos)

		// Write the line
		b.WriteString(fmt.Sprintf("  %-*s: %s%s\n", nameWidth, name, leadingSpaces, bar))
	}

	return b.String()
}

func (m Model) renderLogDetail(log es.LogEntry) string {
	hl := m.Highlighter()
	var b strings.Builder

	// Common fields
	b.WriteString(DetailKeyStyle.Render("Timestamp: "))
	b.WriteString(DetailValueStyle.Render(log.Timestamp.Format(time.RFC3339Nano)))
	b.WriteString("\n\n")

	if log.ServiceName != "" {
		b.WriteString(DetailKeyStyle.Render("Service: "))
		b.WriteString(hl.ApplyToField(log.ServiceName, DetailValueStyle))
		b.WriteString("\n\n")
	}

	// Signal-specific fields
	switch m.signalType {
	case signalMetrics:
		// Scope
		if log.Scope != nil {
			if scopeName, ok := log.Scope["name"].(string); ok && scopeName != "" {
				b.WriteString(DetailKeyStyle.Render("Scope: "))
				b.WriteString(DetailValueStyle.Render(scopeName))
				b.WriteString("\n\n")
			}
		}

		// Metrics values
		if len(log.Metrics) > 0 {
			b.WriteString(DetailKeyStyle.Render("Metrics:"))
			b.WriteString("\n")
			for k, v := range log.Metrics {
				b.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
			b.WriteString("\n")
		}

	case signalTraces:
		// Trace-specific fields
		if log.Name != "" {
			b.WriteString(DetailKeyStyle.Render("Span Name: "))
			b.WriteString(hl.ApplyToField(log.Name, DetailValueStyle))
			b.WriteString("\n\n")
		}

		if log.Kind != "" {
			b.WriteString(DetailKeyStyle.Render("Kind: "))
			b.WriteString(DetailValueStyle.Render(log.Kind))
			b.WriteString("\n\n")
		}

		if log.Duration > 0 {
			ms := float64(log.Duration) / 1_000_000.0
			b.WriteString(DetailKeyStyle.Render("Duration: "))
			if ms < 1 {
				b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.3fms", ms)))
			} else {
				b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.2fms", ms)))
			}
			b.WriteString("\n\n")
		}

		if log.Status != nil {
			if code, ok := log.Status["code"].(string); ok {
				b.WriteString(DetailKeyStyle.Render("Status: "))
				b.WriteString(DetailValueStyle.Render(code))
				b.WriteString("\n\n")
			}
		}

		if log.TraceID != "" {
			b.WriteString(DetailKeyStyle.Render("Trace ID: "))
			b.WriteString(DetailValueStyle.Render(log.TraceID))
			b.WriteString("\n\n")
		}

		if log.SpanID != "" {
			b.WriteString(DetailKeyStyle.Render("Span ID: "))
			b.WriteString(DetailValueStyle.Render(log.SpanID))
			b.WriteString("\n\n")
		}

		// Child spans - render as waterfall timeline
		if m.spansLoading {
			b.WriteString(DetailKeyStyle.Render("Child Spans: "))
			b.WriteString(LoadingStyle.Render("Loading..."))
			b.WriteString("\n\n")
		} else if len(m.spans) > 0 {
			b.WriteString(DetailKeyStyle.Render(fmt.Sprintf("Child Spans (%d):", len(m.spans))))
			b.WriteString("\n")
			// Render waterfall visualization - use viewport width or default
			waterfallWidth := m.viewport.Width
			if waterfallWidth < 60 {
				waterfallWidth = 80
			}
			b.WriteString(renderSpanWaterfall(m.spans, waterfallWidth))
			b.WriteString("\n")
		}

	default: // Logs
		b.WriteString(DetailKeyStyle.Render("Level: "))
		b.WriteString(hl.ApplyToField(log.GetLevel(), LevelStyle(log.GetLevel())))
		b.WriteString("\n\n")

		if log.ContainerID != "" {
			b.WriteString(DetailKeyStyle.Render("Container: "))
			b.WriteString(hl.ApplyToField(log.ContainerID, DetailValueStyle))
			b.WriteString("\n\n")
		}

		b.WriteString(DetailKeyStyle.Render("Message:"))
		b.WriteString("\n")
		b.WriteString(hl.ApplyToField(log.GetMessage(), DetailValueStyle))
		b.WriteString("\n\n")
	}

	// Attributes (common to all signal types)
	if len(log.Attributes) > 0 {
		b.WriteString(DetailKeyStyle.Render("Attributes:"))
		b.WriteString("\n")
		for k, v := range log.Attributes {
			valStr := fmt.Sprintf("%v", v)
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, hl.ApplyToField(valStr, DetailValueStyle)))
		}
		b.WriteString("\n")
	}

	// Resource (common to all signal types)
	if len(log.Resource) > 0 {
		b.WriteString(DetailKeyStyle.Render("Resource:"))
		b.WriteString("\n")
		for k, v := range log.Resource {
			valStr := fmt.Sprintf("%v", v)
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, hl.ApplyToField(valStr, DetailValueStyle)))
		}
	}

	return b.String()
}
