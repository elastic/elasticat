// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

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
