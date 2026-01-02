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

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Calculate heights
	// Total available: m.height
	// Fixed elements: title header (1) + status bar (1) + help bar (1) + compact detail (5) + padding (2) + newlines (4)
	const titleHeaderHeight = 1
	fixedHeight := titleHeaderHeight + statusBarHeight + helpBarHeight + compactDetailHeight + layoutPadding + 4
	logListHeight := m.height - fixedHeight
	if logListHeight < 3 {
		logListHeight = 3
	}

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
	}

	// Help bar (bottom)
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return AppStyle.Render(b.String())
}

// renderTitleHeader renders the title header with cat ASCII art and frame line
func (m Model) renderTitleHeader() string {
	title := " =^..^= 𝓣𝑬𝓵𝓪𝓼𝓽𝓲𝓒𝓪𝓽 =^..^= "

	// Calculate how many characters we need for the line to fill the width
	// Account for padding in the style (2 chars)
	availableWidth := m.width - 2
	titleLen := len([]rune(title)) // Use rune length for Unicode

	if titleLen >= availableWidth {
		return TitleHeaderStyle.Width(m.width).Render(title)
	}

	// Fill the rest with box drawing characters
	lineChars := availableWidth - titleLen
	line := strings.Repeat("═", lineChars)

	fullHeader := title + line
	return TitleHeaderStyle.Width(m.width).Render(fullHeader)
}

func (m Model) renderStatusBar() string {
	var parts []string

	// Signal type
	parts = append(parts, StatusKeyStyle.Render("Signal: ")+StatusValueStyle.Render(m.signalType.String()))

	// Connection status
	if m.err != nil {
		parts = append(parts, ErrorStyle.Render("ES: err"))
	} else {
		parts = append(parts, StatusKeyStyle.Render("ES: ")+StatusValueStyle.Render("ok"))
	}

	// Current index
	parts = append(parts, StatusKeyStyle.Render("Idx: ")+StatusValueStyle.Render(m.client.GetIndex()+"*"))

	// Total logs
	parts = append(parts, StatusKeyStyle.Render("Total: ")+StatusValueStyle.Render(fmt.Sprintf("%d", m.total)))

	// Filters
	if m.searchQuery != "" {
		parts = append(parts, StatusKeyStyle.Render("Query: ")+StatusValueStyle.Render(TruncateWithEllipsis(m.searchQuery, 20)))
	}
	if m.levelFilter != "" {
		parts = append(parts, StatusKeyStyle.Render("Level: ")+StatusValueStyle.Render(m.levelFilter))
	}
	if m.serviceFilter != "" {
		parts = append(parts, StatusKeyStyle.Render("Service: ")+StatusValueStyle.Render(m.serviceFilter))
	}

	// Lookback duration
	parts = append(parts, StatusKeyStyle.Render("Lookback: ")+StatusValueStyle.Render(m.lookback.String()))

	// Sort order
	if m.sortAscending {
		parts = append(parts, StatusKeyStyle.Render("Sort: ")+StatusValueStyle.Render("oldest→"))
	} else {
		parts = append(parts, StatusKeyStyle.Render("Sort: ")+StatusValueStyle.Render("newest→"))
	}

	// Auto-refresh status
	if m.autoRefresh {
		parts = append(parts, StatusKeyStyle.Render("Auto: ")+StatusValueStyle.Render("ON"))
	} else {
		parts = append(parts, StatusKeyStyle.Render("Auto: ")+StatusValueStyle.Render("OFF"))
	}

	// Loading indicator
	if m.loading {
		parts = append(parts, LoadingStyle.Render("loading..."))
	}

	return StatusBarStyle.Width(m.width - 2).Render(strings.Join(parts, "  │  "))
}

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
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 0 {
		diff = -diff
		// Future time
		if diff < time.Minute {
			return fmt.Sprintf("+%ds", int(diff.Seconds()))
		}
		if diff < time.Hour {
			return fmt.Sprintf("+%dm", int(diff.Minutes()))
		}
		return fmt.Sprintf("+%dh", int(diff.Hours()))
	}

	switch {
	case diff < time.Second:
		return "now"
	case diff < time.Minute:
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(diff.Hours()/(24*7)))
	case diff < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(diff.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(diff.Hours()/(24*365)))
	}
}

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

func (m Model) renderSearchInput() string {
	prompt := SearchPromptStyle.Render("Search: ")
	input := m.searchInput.View()
	return SearchStyle.Width(m.width - 4).Render(prompt + input)
}

func (m Model) renderIndexInput() string {
	prompt := SearchPromptStyle.Render("Index: ")
	input := m.indexInput.View()
	return SearchStyle.Width(m.width - 4).Render(prompt + input)
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

	// Calculate height based on content
	height := m.height - 12
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

	// Only add header line if we have something to show
	if header != "" {
		return DetailStyle.Width(m.width - 4).Height(m.height - 6).Render(header + "\n" + content)
	}
	return DetailStyle.Width(m.width - 4).Height(m.height - 6).Render(content)
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

	if m.fieldsLoading {
		b.WriteString(LoadingStyle.Render("Loading fields..."))
		return DetailStyle.Width(m.width - 4).Height(m.height - 6).Render(b.String())
	}

	// Get sorted fields
	sortedFields := m.getSortedFieldList()

	if len(sortedFields) == 0 {
		if m.fieldsSearch != "" {
			b.WriteString(DetailMutedStyle.Render("No fields matching '" + m.fieldsSearch + "'"))
		} else {
			b.WriteString(DetailMutedStyle.Render("No fields available"))
		}
		return DetailStyle.Width(m.width - 4).Height(m.height - 6).Render(b.String())
	}

	// Create set of selected field names
	selectedNames := make(map[string]bool)
	for _, f := range m.displayFields {
		selectedNames[f.Name] = true
	}

	// Calculate visible range
	visibleHeight := m.height - 12
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

	return DetailStyle.Width(m.width - 4).Height(m.height - 6).Render(b.String())
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
func generateSparkline(buckets []es.MetricBucket, width int) string {
	if len(buckets) == 0 {
		return strings.Repeat("-", width)
	}

	// Get min/max for scaling
	var minVal, maxVal float64 = buckets[0].Value, buckets[0].Value
	for _, b := range buckets {
		if b.Value < minVal {
			minVal = b.Value
		}
		if b.Value > maxVal {
			maxVal = b.Value
		}
	}

	// Handle constant values
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
	}

	// Sample or interpolate to fit width
	var result strings.Builder
	step := float64(len(buckets)) / float64(width)

	for i := 0; i < width; i++ {
		idx := int(float64(i) * step)
		if idx >= len(buckets) {
			idx = len(buckets) - 1
		}

		// Normalize to 0-7 range for sparkline chars
		normalized := (buckets[idx].Value - minVal) / valRange
		charIdx := int(normalized * 7)
		if charIdx > 7 {
			charIdx = 7
		}
		if charIdx < 0 {
			charIdx = 0
		}

		result.WriteRune(sparklineChars[charIdx])
	}

	return result.String()
}

// formatMetricValue formats a float64 for compact display
func formatMetricValue(v float64) string {
	if v != v { // NaN check
		return "-"
	}

	absV := v
	if absV < 0 {
		absV = -absV
	}

	switch {
	case absV == 0:
		return "0"
	case absV >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", v/1_000_000_000)
	case absV >= 1_000_000:
		return fmt.Sprintf("%.1fM", v/1_000_000)
	case absV >= 1_000:
		return fmt.Sprintf("%.1fK", v/1_000)
	case absV >= 1:
		return fmt.Sprintf("%.1f", v)
	case absV >= 0.01:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%.3f", v)
	}
}

// renderMetricDetail renders a full-screen view of a single metric with a large chart
func (m Model) renderMetricDetail() string {
	if m.aggregatedMetrics == nil || m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
		return DetailStyle.Width(m.width - 4).Height(m.height - 4).Render(
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

	// Render the large chart
	chartHeight := m.height - 14 // Leave room for header, stats, and help bar
	if chartHeight < 5 {
		chartHeight = 5
	}
	chartWidth := m.width - 10
	if chartWidth < 20 {
		chartWidth = 20
	}

	chart := m.renderLargeChart(metric.Buckets, metric.Min, metric.Max, chartWidth, chartHeight)
	b.WriteString(chart)

	return DetailStyle.Width(m.width - 4).Height(m.height - 4).Render(b.String())
}

// renderLargeChart creates a multi-line ASCII chart from metric buckets
func (m Model) renderLargeChart(buckets []es.MetricBucket, minVal, maxVal float64, width, height int) string {
	if len(buckets) == 0 {
		return DetailMutedStyle.Render("No data points")
	}

	var b strings.Builder

	// Y-axis labels width
	yLabelWidth := 10

	// Chart area dimensions
	chartWidth := width - yLabelWidth - 2
	if chartWidth < 10 {
		chartWidth = 10
	}

	// Handle constant values
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
		minVal = minVal - 0.5
		maxVal = maxVal + 0.5
	}

	// Sample buckets to fit chart width
	sampleBuckets := make([]float64, chartWidth)
	step := float64(len(buckets)) / float64(chartWidth)
	for i := 0; i < chartWidth; i++ {
		idx := int(float64(i) * step)
		if idx >= len(buckets) {
			idx = len(buckets) - 1
		}
		sampleBuckets[i] = buckets[idx].Value
	}

	// Render chart rows (top to bottom)
	for row := height - 1; row >= 0; row-- {
		// Y-axis label
		rowValue := minVal + (valRange * float64(row) / float64(height-1))
		yLabel := formatMetricValue(rowValue)
		b.WriteString(DetailMutedStyle.Render(PadLeft(yLabel, yLabelWidth)))
		b.WriteString(" │")

		// Chart row
		for col := 0; col < chartWidth; col++ {
			val := sampleBuckets[col]
			// Normalize to 0..height-1
			normalized := (val - minVal) / valRange * float64(height-1)
			valRow := int(normalized)

			if valRow == row {
				// This is the data point
				b.WriteString(SparklineStyle.Render("█"))
			} else if valRow > row {
				// Value is above this row - show bar
				b.WriteString(SparklineStyle.Render("│"))
			} else {
				// Value is below this row - empty
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}

	// X-axis
	b.WriteString(strings.Repeat(" ", yLabelWidth))
	b.WriteString(" └")
	b.WriteString(strings.Repeat("─", chartWidth))
	b.WriteString("\n")

	// X-axis labels (start and end times)
	if len(buckets) > 0 {
		startTime := buckets[0].Timestamp.Format("15:04:05")
		endTime := buckets[len(buckets)-1].Timestamp.Format("15:04:05")
		padding := chartWidth - len(startTime) - len(endTime)
		if padding < 0 {
			padding = 0
		}
		b.WriteString(strings.Repeat(" ", yLabelWidth+2))
		b.WriteString(DetailMutedStyle.Render(startTime))
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(DetailMutedStyle.Render(endTime))
	}

	return b.String()
}

// PadLeft pads a string to the left to reach the specified width
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

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
	// TRANSACTION NAME (flex) | COUNT (10) | AVG(ms) (12) | ERR% (8)
	countWidth := 10
	avgWidth := 12
	errWidth := 8
	fixedWidth := countWidth + avgWidth + errWidth + 4 // separators
	nameWidth := m.width - fixedWidth - 10
	if nameWidth < 20 {
		nameWidth = 20
	}

	// Header
	header := HeaderRowStyle.Render(
		PadOrTruncate("TRANSACTION NAME", nameWidth) + " " +
			PadOrTruncate("COUNT", countWidth) + " " +
			PadOrTruncate("AVG(ms)", avgWidth) + " " +
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
		avgStr := fmt.Sprintf("%.2f", tx.AvgDuration)
		errStr := fmt.Sprintf("%.1f%%", tx.ErrorRate)

		line := PadOrTruncate(tx.Name, nameWidth) + " " +
			PadOrTruncate(countStr, countWidth) + " " +
			PadOrTruncate(avgStr, avgWidth) + " " +
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

		// Format values
		logsStr := fmt.Sprintf("%d", item.LogCount)
		tracesStr := fmt.Sprintf("%d", item.TraceCount)
		metricsStr := fmt.Sprintf("%d", item.MetricCount)

		line := PadOrTruncate(item.Name, nameWidth) + " " +
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

func (m Model) renderHelpBar() string {
	var keys []string

	switch m.mode {
	case viewLogs:
		keys = []string{
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" signal"),
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" lookback"),
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" search"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" details"),
			HelpKeyStyle.Render("s") + HelpDescStyle.Render(" sort"),
			HelpKeyStyle.Render("f") + HelpDescStyle.Render(" fields"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" clear"),
			HelpKeyStyle.Render("Q") + HelpDescStyle.Render(" query"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
		// Add 'd' for dashboard when viewing metrics documents
		if m.signalType == signalMetrics && m.metricsViewMode == metricsViewDocuments {
			keys = append([]string{HelpKeyStyle.Render("d") + HelpDescStyle.Render(" dashboard")}, keys...)
		}
		// Add 'esc' for trace navigation
		if m.signalType == signalTraces && (m.traceViewLevel == traceViewTransactions || m.traceViewLevel == traceViewSpans) {
			keys = append([]string{HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back")}, keys...)
		}
	case viewSearch:
		keys = []string{
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" search"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" cancel"),
		}
	case viewIndex:
		keys = []string{
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" apply"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" cancel"),
		}
	case viewQuery:
		keys = []string{
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("k") + HelpDescStyle.Render(" Kibana"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" curl"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	case viewFields:
		keys = []string{
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("space") + HelpDescStyle.Render(" toggle"),
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" search"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" reset"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	case viewDetail:
		keys = []string{
			HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" prev/next"),
			HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("j") + HelpDescStyle.Render(" JSON"),
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
		// Add 's' for viewing spans when in traces
		if m.signalType == signalTraces {
			keys = append([]string{HelpKeyStyle.Render("s") + HelpDescStyle.Render(" spans")}, keys...)
		}
	case viewDetailJSON:
		keys = []string{
			HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" prev/next"),
			HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" details"),
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	case viewMetricsDashboard:
		keys = []string{
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" detail"),
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" lookback"),
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" documents"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" signal"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	case viewMetricDetail:
		keys = []string{
			HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" prev/next metric"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back to dashboard"),
		}
	case viewTraceNames:
		keys = []string{
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" select"),
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" lookback"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" signal"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	}

	return HelpStyle.Render(strings.Join(keys, "  "))
}

// Commands

