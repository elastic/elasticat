package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
	"golang.design/x/clipboard"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		if m.mode == viewLogs {
			return m, tea.Quit
		}
		// Exit current mode
		m.mode = viewLogs
		m.searchInput.Blur()
		return m, nil

	case "esc":
		// Close error modal and return to previous view
		if m.mode == viewErrorModal {
			m.mode = m.previousMode
			m.err = nil // Clear error
			return m, nil
		}
		// Let metric and trace views handle their own escape
		if m.mode == viewMetricDetail || m.mode == viewMetricsDashboard || m.mode == viewTraceNames {
			break // Fall through to mode-specific handler
		}
		if m.mode != viewLogs {
			m.mode = viewLogs
			m.searchInput.Blur()
			return m, nil
		}
	}

	// Mode-specific keys
	switch m.mode {
	case viewLogs:
		return m.handleLogsKey(msg)
	case viewSearch:
		return m.handleSearchKey(msg)
	case viewDetail, viewDetailJSON:
		return m.handleDetailKey(msg)
	case viewIndex:
		return m.handleIndexKey(msg)
	case viewQuery:
		return m.handleQueryKey(msg)
	case viewFields:
		return m.handleFieldsKey(msg)
	case viewMetricsDashboard:
		return m.handleMetricsDashboardKey(msg)
	case viewMetricDetail:
		return m.handleMetricDetailKey(msg)
	case viewTraceNames:
		return m.handleTraceNamesKey(msg)
	case viewPerspectiveList:
		return m.handlePerspectiveListKey(msg)
	case viewErrorModal:
		return m.handleErrorModalKey(msg)
	}

	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel scrolling
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		switch m.mode {
		case viewLogs:
			// Scroll up in log list (2 items at a time for speed)
			if m.selectedIndex > 0 {
				m.selectedIndex -= 2
				if m.selectedIndex < 0 {
					m.selectedIndex = 0
				}
			}
		case viewDetail, viewDetailJSON:
			// Scroll up in detail viewport
			m.viewport.LineUp(3)
		case viewFields:
			// Scroll up in field selector
			if m.fieldsCursor > 0 {
				m.fieldsCursor -= 2
				if m.fieldsCursor < 0 {
					m.fieldsCursor = 0
				}
			}
		case viewMetricsDashboard:
			// Scroll up in metrics dashboard
			if m.metricsCursor > 0 {
				m.metricsCursor -= 2
				if m.metricsCursor < 0 {
					m.metricsCursor = 0
				}
			}
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		switch m.mode {
		case viewLogs:
			// Scroll down in log list (2 items at a time for speed)
			if m.selectedIndex < len(m.logs)-1 {
				m.selectedIndex += 2
				if m.selectedIndex >= len(m.logs) {
					m.selectedIndex = len(m.logs) - 1
				}
			}
		case viewDetail, viewDetailJSON:
			// Scroll down in detail viewport
			m.viewport.LineDown(3)
		case viewFields:
			// Scroll down in field selector
			sortedFields := m.getSortedFieldList()
			if m.fieldsCursor < len(sortedFields)-1 {
				m.fieldsCursor += 2
				if m.fieldsCursor >= len(sortedFields) {
					m.fieldsCursor = len(sortedFields) - 1
				}
			}
		case viewMetricsDashboard:
			// Scroll down in metrics dashboard
			if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics)-1 {
				m.metricsCursor += 2
				if m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
					m.metricsCursor = len(m.aggregatedMetrics.Metrics) - 1
				}
			}
		}
		return m, nil
	}

	// Handle left clicks
	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionRelease {
		return m, nil
	}

	// Only handle clicks in log list mode on the first row (status bar)
	if m.mode == viewLogs && msg.Y == 0 {
		// Calculate the approximate position of the "Sort:" label in the status bar
		// The status bar contains: Signal, ES, Idx, Total, [Query], [Level], [Service], Sort, Auto
		sortStart, sortEnd := m.getSortLabelPosition()
		if msg.X >= sortStart && msg.X <= sortEnd {
			// Toggle sort order
			m.sortAscending = !m.sortAscending
			m.loading = true
			return m, m.fetchLogs()
		}
	}

	return m, nil
}

// getSortLabelPosition returns the approximate start and end X positions of the "Sort:" label
func (m Model) getSortLabelPosition() (start, end int) {
	// Build the status bar parts to calculate position
	// Note: This mirrors the logic in renderStatusBar but just calculates lengths
	pos := 1 // Start after padding

	// Signal: <type>
	pos += len("Signal: ") + len(m.signalType.String()) + 5 // + separator

	// ES: ok/err
	if m.err != nil {
		pos += len("ES: err") + 5
	} else {
		pos += len("ES: ok") + 5
	}

	// Idx: <index>*
	pos += len("Idx: ") + len(m.client.GetIndex()) + 1 + 5 // +1 for *, +5 for separator

	// Total: <count>
	pos += len("Total: ") + len(fmt.Sprintf("%d", m.total)) + 5

	// Optional Query filter
	if m.searchQuery != "" {
		displayed := TruncateWithEllipsis(m.searchQuery, 20)
		pos += len("Query: ") + len(displayed) + 5
	}

	// Optional Level filter
	if m.levelFilter != "" {
		pos += len("Level: ") + len(m.levelFilter) + 5
	}

	// Optional Service filter
	if m.filterService != "" {
		pos += len("Service: ") + len(m.filterService) + 5
	}

	// Lookback
	pos += len("Lookback: ") + len(m.lookback.String()) + 5

	// Now we're at "Sort: "
	start = pos
	sortText := "newest→"
	if m.sortAscending {
		sortText = "oldest→"
	}
	end = start + len("Sort: ") + len(sortText)

	return start, end
}

func (m Model) handleLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// For traces, go back up the hierarchy
		if m.signalType == signalTraces {
			switch m.traceViewLevel {
			case traceViewSpans:
				// Go back to transactions list
				m.traceViewLevel = traceViewTransactions
				m.selectedTraceID = ""
				m.selectedIndex = 0
				m.loading = true
				return m, m.fetchLogs()
			case traceViewTransactions:
				// Go back to transaction names
				m.traceViewLevel = traceViewNames
				m.selectedTxName = ""
				m.mode = viewTraceNames
				m.tracesLoading = true
				return m, m.fetchTransactionNames()
			}
		}
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.userHasScrolled = true // User manually scrolled
			// Fetch spans for traces when selection changes
			if m.signalType == signalTraces && len(m.logs) > 0 {
				traceID := m.logs[m.selectedIndex].TraceID
				if traceID != "" {
					m.spansLoading = true
					return m, m.fetchSpans(traceID)
				}
			}
		}
	case "down", "j":
		if m.selectedIndex < len(m.logs)-1 {
			m.selectedIndex++
			m.userHasScrolled = true // User manually scrolled
			// Fetch spans for traces when selection changes
			if m.signalType == signalTraces && len(m.logs) > 0 {
				traceID := m.logs[m.selectedIndex].TraceID
				if traceID != "" {
					m.spansLoading = true
					return m, m.fetchSpans(traceID)
				}
			}
		}
	case "home", "g":
		m.selectedIndex = 0
		m.userHasScrolled = true // User manually scrolled
	case "end", "G":
		if len(m.logs) > 0 {
			m.selectedIndex = len(m.logs) - 1
		}
		m.userHasScrolled = true // User manually scrolled
	case "pgup":
		m.selectedIndex -= 10
		if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
		m.userHasScrolled = true // User manually scrolled
	case "pgdown":
		m.selectedIndex += 10
		if m.selectedIndex >= len(m.logs) {
			m.selectedIndex = len(m.logs) - 1
		}
		m.userHasScrolled = true // User manually scrolled
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "enter":
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			m.mode = viewDetail
			m.viewport.SetContent(m.renderLogDetail(m.logs[m.selectedIndex]))
			m.viewport.GotoTop()
		}
	case "r":
		m.loading = true
		return m, m.fetchLogs()
	case "a":
		m.autoRefresh = !m.autoRefresh
	case "1":
		m.levelFilter = "ERROR"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "2":
		m.levelFilter = "WARN"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "3":
		m.levelFilter = "INFO"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "4":
		m.levelFilter = "DEBUG"
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "0":
		m.levelFilter = ""
		m.userHasScrolled = false // Reset for tail -f behavior
		m.loading = true
		return m, m.fetchLogs()
	case "i":
		m.mode = viewIndex
		m.indexInput.SetValue(m.client.GetIndex())
		m.indexInput.Focus()
		return m, textinput.Blink
	case "t":
		m.relativeTime = !m.relativeTime
	case "Q":
		m.mode = viewQuery
		m.queryFormat = formatKibana
	case "f":
		m.mode = viewFields
		m.fieldsCursor = 0
		m.fieldsSearch = ""
		m.fieldsSearchMode = false
		m.fieldsLoading = true
		return m, m.fetchFieldCaps()
	case "s":
		m.sortAscending = !m.sortAscending
		m.loading = true
		return m, m.fetchLogs()
	case "l":
		// Cycle through lookback durations
		for i, lb := range lookbackDurations {
			if lb == m.lookback {
				m.lookback = lookbackDurations[(i+1)%len(lookbackDurations)]
				break
			}
		}
		m.loading = true
		return m, m.fetchLogs()
	case "m":
		// Cycle through signal types: logs -> traces -> metrics -> logs
		switch m.signalType {
		case signalLogs:
			m.signalType = signalTraces
		case signalTraces:
			m.signalType = signalMetrics
		case signalMetrics:
			m.signalType = signalLogs
		}
		// Update index pattern and reset fields for new signal type
		m.client.SetIndex(m.signalType.IndexPattern())
		m.displayFields = DefaultFields(m.signalType)
		m.logs = []es.LogEntry{}
		m.selectedIndex = 0
		m.statusMessage = "Auto-detecting time range..."
		m.statusTime = time.Now()

		// Signal-specific view modes
		switch m.signalType {
		case signalMetrics:
			m.metricsViewMode = metricsViewAggregated
			m.mode = viewMetricsDashboard
			m.metricsLoading = true
			m.metricsCursor = 0
			m.loading = false
		case signalTraces:
			m.traceViewLevel = traceViewNames
			m.mode = viewTraceNames
			m.tracesLoading = true
			m.traceNamesCursor = 0
			m.selectedTxName = ""
			m.selectedTraceID = ""
			m.loading = false
		default:
			m.mode = viewLogs
			m.loading = true
		}

		// Auto-detect the best lookback for the new signal type
		return m, m.autoDetectLookback()

	case "p":
		// Cycle through perspectives: Services -> Resources -> Services
		switch m.currentPerspective {
		case PerspectiveServices:
			m.currentPerspective = PerspectiveResources
		case PerspectiveResources:
			m.currentPerspective = PerspectiveServices
		}
		// Enter perspective list view
		m.mode = viewPerspectiveList
		m.perspectiveCursor = 0
		m.perspectiveItems = []PerspectiveItem{}
		m.perspectiveLoading = true
		m.statusMessage = fmt.Sprintf("Loading %s...", m.currentPerspective.String())
		m.statusTime = time.Now()
		return m, m.fetchPerspectiveData()

	case "d":
		// Toggle between document view and aggregated view for metrics
		if m.signalType == signalMetrics {
			if m.metricsViewMode == metricsViewAggregated {
				m.metricsViewMode = metricsViewDocuments
				m.mode = viewLogs
				m.loading = true
				return m, m.fetchLogs()
			} else {
				m.metricsViewMode = metricsViewAggregated
				m.mode = viewMetricsDashboard
				m.metricsLoading = true
				return m, m.fetchAggregatedMetrics()
			}
		}
	}

	return m, nil
}

func (m Model) handleQueryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "Q":
		m.mode = viewLogs
		m.statusMessage = ""
		return m, nil
	case "c":
		m.queryFormat = formatCurl
	case "k":
		m.queryFormat = formatKibana
	case "y":
		// Copy query to clipboard
		queryText := m.getQueryText()
		err := clipboard.Init()
		if err != nil {
			m.statusMessage = "Clipboard error: " + err.Error()
		} else {
			clipboard.Write(clipboard.FmtText, []byte(queryText))
			m.statusMessage = "Copied to clipboard!"
		}
		m.statusTime = time.Now()
	}
	return m, nil
}

// getQueryText returns the raw query text (without styling) for clipboard
func (m Model) getQueryText() string {
	index := m.lastQueryIndex + "*"

	if m.queryFormat == formatKibana {
		return fmt.Sprintf("GET %s/_search\n%s", index, m.lastQueryJSON)
	}

	// curl format
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(m.lastQueryJSON)); err != nil {
		return fmt.Sprintf("curl -X GET 'http://localhost:9200/%s/_search' \\\n  -H 'Content-Type: application/json' \\\n  -d '%s'",
			index, m.lastQueryJSON)
	}
	return fmt.Sprintf("curl -X GET 'http://localhost:9200/%s/_search' \\\n  -H 'Content-Type: application/json' \\\n  -d '%s'",
		index, compact.String())
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searchQuery = m.searchInput.Value()
		m.userHasScrolled = false // Reset for tail -f behavior
		m.mode = viewLogs
		m.searchInput.Blur()
		m.loading = true
		return m, m.fetchLogs()
	case "esc":
		m.mode = viewLogs
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) handleIndexKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newIndex := m.indexInput.Value()
		if newIndex != "" {
			m.client.SetIndex(newIndex)
		}
		m.mode = viewLogs
		m.indexInput.Blur()
		m.loading = true
		return m, m.fetchLogs()
	case "esc":
		m.mode = viewLogs
		m.indexInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.indexInput, cmd = m.indexInput.Update(msg)
	return m, cmd
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = viewLogs
		m.statusMessage = ""
		return m, nil
	case "left", "h":
		// Navigate to previous entry
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.updateDetailContent()
		}
		return m, nil
	case "right", "l":
		// Navigate to next entry
		if m.selectedIndex < len(m.logs)-1 {
			m.selectedIndex++
			m.updateDetailContent()
		}
		return m, nil
	case "enter":
		// Toggle between detail and JSON view
		if m.mode == viewDetail {
			m.mode = viewDetailJSON
			if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				m.viewport.SetContent(m.logs[m.selectedIndex].RawJSON)
				m.viewport.GotoTop()
			}
		} else {
			m.mode = viewDetail
			if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				m.viewport.SetContent(m.renderLogDetail(m.logs[m.selectedIndex]))
				m.viewport.GotoTop()
			}
		}
		return m, nil
	case "j":
		// Switch to JSON view
		m.mode = viewDetailJSON
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			m.viewport.SetContent(m.logs[m.selectedIndex].RawJSON)
			m.viewport.GotoTop()
		}
		return m, nil
	case "y":
		// Copy raw JSON to clipboard
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			err := clipboard.Init()
			if err != nil {
				m.statusMessage = "Clipboard error: " + err.Error()
			} else {
				clipboard.Write(clipboard.FmtText, []byte(m.logs[m.selectedIndex].RawJSON))
				m.statusMessage = "Copied JSON to clipboard!"
			}
			m.statusTime = time.Now()
		}
		return m, nil
	case "s":
		// Show spans for this trace (only for traces)
		if m.signalType == signalTraces && len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			log := m.logs[m.selectedIndex]
			if log.TraceID != "" {
				m.selectedTraceID = log.TraceID
				m.traceViewLevel = traceViewSpans
				m.mode = viewLogs
				m.selectedIndex = 0
				m.loading = true
				return m, m.fetchLogs()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// updateDetailContent refreshes the detail view content for the current selection
func (m *Model) updateDetailContent() {
	if len(m.logs) == 0 || m.selectedIndex >= len(m.logs) {
		return
	}
	if m.mode == viewDetailJSON {
		m.viewport.SetContent(m.logs[m.selectedIndex].RawJSON)
	} else {
		m.viewport.SetContent(m.renderLogDetail(m.logs[m.selectedIndex]))
	}
	m.viewport.GotoTop()
}

func (m Model) handleFieldsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If in search mode, handle text input
	if m.fieldsSearchMode {
		switch msg.String() {
		case "esc":
			m.fieldsSearchMode = false
			m.fieldsSearch = ""
			return m, nil
		case "enter":
			m.fieldsSearchMode = false
			return m, nil
		case "backspace":
			if len(m.fieldsSearch) > 0 {
				m.fieldsSearch = m.fieldsSearch[:len(m.fieldsSearch)-1]
			}
			return m, nil
		default:
			// Add character to search
			if len(msg.String()) == 1 {
				m.fieldsSearch += msg.String()
			}
			return m, nil
		}
	}

	// Get the sorted field list for navigation
	sortedFields := m.getSortedFieldList()

	switch msg.String() {
	case "esc", "q":
		m.mode = viewLogs
		return m, nil
	case "up", "k":
		if m.fieldsCursor > 0 {
			m.fieldsCursor--
		}
	case "down", "j":
		if m.fieldsCursor < len(sortedFields)-1 {
			m.fieldsCursor++
		}
	case "home", "g":
		m.fieldsCursor = 0
	case "end", "G":
		if len(sortedFields) > 0 {
			m.fieldsCursor = len(sortedFields) - 1
		}
	case "pgup":
		m.fieldsCursor -= 10
		if m.fieldsCursor < 0 {
			m.fieldsCursor = 0
		}
	case "pgdown":
		m.fieldsCursor += 10
		if m.fieldsCursor >= len(sortedFields) {
			m.fieldsCursor = len(sortedFields) - 1
		}
	case " ", "enter":
		// Toggle field selection
		if m.fieldsCursor < len(sortedFields) {
			fieldName := sortedFields[m.fieldsCursor].Name
			m.toggleField(fieldName)
		}
	case "/":
		m.fieldsSearchMode = true
		m.fieldsSearch = ""
	case "r":
		// Reset to defaults for current signal type
		m.displayFields = DefaultFields(m.signalType)
	}

	return m, nil
}

// toggleField toggles a field's selection state
func (m *Model) toggleField(fieldName string) {
	// Check if it's already in displayFields
	for i, f := range m.displayFields {
		if f.Name == fieldName {
			// Remove it
			m.displayFields = append(m.displayFields[:i], m.displayFields[i+1:]...)
			return
		}
	}

	// Add it as a new field
	label := fieldName
	// Use last part of field name as label
	if idx := strings.LastIndex(fieldName, "."); idx >= 0 {
		label = fieldName[idx+1:]
	}
	label = strings.ToUpper(label)
	if len(label) > 12 {
		label = label[:12]
	}

	// Determine if this field should be searchable
	// Skip timestamp-like and numeric fields
	var searchFields []string
	if !strings.Contains(fieldName, "timestamp") && !strings.Contains(fieldName, "time") {
		searchFields = []string{} // Empty slice means use Name as search field
	}

	m.displayFields = append(m.displayFields, DisplayField{
		Name:         fieldName,
		Label:        label,
		Width:        15, // Default width for custom fields
		Selected:     true,
		SearchFields: searchFields,
	})
}

// getSortedFieldList returns available fields sorted with selected/default fields first
func (m Model) getSortedFieldList() []es.FieldInfo {
	// Create a map of selected field names
	selectedNames := make(map[string]bool)
	for _, f := range m.displayFields {
		selectedNames[f.Name] = true
	}

	// Create a map of available fields for quick lookup
	availableByName := make(map[string]es.FieldInfo)
	for _, f := range m.availableFields {
		availableByName[f.Name] = f
	}

	// Filter available fields by search if active
	var filtered []es.FieldInfo
	for _, f := range m.availableFields {
		if m.fieldsSearch != "" {
			if !strings.Contains(strings.ToLower(f.Name), strings.ToLower(m.fieldsSearch)) {
				continue
			}
		}
		filtered = append(filtered, f)
	}

	// Also filter display fields by search
	var filteredDisplayFields []DisplayField
	for _, df := range m.displayFields {
		if m.fieldsSearch != "" {
			if !strings.Contains(strings.ToLower(df.Name), strings.ToLower(m.fieldsSearch)) {
				continue
			}
		}
		filteredDisplayFields = append(filteredDisplayFields, df)
	}

	// Sort: selected fields first (in display order), then others by doc count
	result := make([]es.FieldInfo, 0, len(filtered)+len(filteredDisplayFields))

	// First, add selected fields in their current display order
	// Try to find matching FieldInfo to get DocCount, otherwise create one
	for _, df := range filteredDisplayFields {
		if f, ok := availableByName[df.Name]; ok {
			// Found exact match - use it with its DocCount
			result = append(result, f)
		} else {
			// No exact match (virtual field like _resource, or different naming)
			// Try to find a related field for the count
			var docCount int64
			for _, searchField := range df.SearchFields {
				if f, ok := availableByName[searchField]; ok {
					if f.DocCount > docCount {
						docCount = f.DocCount
					}
				}
			}
			// Create a FieldInfo for this display field
			result = append(result, es.FieldInfo{
				Name:         df.Name,
				Type:         "display", // Mark as display-only field
				Searchable:   len(df.SearchFields) > 0 || len(df.GetSearchFields()) > 0,
				Aggregatable: false,
				DocCount:     docCount,
			})
		}
	}

	// Then add non-selected fields sorted by doc count (descending)
	var nonSelected []es.FieldInfo
	for _, f := range filtered {
		if !selectedNames[f.Name] {
			nonSelected = append(nonSelected, f)
		}
	}
	// Sort non-selected by DocCount descending (most popular first)
	for i := 0; i < len(nonSelected); i++ {
		for j := i + 1; j < len(nonSelected); j++ {
			if nonSelected[i].DocCount < nonSelected[j].DocCount {
				nonSelected[i], nonSelected[j] = nonSelected[j], nonSelected[i]
			}
		}
	}
	result = append(result, nonSelected...)

	return result
}

func (m Model) handleMetricsDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.metricsCursor > 0 {
			m.metricsCursor--
		}
	case "down", "j":
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics)-1 {
			m.metricsCursor++
		}
	case "home", "g":
		m.metricsCursor = 0
	case "end", "G":
		if m.aggregatedMetrics != nil && len(m.aggregatedMetrics.Metrics) > 0 {
			m.metricsCursor = len(m.aggregatedMetrics.Metrics) - 1
		}
	case "pgup":
		m.metricsCursor -= 10
		if m.metricsCursor < 0 {
			m.metricsCursor = 0
		}
	case "pgdown":
		m.metricsCursor += 10
		if m.aggregatedMetrics != nil && m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
			m.metricsCursor = len(m.aggregatedMetrics.Metrics) - 1
			if m.metricsCursor < 0 {
				m.metricsCursor = 0
			}
		}
	case "enter":
		// Enter detail view for the selected metric
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
			m.mode = viewMetricDetail
		}
	case "r":
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	case "d":
		// Switch to document view
		m.metricsViewMode = metricsViewDocuments
		m.mode = viewLogs
		m.loading = true
		return m, m.fetchLogs()
	case "p":
		// Cycle through perspectives: Services -> Resources -> Services
		switch m.currentPerspective {
		case PerspectiveServices:
			m.currentPerspective = PerspectiveResources
		case PerspectiveResources:
			m.currentPerspective = PerspectiveServices
		}
		// Enter perspective list view
		m.mode = viewPerspectiveList
		m.perspectiveCursor = 0
		m.perspectiveItems = []PerspectiveItem{}
		m.perspectiveLoading = true
		m.statusMessage = fmt.Sprintf("Loading %s...", m.currentPerspective.String())
		m.statusTime = time.Now()
		return m, m.fetchPerspectiveData()
	case "l":
		// Cycle lookback duration
		for i, lb := range lookbackDurations {
			if lb == m.lookback {
				m.lookback = lookbackDurations[(i+1)%len(lookbackDurations)]
				break
			}
		}
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	case "m":
		// Cycle through signal types: metrics -> logs -> traces -> metrics
		switch m.signalType {
		case signalLogs:
			m.signalType = signalTraces
		case signalTraces:
			m.signalType = signalMetrics
		case signalMetrics:
			m.signalType = signalLogs
		}
		// Update index pattern and reset fields for new signal type
		m.client.SetIndex(m.signalType.IndexPattern())
		m.displayFields = DefaultFields(m.signalType)
		m.logs = []es.LogEntry{}
		m.selectedIndex = 0
		m.statusMessage = "Auto-detecting time range..."
		m.statusTime = time.Now()

		// Signal-specific view modes
		switch m.signalType {
		case signalMetrics:
			m.mode = viewMetricsDashboard
			m.metricsLoading = true
			return m, tea.Batch(m.autoDetectLookback(), m.fetchAggregatedMetrics())
		case signalTraces:
			m.mode = viewTraceNames
			m.traceViewLevel = traceViewNames
			m.tracesLoading = true
			return m, tea.Batch(m.autoDetectLookback(), m.fetchTransactionNames())
		default:
			m.mode = viewLogs
			m.loading = true
			return m, m.autoDetectLookback()
		}
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleMetricDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace", "q":
		// Return to metrics dashboard
		m.mode = viewMetricsDashboard
	case "left", "h":
		// Previous metric
		if m.metricsCursor > 0 {
			m.metricsCursor--
		}
	case "right", "l":
		// Next metric
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics)-1 {
			m.metricsCursor++
		}
	case "r":
		// Refresh
		m.metricsLoading = true
		return m, m.fetchAggregatedMetrics()
	}

	return m, nil
}

func (m Model) handleTraceNamesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.traceNamesCursor > 0 {
			m.traceNamesCursor--
		}
	case "down", "j":
		if m.traceNamesCursor < len(m.transactionNames)-1 {
			m.traceNamesCursor++
		}
	case "home", "g":
		m.traceNamesCursor = 0
	case "end", "G":
		if len(m.transactionNames) > 0 {
			m.traceNamesCursor = len(m.transactionNames) - 1
		}
	case "pgup":
		m.traceNamesCursor -= 10
		if m.traceNamesCursor < 0 {
			m.traceNamesCursor = 0
		}
	case "pgdown":
		m.traceNamesCursor += 10
		if m.traceNamesCursor >= len(m.transactionNames) {
			m.traceNamesCursor = len(m.transactionNames) - 1
			if m.traceNamesCursor < 0 {
				m.traceNamesCursor = 0
			}
		}
	case "enter":
		// Select transaction name and show transactions
		if len(m.transactionNames) > 0 && m.traceNamesCursor < len(m.transactionNames) {
			m.selectedTxName = m.transactionNames[m.traceNamesCursor].Name
			m.traceViewLevel = traceViewTransactions
			m.mode = viewLogs
			m.selectedIndex = 0
			m.loading = true
			return m, m.fetchLogs()
		}
	case "r":
		m.tracesLoading = true
		return m, m.fetchTransactionNames()
	case "p":
		// Cycle through perspectives: Services -> Resources -> Services
		switch m.currentPerspective {
		case PerspectiveServices:
			m.currentPerspective = PerspectiveResources
		case PerspectiveResources:
			m.currentPerspective = PerspectiveServices
		}
		// Enter perspective list view
		m.mode = viewPerspectiveList
		m.perspectiveCursor = 0
		m.perspectiveItems = []PerspectiveItem{}
		m.perspectiveLoading = true
		m.statusMessage = fmt.Sprintf("Loading %s...", m.currentPerspective.String())
		m.statusTime = time.Now()
		return m, m.fetchPerspectiveData()
	case "l":
		// Cycle lookback duration
		for i, lb := range lookbackDurations {
			if lb == m.lookback {
				m.lookback = lookbackDurations[(i+1)%len(lookbackDurations)]
				break
			}
		}
		m.tracesLoading = true
		return m, m.fetchTransactionNames()
	case "m":
		// Cycle through signal types: traces -> metrics -> logs -> traces
		switch m.signalType {
		case signalLogs:
			m.signalType = signalTraces
		case signalTraces:
			m.signalType = signalMetrics
		case signalMetrics:
			m.signalType = signalLogs
		}
		// Update index pattern and reset fields for new signal type
		m.client.SetIndex(m.signalType.IndexPattern())
		m.displayFields = DefaultFields(m.signalType)
		m.logs = []es.LogEntry{}
		m.selectedIndex = 0
		m.statusMessage = "Auto-detecting time range..."
		m.statusTime = time.Now()

		// Signal-specific view modes
		switch m.signalType {
		case signalMetrics:
			m.mode = viewMetricsDashboard
			m.metricsLoading = true
			return m, tea.Batch(m.autoDetectLookback(), m.fetchAggregatedMetrics())
		case signalTraces:
			m.mode = viewTraceNames
			m.traceViewLevel = traceViewNames
			m.tracesLoading = true
			return m, tea.Batch(m.autoDetectLookback(), m.fetchTransactionNames())
		default:
			m.mode = viewLogs
			m.loading = true
			return m, m.autoDetectLookback()
		}
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handlePerspectiveListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.perspectiveCursor > 0 {
			m.perspectiveCursor--
		}
		return m, nil
	case "down", "j":
		if m.perspectiveCursor < len(m.perspectiveItems)-1 {
			m.perspectiveCursor++
		}
		return m, nil
	case "enter":
		// Toggle the current perspective item filter
		if len(m.perspectiveItems) > 0 {
			selected := m.perspectiveItems[m.perspectiveCursor]

			// Check if this item is already active, and toggle it
			switch m.currentPerspective {
			case PerspectiveServices:
				if m.filterService == selected.Name {
					// Unset if already active
					m.filterService = ""
					m.statusMessage = fmt.Sprintf("Cleared service filter: %s", selected.Name)
				} else {
					// Set the filter
					m.filterService = selected.Name
					m.statusMessage = fmt.Sprintf("Filtered to service: %s", selected.Name)
				}
				m.userHasScrolled = false // Reset for tail -f behavior
			case PerspectiveResources:
				if m.filterResource == selected.Name {
					// Unset if already active
					m.filterResource = ""
					m.statusMessage = fmt.Sprintf("Cleared resource filter: %s", selected.Name)
				} else {
					// Set the filter
					m.filterResource = selected.Name
					m.statusMessage = fmt.Sprintf("Filtered to resource: %s", selected.Name)
				}
				m.userHasScrolled = false // Reset for tail -f behavior
			}
			m.statusTime = time.Now()

			// Stay in perspective view - user can navigate back with 'esc' when ready
		}
		return m, nil
	case "p":
		// Cycle to next perspective
		switch m.currentPerspective {
		case PerspectiveServices:
			m.currentPerspective = PerspectiveResources
		case PerspectiveResources:
			m.currentPerspective = PerspectiveServices
		}
		m.perspectiveCursor = 0
		m.perspectiveItems = []PerspectiveItem{}
		m.perspectiveLoading = true
		m.statusMessage = fmt.Sprintf("Loading %s...", m.currentPerspective.String())
		m.statusTime = time.Now()
		return m, m.fetchPerspectiveData()
	case "l":
		// Cycle lookback duration
		for i, lb := range lookbackDurations {
			if lb == m.lookback {
				m.lookback = lookbackDurations[(i+1)%len(lookbackDurations)]
				break
			}
		}
		m.perspectiveLoading = true
		return m, m.fetchPerspectiveData()
	case "r":
		// Refresh perspective data
		m.perspectiveLoading = true
		return m, m.fetchPerspectiveData()
	case "esc":
		// Return to appropriate view based on signal type
		switch m.signalType {
		case signalMetrics:
			m.mode = viewMetricsDashboard
		case signalTraces:
			m.mode = viewTraceNames
		default:
			m.mode = viewLogs
		}
		return m, nil
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleErrorModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "y":
		// Copy error to clipboard (consistent with 'y' in other views)
		if m.err != nil {
			err := clipboard.Init()
			if err != nil {
				m.statusMessage = "Clipboard error: " + err.Error()
			} else {
				clipboard.Write(clipboard.FmtText, []byte(m.err.Error()))
				m.statusMessage = "Error copied to clipboard!"
			}
			m.statusTime = time.Now()
		}
		return m, nil

	case "q", "esc":
		// Close modal and return to previous view
		m.mode = m.previousMode
		m.err = nil
		return m, nil

	case "j", "down":
		m.errorViewport.LineDown(1)
		return m, nil

	case "k", "up":
		m.errorViewport.LineUp(1)
		return m, nil

	case "d", "pgdown":
		m.errorViewport.HalfViewDown()
		return m, nil

	case "u", "pgup":
		m.errorViewport.HalfViewUp()
		return m, nil

	case "g", "home":
		m.errorViewport.GotoTop()
		return m, nil

	case "G", "end":
		m.errorViewport.GotoBottom()
		return m, nil
	}

	// Pass other keys to viewport for mouse wheel support
	m.errorViewport, cmd = m.errorViewport.Update(msg)
	return m, cmd
}

// Layout constants
const (
	statusBarHeight     = 1 // Usually one row (two when ES error)
	helpBarHeight       = 1
	compactDetailHeight = 5 // 3 lines of content + 2 for border
	layoutPadding       = 2 // Top/bottom padding from AppStyle
)

// View renders the TUI
