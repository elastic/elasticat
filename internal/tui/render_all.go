package tui

import (
	"fmt"
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/andrewvc/turboelasticat/internal/es"
	"github.com/charmbracelet/lipgloss"
)

// getContentHeight returns the available height for main content
// accounting for title, status bar, help bar, and optional compact detail
func (m Model) getContentHeight(includeCompactDetail bool) int {
	const titleHeaderHeight = 1
	const newlines = 4 // Newlines between sections

	fixedHeight := titleHeaderHeight + statusBarHeight + helpBarHeight + layoutPadding + newlines
	if includeCompactDetail {
		fixedHeight += compactDetailHeight
	}

	contentHeight := m.height - fixedHeight
	if contentHeight < 3 {
		contentHeight = 3
	}
	return contentHeight
}

// getFullScreenHeight returns height for full-screen views (detail, fields, etc.)
// These views don't have compact detail but need extra space for their own headers
func (m Model) getFullScreenHeight() int {
	const titleHeaderHeight = 1
	const extraPadding = 2 // Extra padding for full-screen views

	fixedHeight := titleHeaderHeight + statusBarHeight + helpBarHeight + layoutPadding + extraPadding

	contentHeight := m.height - fixedHeight
	if contentHeight < 3 {
		contentHeight = 3
	}
	return contentHeight
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Calculate heights using helper method
	logListHeight := m.getContentHeight(true)

	var b strings.Builder

	// Title header (top)
	b.WriteString(m.renderTitleHeader())
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// Main content based on mode
	switch m.mode {
	case viewLogs, viewSearch, viewIndex, viewQuery:
		// Log list (fills available space)
		b.WriteString(m.renderLogListWithHeight(logListHeight))
		b.WriteString("\n")
		// Compact detail (anchored to bottom, fixed height)
		b.WriteString(m.renderCompactDetail())
		if m.mode == viewSearch {
			b.WriteString("\n")
			b.WriteString(m.renderSearchInput())
		}
		if m.mode == viewIndex {
			b.WriteString("\n")
			b.WriteString(m.renderIndexInput())
		}
		if m.mode == viewQuery {
			b.WriteString("\n")
			b.WriteString(m.renderQueryOverlay())
		}
	case viewDetail, viewDetailJSON:
		b.WriteString(m.renderDetailView())
	case viewFields:
		b.WriteString(m.renderFieldSelector())
	case viewMetricsDashboard:
		b.WriteString(m.renderMetricsDashboard(logListHeight))
		b.WriteString("\n")
		b.WriteString(m.renderMetricsCompactDetail())
	case viewMetricDetail:
		b.WriteString(m.renderMetricDetail())
	case viewTraceNames:
		b.WriteString(m.renderTransactionNames(logListHeight))
	case viewPerspectiveList:
		b.WriteString(m.renderPerspectiveList(logListHeight))
		b.WriteString("\n")
		b.WriteString(m.renderCompactDetail())
	case viewErrorModal:
		// Render the underlying view first (so modal appears on top)
		switch m.previousMode {
		case viewLogs:
			b.WriteString(m.renderLogListWithHeight(logListHeight))
		case viewMetricsDashboard:
			b.WriteString(m.renderMetricsDashboard(logListHeight))
		case viewTraceNames:
			b.WriteString(m.renderTransactionNames(logListHeight))
		case viewPerspectiveList:
			b.WriteString(m.renderPerspectiveList(logListHeight))
		default:
			b.WriteString(m.renderLogListWithHeight(logListHeight))
		}
		// Render error modal overlay
		b.WriteString(m.renderErrorModal())
		// Skip help bar for modal (it has its own instructions)
		return b.String()
	}

	// Help bar (bottom)
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return AppStyle.Render(b.String())
}

// renderTitleHeader renders the title header with cat ASCII art and frame line
// Component render functions have been extracted to separate files:
// - renderTitleHeader() → header.go
// - renderStatusBar() → statusbar.go
// - renderSearchInput() → inputs.go
// - renderIndexInput() → inputs.go
// - renderHelpBar() → helpbar.go
//
// Formatting functions have been extracted to separate files:
// - formatRelativeTime() → formatting_time.go
// - generateSparkline() → formatting_metrics.go
// - formatMetricValue() → formatting_metrics.go
// - renderLargeChart() → formatting_charts.go
// - PadLeft() → formatting_text.go
// - TruncateWithEllipsis() → formatting_text.go (was in styles.go)
// - PadOrTruncate() → formatting_text.go (was in styles.go)

func (m Model) renderLogList() string {
	// Default height calculation for backwards compatibility
	defaultHeight := m.height - statusBarHeight - helpBarHeight - compactDetailHeight - layoutPadding - 3
	if defaultHeight < 3 {
		defaultHeight = 3
	}
	return m.renderLogListWithHeight(defaultHeight)
}

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
		if field.Width > 0 {
			fixedWidth += field.Width + 1 // +1 for space between columns
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
		if field.Name == "@timestamp" && m.relativeTime {
			label = "AGE"
		}
		width := field.Width
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

func (m Model) renderLogEntry(log es.LogEntry, selected bool) string {
	// Calculate fixed column widths to determine flexible column width
	fixedWidth := 0
	flexibleFieldIdx := -1
	for i, field := range m.displayFields {
		if field.Width > 0 {
			fixedWidth += field.Width + 1 // +1 for space between columns
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
			return SelectedLogStyle
		}
		return baseStyle
	}

	var parts []string
	for i, field := range m.displayFields {
		var value string
		var styled string
		width := field.Width
		if width == 0 {
			if i == flexibleFieldIdx {
				width = flexWidth
			} else {
				width = 15 // Default width for unspecified fields
			}
		}

		switch field.Name {
		case "@timestamp":
			if m.relativeTime {
				value = formatRelativeTime(log.Timestamp)
			} else {
				value = log.Timestamp.Format("15:04:05")
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

// formatRelativeTime returns a human-readable relative time string
// renderCompactDetail renders a compact detail view of the selected log at the bottom
func (m Model) renderCompactDetail() string {
	if len(m.logs) == 0 || m.selectedIndex >= len(m.logs) {
		return CompactDetailStyle.Width(m.width - 4).Height(4).Render(
			DetailMutedStyle.Render("No entry selected"),
		)
	}

	log := m.logs[m.selectedIndex]
	hl := m.Highlighter()
	var b strings.Builder

	// First line varies by signal type
	ts := log.Timestamp.Format("2006-01-02 15:04:05.000")
	service := log.ServiceName
	if service == "" {
		service = "unknown"
	}

	b.WriteString(DetailKeyStyle.Render("Time: "))
	b.WriteString(DetailValueStyle.Render(ts))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Service: "))
	b.WriteString(hl.ApplyToField(service, DetailValueStyle))

	switch m.signalType {
	case signalTraces:
		// Trace-specific first line
		if log.Kind != "" {
			b.WriteString("  ")
			b.WriteString(DetailKeyStyle.Render("Kind: "))
			b.WriteString(DetailValueStyle.Render(log.Kind))
		}
		if log.Duration > 0 {
			ms := float64(log.Duration) / 1_000_000.0
			b.WriteString("  ")
			b.WriteString(DetailKeyStyle.Render("Duration: "))
			if ms < 1 {
				b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.3fms", ms)))
			} else {
				b.WriteString(DetailValueStyle.Render(fmt.Sprintf("%.2fms", ms)))
			}
		}
		if log.Status != nil {
			if code, ok := log.Status["code"].(string); ok {
				b.WriteString("  ")
				b.WriteString(DetailKeyStyle.Render("Status: "))
				b.WriteString(DetailValueStyle.Render(code))
			}
		}
	case signalMetrics:
		// Metrics-specific first line - show scope
		if log.Scope != nil {
			if scopeName, ok := log.Scope["name"].(string); ok && scopeName != "" {
				b.WriteString("  ")
				b.WriteString(DetailKeyStyle.Render("Scope: "))
				b.WriteString(DetailValueStyle.Render(scopeName))
			}
		}
	default:
		// Log-specific first line
		level := log.GetLevel()
		b.WriteString("  ")
		b.WriteString(DetailKeyStyle.Render("Level: "))
		b.WriteString(hl.ApplyToField(level, LevelStyle(level)))
		resource := log.GetResource()
		if resource != "" {
			b.WriteString("  ")
			b.WriteString(DetailKeyStyle.Render("Resource: "))
			b.WriteString(hl.ApplyToField(resource, DetailValueStyle))
		}
	}
	b.WriteString("\n")

	// Second line: varies by signal type
	switch m.signalType {
	case signalTraces:
		msg := log.Name
		if msg == "" {
			msg = log.GetMessage()
		}
		b.WriteString(DetailKeyStyle.Render("Name: "))
		msg = strings.ReplaceAll(msg, "\n", " ")
		maxMsgLen := (m.width - 10) * 2
		if len(msg) > maxMsgLen {
			msg = msg[:maxMsgLen-3] + "..."
		}
		b.WriteString(hl.ApplyToField(msg, DetailValueStyle))
	case signalMetrics:
		// Show metrics on second line
		if len(log.Metrics) > 0 {
			b.WriteString(DetailKeyStyle.Render("Metrics: "))
			metricParts := []string{}
			for k, v := range log.Metrics {
				metricParts = append(metricParts, fmt.Sprintf("%s=%v", k, v))
			}
			metricsStr := strings.Join(metricParts, ", ")
			maxLen := m.width - 15
			if len(metricsStr) > maxLen {
				metricsStr = metricsStr[:maxLen-3] + "..."
			}
			b.WriteString(hl.ApplyToField(metricsStr, DetailValueStyle))
		} else {
			b.WriteString(DetailMutedStyle.Render("No metrics data"))
		}
	default:
		msg := log.GetMessage()
		b.WriteString(DetailKeyStyle.Render("Message: "))
		msg = strings.ReplaceAll(msg, "\n", " ")
		maxMsgLen := (m.width - 10) * 2
		if len(msg) > maxMsgLen {
			msg = msg[:maxMsgLen-3] + "..."
		}
		b.WriteString(hl.ApplyToField(msg, DetailValueStyle))
	}
	b.WriteString("\n")

	// Third line: signal-specific details
	switch m.signalType {
	case signalTraces:
		if log.TraceID != "" {
			b.WriteString(DetailKeyStyle.Render("Trace: "))
			b.WriteString(DetailMutedStyle.Render(log.TraceID))
			if log.SpanID != "" {
				b.WriteString("  ")
				b.WriteString(DetailKeyStyle.Render("Span: "))
				b.WriteString(DetailMutedStyle.Render(log.SpanID))
			}
		}
		// Fourth line: Show child spans
		b.WriteString("\n")
		if m.spansLoading {
			b.WriteString(DetailKeyStyle.Render("Spans: "))
			b.WriteString(LoadingStyle.Render("Loading..."))
		} else if len(m.spans) > 0 {
			b.WriteString(DetailKeyStyle.Render(fmt.Sprintf("Spans (%d): ", len(m.spans))))
			// Show first 5 span names
			spanNames := []string{}
			for i, span := range m.spans {
				if i >= 5 {
					spanNames = append(spanNames, "…")
					break
				}
				name := span.Name
				if name == "" {
					name = span.GetMessage()
				}
				if name == "" {
					name = "unnamed"
				}
				spanNames = append(spanNames, name)
			}
			spansStr := strings.Join(spanNames, " → ")
			maxLen := m.width - 20
			if len(spansStr) > maxLen {
				spansStr = spansStr[:maxLen-3] + "..."
			}
			b.WriteString(DetailValueStyle.Render(spansStr))
		} else if log.TraceID != "" {
			b.WriteString(DetailKeyStyle.Render("Spans: "))
			b.WriteString(DetailMutedStyle.Render("No child spans"))
		}
	case signalMetrics:
		// Show attributes for metrics (often contains span info)
		if len(log.Attributes) > 0 {
			b.WriteString(DetailKeyStyle.Render("Attrs: "))
			attrParts := []string{}
			maxAttrs := 5
			count := 0
			for k, v := range log.Attributes {
				if count >= maxAttrs {
					attrParts = append(attrParts, "...")
					break
				}
				attrParts = append(attrParts, fmt.Sprintf("%s=%v", k, v))
				count++
			}
			attrStr := strings.Join(attrParts, ", ")
			if len(attrStr) > m.width-15 {
				attrStr = attrStr[:m.width-18] + "..."
			}
			b.WriteString(hl.ApplyToField(attrStr, DetailMutedStyle))
		}
	default:
		// Logs - show attributes
		if len(log.Attributes) > 0 {
			b.WriteString(DetailKeyStyle.Render("Attrs: "))
			attrParts := []string{}
			maxAttrs := 5
			count := 0
			for k, v := range log.Attributes {
				if count >= maxAttrs {
					attrParts = append(attrParts, "...")
					break
				}
				attrParts = append(attrParts, fmt.Sprintf("%s=%v", k, v))
				count++
			}
			attrStr := strings.Join(attrParts, ", ")
			if len(attrStr) > m.width-15 {
				attrStr = attrStr[:m.width-18] + "..."
			}
			b.WriteString(hl.ApplyToField(attrStr, DetailMutedStyle))
		}
	}

	return CompactDetailStyle.Width(m.width - 4).Height(compactDetailHeight).Render(b.String())
}

// renderQueryOverlay renders a floating window with the ES query
func (m Model) renderQueryOverlay() string {
	var b strings.Builder

	index := m.lastQueryIndex + "*"

	// Header showing format and status
	var formatLabel string
	if m.queryFormat == formatKibana {
		formatLabel = "Kibana Dev Tools"
	} else {
		formatLabel = "curl"
	}

	header := fmt.Sprintf("Query (%s)", formatLabel)
	b.WriteString(QueryHeaderStyle.Render(header))

	// Show status message if recent (within 2 seconds)
	if m.statusMessage != "" && time.Since(m.statusTime) < 2*time.Second {
		b.WriteString("  ")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render(m.statusMessage))
	}
	b.WriteString("\n\n")

	if m.queryFormat == formatKibana {
		// Kibana Dev Tools format
		b.WriteString(QueryMethodStyle.Render("GET "))
		b.WriteString(QueryPathStyle.Render(index + "/_search"))
		b.WriteString("\n")
		b.WriteString(QueryBodyStyle.Render(m.lastQueryJSON))
	} else {
		// curl format
		b.WriteString(QueryBodyStyle.Render("curl -X GET 'http://localhost:9200/" + index + "/_search' \\\n"))
		b.WriteString(QueryBodyStyle.Render("  -H 'Content-Type: application/json' \\\n"))
		b.WriteString(QueryBodyStyle.Render("  -d '"))
		// Compact JSON for curl
		var compact bytes.Buffer
		if err := json.Compact(&compact, []byte(m.lastQueryJSON)); err == nil {
			b.WriteString(QueryBodyStyle.Render(compact.String()))
		} else {
			b.WriteString(QueryBodyStyle.Render(m.lastQueryJSON))
		}
		b.WriteString(QueryBodyStyle.Render("'"))
	}

	// Calculate height - use full screen height since this replaces the log list
	height := m.getFullScreenHeight()
	if height < 10 {
		height = 10
	}

	return QueryOverlayStyle.Width(m.width - 8).Height(height).Render(b.String())
}

func (m Model) renderDetailView() string {
	// Minimal header with position and status
	var parts []string

	// Position indicator
	if len(m.logs) > 0 {
		parts = append(parts, DetailMutedStyle.Render(fmt.Sprintf("%d/%d", m.selectedIndex+1, len(m.logs))))
	}

	// View mode indicator
	if m.mode == viewDetailJSON {
		parts = append(parts, DetailKeyStyle.Render("JSON"))
	}

	// Show status message if recent (within 2 seconds)
	if m.statusMessage != "" && time.Since(m.statusTime) < 2*time.Second {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render(m.statusMessage))
	}

	header := strings.Join(parts, "  ")
	content := m.viewport.View()

	contentHeight := m.getFullScreenHeight()

	// Only add header line if we have something to show
	if header != "" {
		return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(header + "\n" + content)
	}
	return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(content)
}

func (m Model) renderFieldSelector() string {
	var b strings.Builder

	// Header
	header := "Select Fields"
	if m.fieldsSearchMode {
		header = fmt.Sprintf("Search: %s_", m.fieldsSearch)
	} else if m.fieldsSearch != "" {
		header = fmt.Sprintf("Filter: %s", m.fieldsSearch)
	}
	b.WriteString(QueryHeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(DetailMutedStyle.Render("Space/Enter to toggle • / to search • r to reset • ESC to close"))
	b.WriteString("\n\n")

	contentHeight := m.getFullScreenHeight()

	if m.fieldsLoading {
		b.WriteString(LoadingStyle.Render("Loading fields..."))
		return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(b.String())
	}

	// Get sorted fields
	sortedFields := m.getSortedFieldList()

	if len(sortedFields) == 0 {
		if m.fieldsSearch != "" {
			b.WriteString(DetailMutedStyle.Render("No fields matching '" + m.fieldsSearch + "'"))
		} else {
			b.WriteString(DetailMutedStyle.Render("No fields available"))
		}
		return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(b.String())
	}

	// Create set of selected field names
	selectedNames := make(map[string]bool)
	for _, f := range m.displayFields {
		selectedNames[f.Name] = true
	}

	// Calculate visible range (account for header and instructions)
	visibleHeight := contentHeight - 4
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	startIdx := m.fieldsCursor - visibleHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(sortedFields) {
		endIdx = len(sortedFields)
		startIdx = endIdx - visibleHeight
		if startIdx < 0 {
			startIdx = 0
		}
	}

	for i := startIdx; i < endIdx; i++ {
		field := sortedFields[i]
		selected := selectedNames[field.Name]

		// Checkbox
		var checkbox string
		if selected {
			checkbox = "[✓]"
		} else {
			checkbox = "[ ]"
		}

		// Format doc count
		var countStr string
		if field.DocCount > 0 {
			if field.DocCount >= 1000000 {
				countStr = fmt.Sprintf("%dM", field.DocCount/1000000)
			} else if field.DocCount >= 1000 {
				countStr = fmt.Sprintf("%dK", field.DocCount/1000)
			} else {
				countStr = fmt.Sprintf("%d", field.DocCount)
			}
		} else {
			countStr = "-"
		}

		// Field name, type, and count
		fieldLine := fmt.Sprintf("%s %-40s %-10s %6s docs", checkbox, field.Name, field.Type, countStr)

		if i == m.fieldsCursor {
			b.WriteString(SelectedLogStyle.Width(m.width - 8).Render(fieldLine))
		} else if selected {
			b.WriteString(StatusKeyStyle.Render(fieldLine))
		} else {
			b.WriteString(DetailValueStyle.Render(fieldLine))
		}
		b.WriteString("\n")
	}

	// Show scroll indicator
	if len(sortedFields) > visibleHeight {
		b.WriteString(DetailMutedStyle.Render(fmt.Sprintf("\n%d/%d fields", m.fieldsCursor+1, len(sortedFields))))
	}

	return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(b.String())
}

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

	// Calculate visible range
	contentHeight := listHeight - 4 // Account for borders and header
	if contentHeight < 3 {
		contentHeight = 3
	}

	metrics := m.aggregatedMetrics.Metrics
	startIdx := m.metricsCursor - contentHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + contentHeight
	if endIdx > len(metrics) {
		endIdx = len(metrics)
		startIdx = endIdx - contentHeight
		if startIdx < 0 {
			startIdx = 0
		}
	}

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
			lines = append(lines, SelectedLogStyle.Width(m.width - 6).Render(line))
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

// generateSparkline creates a Unicode sparkline from time series buckets
// renderMetricDetail renders a full-screen view of a single metric with a large chart
func (m Model) renderMetricDetail() string {
	contentHeight := m.getFullScreenHeight()

	if m.aggregatedMetrics == nil || m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
		return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(
			DetailMutedStyle.Render("No metric selected"))
	}

	metric := m.aggregatedMetrics.Metrics[m.metricsCursor]
	var b strings.Builder

	// Header: Metric name and type
	b.WriteString(DetailKeyStyle.Render("Metric: "))
	b.WriteString(DetailValueStyle.Render(metric.Name))
	if metric.Type != "" {
		b.WriteString("  ")
		b.WriteString(DetailMutedStyle.Render(fmt.Sprintf("(%s)", metric.Type)))
	}
	b.WriteString("\n\n")

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
	b.WriteString("\n\n")

	// Time range info
	if len(metric.Buckets) > 0 {
		b.WriteString(DetailKeyStyle.Render("Time Range: "))
		b.WriteString(DetailMutedStyle.Render(fmt.Sprintf(
			"%s → %s (%d buckets @ %s intervals)",
			metric.Buckets[0].Timestamp.Format("15:04:05"),
			metric.Buckets[len(metric.Buckets)-1].Timestamp.Format("15:04:05"),
			len(metric.Buckets),
			m.aggregatedMetrics.BucketSize)))
		b.WriteString("\n\n")
	}

	// Render the large chart - leave room for header and stats (8 lines)
	chartHeight := contentHeight - 8
	if chartHeight < 5 {
		chartHeight = 5
	}
	chartWidth := m.width - 10
	if chartWidth < 20 {
		chartWidth = 20
	}

	chart := m.renderLargeChart(metric.Buckets, metric.Min, metric.Max, chartWidth, chartHeight)
	b.WriteString(chart)

	return DetailStyle.Width(m.width - 4).Height(contentHeight).Render(b.String())
}

// renderLargeChart creates a multi-line ASCII chart from metric buckets
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

	// Calculate visible range
	contentHeight := listHeight - 4
	if contentHeight < 3 {
		contentHeight = 3
	}

	startIdx := m.traceNamesCursor - contentHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + contentHeight
	if endIdx > len(m.transactionNames) {
		endIdx = len(m.transactionNames)
		startIdx = endIdx - contentHeight
		if startIdx < 0 {
			startIdx = 0
		}
	}

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
			lines = append(lines, SelectedLogStyle.Width(m.width - 6).Render(line))
		} else {
			lines = append(lines, LogEntryStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.width - 4).Height(listHeight).Render(content)
}

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

	// Calculate visible range
	contentHeight := listHeight - 4
	if contentHeight < 3 {
		contentHeight = 3
	}

	startIdx := m.perspectiveCursor - contentHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + contentHeight
	if endIdx > len(m.perspectiveItems) {
		endIdx = len(m.perspectiveItems)
		startIdx = endIdx - contentHeight
		if startIdx < 0 {
			startIdx = 0
		}
	}

	var lines []string
	lines = append(lines, header)

	for i := startIdx; i < endIdx; i++ {
		item := m.perspectiveItems[i]
		selected := i == m.perspectiveCursor

		// Check if this item is currently active as a filter
		isActive := false
		if m.currentPerspective == PerspectiveServices && m.filterService == item.Name {
			isActive = true
		} else if m.currentPerspective == PerspectiveResources && m.filterResource == item.Name {
			isActive = true
		}

		// Format values
		logsStr := fmt.Sprintf("%d", item.LogCount)
		tracesStr := fmt.Sprintf("%d", item.TraceCount)
		metricsStr := fmt.Sprintf("%d", item.MetricCount)

		// Add active marker
		nameDisplay := item.Name
		if isActive {
			nameDisplay = "✓ " + item.Name
		}

		line := PadOrTruncate(nameDisplay, nameWidth) + " " +
			PadOrTruncate(logsStr, logsWidth) + " " +
			PadOrTruncate(tracesStr, tracesWidth) + " " +
			PadOrTruncate(metricsStr, metricsWidth)

		if selected {
			lines = append(lines, SelectedLogStyle.Width(m.width - 6).Render(line))
		} else {
			lines = append(lines, LogEntryStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return LogListStyle.Width(m.width - 4).Height(listHeight).Render(content)
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

		// Child spans
		if m.spansLoading {
			b.WriteString(DetailKeyStyle.Render("Child Spans: "))
			b.WriteString(LoadingStyle.Render("Loading..."))
			b.WriteString("\n\n")
		} else if len(m.spans) > 0 {
			b.WriteString(DetailKeyStyle.Render(fmt.Sprintf("Child Spans (%d):", len(m.spans))))
			b.WriteString("\n")
			// Show all spans in a simple waterfall-like view
			for i, span := range m.spans {
				name := span.Name
				if name == "" {
					name = span.GetMessage()
				}
				if name == "" {
					name = "unnamed"
				}

				// Add indentation to show hierarchy (basic waterfall approximation)
				indent := "  "
				if i > 0 {
					indent = "    "
				}

				// Duration
				durationStr := ""
				if span.Duration > 0 {
					ms := float64(span.Duration) / 1_000_000.0
					if ms < 1 {
						durationStr = fmt.Sprintf(" (%.3fms)", ms)
					} else {
						durationStr = fmt.Sprintf(" (%.2fms)", ms)
					}
				}

				b.WriteString(fmt.Sprintf("%s%d. %s%s\n", indent, i+1, name, durationStr))
			}
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

func (m Model) renderErrorModal() string {
	var b strings.Builder

	// Modal dimensions
	modalWidth := min(m.width-8, 80)

	// Calculate centering (use fixed top padding to avoid cut-off)
	leftPadding := (m.width - modalWidth) / 2
	topPadding := 3 // Fixed small padding from top

	// Add top padding
	for i := 0; i < topPadding; i++ {
		b.WriteString("\n")
	}

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")). // Red border
		Padding(1, 2).
		Align(lipgloss.Left)

	// Error title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Render("⚠ Error")

	// Check if we just copied (statusMessage set within last 2 seconds)
	justCopied := m.statusMessage == "Error copied to clipboard!" &&
		time.Since(m.statusTime) < 2*time.Second

	// Scroll indicator
	scrollInfo := ""
	if m.errorViewport.TotalLineCount() > m.errorViewport.Height {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf(" (scroll: %d%%) ", int(m.errorViewport.ScrollPercent()*100)))
	}

	// Action buttons
	var copyButton string
	if justCopied {
		copyButton = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Render("[y] Copy ✓ copied")
	} else {
		copyButton = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Render("[y] Copy")
	}

	actions := lipgloss.JoinHorizontal(
		lipgloss.Left,
		copyButton,
		"  ",
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("[j/k] Scroll"),
		"  ",
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("[esc/q] Close"),
		scrollInfo,
	)

	// Get viewport content (wrap it in a style to constrain width)
	viewportContent := lipgloss.NewStyle().
		Width(modalWidth - 8). // Account for border (2) + padding (2*2) + margin (2)
		Render(m.errorViewport.View())

	// Combine content with viewport for scrollable error message
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		viewportContent,
		"",
		actions,
	)

	// Render modal with centering
	modal := modalStyle.Render(content)

	// Add left padding to center horizontally
	lines := strings.Split(modal, "\n")
	for _, line := range lines {
		b.WriteString(strings.Repeat(" ", leftPadding))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// Commands

