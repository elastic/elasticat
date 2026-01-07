// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/elastic/elasticat/internal/es"
)

// renderCompactDetail renders a compact detail view of the selected log at the bottom
func (m Model) renderCompactDetail() string {
	if len(m.logs) == 0 || m.selectedIndex >= len(m.logs) {
		return CompactDetailStyle.Width(m.width - 4).Height(4).Render(
			DetailMutedStyle.Render("No entry selected"),
		)
	}

	log := m.logs[m.selectedIndex]
	switch m.signalType {
	case signalTraces:
		return m.renderCompactDetailTraces(log)
	case signalMetrics:
		return m.renderCompactDetailMetrics(log)
	default:
		return m.renderCompactDetailLogs(log)
	}
}

func (m Model) renderCompactDetailLogs(log es.LogEntry) string {
	hl := m.Highlighter()
	var b strings.Builder

	m.writeBaseHeader(&b, log, hl, func() {
		level := log.GetLevel()
		b.WriteString("  ")
		b.WriteString(DetailKeyStyle.Render("Level: "))
		b.WriteString(hl.ApplyToField(level, LevelStyle(level)))
		if resource := log.GetResource(); resource != "" {
			b.WriteString("  ")
			b.WriteString(DetailKeyStyle.Render("Resource: "))
			b.WriteString(hl.ApplyToField(resource, DetailValueStyle))
		}
	})

	b.WriteString("\n")
	b.WriteString(DetailKeyStyle.Render("Message: "))
	msg := singleLine(log.GetMessage())
	b.WriteString(hl.ApplyToField(msg, DetailValueStyle))
	b.WriteString("\n")

	if len(log.Attributes) > 0 {
		b.WriteString(DetailKeyStyle.Render("Attrs: "))
		attrs := formatKVPreview(log.Attributes, 5, 0)
		b.WriteString(hl.ApplyToField(attrs, DetailMutedStyle))
	}

	return CompactDetailStyle.Width(m.width - 4).Height(compactDetailHeight).Render(b.String())
}

func (m Model) renderCompactDetailTraces(log es.LogEntry) string {
	hl := m.Highlighter()
	var b strings.Builder

	m.writeBaseHeader(&b, log, hl, func() {
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
		if code, ok := log.Status["code"].(string); ok {
			b.WriteString("  ")
			b.WriteString(DetailKeyStyle.Render("Status: "))
			b.WriteString(DetailValueStyle.Render(code))
		}
	})

	b.WriteString("\n")
	b.WriteString(DetailKeyStyle.Render("Name: "))
	name := log.Name
	if name == "" {
		name = log.GetMessage()
	}
	name = singleLine(name)
	b.WriteString(hl.ApplyToField(name, DetailValueStyle))
	b.WriteString("\n")

	if log.TraceID != "" {
		b.WriteString(DetailKeyStyle.Render("Trace: "))
		b.WriteString(DetailMutedStyle.Render(log.TraceID))
		if log.SpanID != "" {
			b.WriteString("  ")
			b.WriteString(DetailKeyStyle.Render("Span: "))
			b.WriteString(DetailMutedStyle.Render(log.SpanID))
		}
	}

	b.WriteString("\n")
	if m.spansLoading {
		b.WriteString(DetailKeyStyle.Render("Spans: "))
		b.WriteString(LoadingStyle.Render("Loading..."))
	} else if len(m.spans) > 0 {
		b.WriteString(DetailKeyStyle.Render(fmt.Sprintf("Spans (%d): ", len(m.spans))))
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
		b.WriteString(DetailValueStyle.Render(spansStr))
	} else if log.TraceID != "" {
		b.WriteString(DetailKeyStyle.Render("Spans: "))
		b.WriteString(DetailMutedStyle.Render("No child spans"))
	}

	return CompactDetailStyle.Width(m.width - 4).Height(compactDetailHeight).Render(b.String())
}

func (m Model) renderCompactDetailMetrics(log es.LogEntry) string {
	hl := m.Highlighter()
	var b strings.Builder

	m.writeBaseHeader(&b, log, hl, func() {
		if scopeName, ok := log.Scope["name"].(string); ok && scopeName != "" {
			b.WriteString("  ")
			b.WriteString(DetailKeyStyle.Render("Scope: "))
			b.WriteString(DetailValueStyle.Render(scopeName))
		}
	})

	b.WriteString("\n")
	b.WriteString(DetailKeyStyle.Render("Metrics: "))
	if len(log.Metrics) > 0 {
		metricsStr := formatKVPreview(log.Metrics, 8, 0)
		b.WriteString(hl.ApplyToField(metricsStr, DetailValueStyle))
	} else {
		b.WriteString(DetailMutedStyle.Render("No metrics data"))
	}
	b.WriteString("\n")

	if len(log.Attributes) > 0 {
		b.WriteString(DetailKeyStyle.Render("Attrs: "))
		attrs := formatKVPreview(log.Attributes, 5, 0)
		b.WriteString(hl.ApplyToField(attrs, DetailMutedStyle))
	}

	return CompactDetailStyle.Width(m.width - 4).Height(compactDetailHeight).Render(b.String())
}

func (m Model) writeBaseHeader(b *strings.Builder, log es.LogEntry, hl *Highlighter, appendExtras func()) {
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

	if appendExtras != nil {
		appendExtras()
	}
}

func truncateWithEllipsis(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func singleLine(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
}

// formatKVPreview renders up to maxItems sorted keys. If maxLen > 0, the result is truncated.
func formatKVPreview(m map[string]interface{}, maxItems, maxLen int) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, maxItems)
	for i, k := range keys {
		if i >= maxItems {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%v", k, m[k]))
	}

	result := strings.Join(parts, ", ")
	if maxLen <= 0 {
		return result
	}
	return truncateWithEllipsis(result, maxLen)
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
		return DetailStyle.Height(contentHeight).Render(header + "\n" + content)
	}
	return DetailStyle.Height(contentHeight).Render(content)
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
