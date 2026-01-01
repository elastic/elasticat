package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/andrewvc/turbodevlog/internal/es"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.design/x/clipboard"
)

// View modes
type viewMode int

const (
	viewLogs viewMode = iota
	viewSearch
	viewDetail
	viewDetailJSON
	viewIndex
	viewQuery
	viewFields
)

// Query display format
type queryFormat int

const (
	formatKibana queryFormat = iota
	formatCurl
)

// DisplayField represents a field that can be shown in the log list
type DisplayField struct {
	Name         string   // ES field path for display (e.g., "severity_text", "body.text")
	Label        string   // Display label (e.g., "LEVEL", "MESSAGE")
	Width        int      // Column width (0 = flexible/remaining)
	Selected     bool     // Whether this field is currently displayed
	SearchFields []string // ES fields to search (nil = not searchable, empty = use Name)
}

// Highlighter encapsulates search highlighting state and behavior
type Highlighter struct {
	Query string // Current search query (empty = no highlighting)
}

// NewHighlighter creates a highlighter with the given search query
func NewHighlighter(query string) *Highlighter {
	return &Highlighter{Query: query}
}

// IsActive returns true if highlighting is enabled (has a query)
func (h *Highlighter) IsActive() bool {
	return h != nil && h.Query != ""
}

// Apply extracts and highlights text, returning styled output with exact width
// If no match or no query, returns normally truncated text with base style
func (h *Highlighter) Apply(text string, maxWidth int, baseStyle lipgloss.Style) string {
	// Set MaxWidth on style to prevent any overflow/scrolling behavior
	style := baseStyle.MaxWidth(maxWidth)

	if !h.IsActive() {
		// Pad to exact width for alignment
		padded := PadOrTruncate(text, maxWidth)
		return style.Render(padded)
	}
	extracted, start, end := ExtractWithHighlight(text, h.Query, maxWidth)
	// Pad result to exact width for alignment
	padded := PadOrTruncate(extracted, maxWidth)
	// Adjust end position if padding was added
	if end > len(padded) {
		end = len(padded)
	}
	return RenderWithHighlight(padded, start, end, style)
}

// ApplyToField highlights text without truncation (for detail views)
func (h *Highlighter) ApplyToField(text string, baseStyle lipgloss.Style) string {
	if !h.IsActive() {
		return baseStyle.Render(text)
	}
	// Find and highlight all occurrences
	return h.highlightAll(text, baseStyle)
}

// highlightAll highlights all occurrences of the query in text
func (h *Highlighter) highlightAll(text string, baseStyle lipgloss.Style) string {
	if h.Query == "" {
		return baseStyle.Render(text)
	}

	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(h.Query)
	queryLen := len(h.Query)

	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(textLower[lastEnd:], queryLower)
		if idx == -1 {
			// No more matches - append rest of text
			result.WriteString(baseStyle.Render(text[lastEnd:]))
			break
		}

		matchStart := lastEnd + idx
		matchEnd := matchStart + queryLen

		// Append text before match
		if matchStart > lastEnd {
			result.WriteString(baseStyle.Render(text[lastEnd:matchStart]))
		}

		// Append highlighted match
		result.WriteString(HighlightStyle.Render(text[matchStart:matchEnd]))

		lastEnd = matchEnd
	}

	return result.String()
}

// GetSearchFields returns the ES field names to use for searching this field
func (f DisplayField) GetSearchFields() []string {
	if f.SearchFields == nil {
		return nil // Not searchable (e.g., @timestamp)
	}
	if len(f.SearchFields) == 0 {
		return []string{f.Name} // Use display name as search field
	}
	return f.SearchFields
}

// CollectSearchFields gathers all unique ES field names from display fields for searching
func CollectSearchFields(fields []DisplayField) []string {
	seen := make(map[string]bool)
	var result []string

	for _, f := range fields {
		for _, sf := range f.GetSearchFields() {
			if !seen[sf] {
				seen[sf] = true
				result = append(result, sf)
			}
		}
	}

	return result
}

// DefaultFields returns the default field configuration
func DefaultFields() []DisplayField {
	return []DisplayField{
		{Name: "@timestamp", Label: "TIME", Width: 8, Selected: true, SearchFields: nil}, // Not text-searchable
		{Name: "severity_text", Label: "LEVEL", Width: 7, Selected: true, SearchFields: []string{"severity_text", "level"}},
		{Name: "_resource", Label: "RESOURCE", Width: 12, Selected: true, SearchFields: []string{"resource.attributes.service.namespace", "resource.attributes.deployment.environment"}},
		{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"resource.attributes.service.name", "service.name"}},
		{Name: "body.text", Label: "MESSAGE", Width: 0, Selected: true, SearchFields: []string{"body.text", "body", "message", "event_name"}},
	}
}

// Model is the main TUI model
type Model struct {
	// ES client
	client *es.Client

	// State
	logs          []es.LogEntry
	selectedIndex int
	mode          viewMode
	err           error
	loading       bool
	total         int64

	// Filters
	serviceFilter string
	levelFilter   string
	searchQuery   string

	// Auto-refresh
	autoRefresh   bool
	refreshTicker *time.Ticker

	// Time display
	relativeTime bool // Show "2m ago" vs "15:04:05"

	// Sort order
	sortAscending bool // false = newest first (desc), true = oldest first (asc)

	// Query display
	lastQueryJSON  string      // Last ES query body as JSON
	lastQueryIndex string      // Index pattern used
	queryFormat    queryFormat // Kibana or curl format
	statusMessage  string      // Temporary status message (e.g., "Copied!")
	statusTime     time.Time   // When status was set (for auto-clear)

	// Field selection
	displayFields    []DisplayField  // Currently configured display fields
	availableFields  []es.FieldInfo  // All fields from field_caps
	fieldsCursor     int             // Cursor position in field selector
	fieldsLoading    bool            // Loading field caps
	fieldsSearchMode bool            // Whether we're in search mode within fields view
	fieldsSearch     string          // Search filter for fields

	// Components
	searchInput textinput.Model
	indexInput  textinput.Model
	viewport    viewport.Model

	// Dimensions
	width  int
	height int

	// Last refresh time
	lastRefresh time.Time
}

// Messages
type (
	logsMsg struct {
		logs      []es.LogEntry
		total     int64
		err       error
		queryJSON string
		index     string
	}
	fieldCapsMsg struct {
		fields []es.FieldInfo
		err    error
	}
	tickMsg   time.Time
	errMsg    error
	statusMsg string
)

// Highlighter returns a Highlighter configured with the current search query
func (m Model) Highlighter() *Highlighter {
	return NewHighlighter(m.searchQuery)
}

// NewModel creates a new TUI model
func NewModel(client *es.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "Search logs... (supports ES query syntax)"
	ti.CharLimit = 256
	ti.Width = 50

	ii := textinput.New()
	ii.Placeholder = "Index pattern (e.g., logs, logs-myapp)"
	ii.CharLimit = 128
	ii.Width = 50
	ii.SetValue(client.GetIndex())

	vp := viewport.New(80, 20)

	return Model{
		client:        client,
		logs:          []es.LogEntry{},
		mode:          viewLogs,
		autoRefresh:   true,
		displayFields: DefaultFields(),
		searchInput:   ti,
		indexInput:    ii,
		viewport:      vp,
		width:         80,
		height:        24,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchLogs(),
		m.tickCmd(),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		return m, nil

	case logsMsg:
		m.loading = false
		m.lastRefresh = time.Now()
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.logs = msg.logs
			m.total = msg.total
			m.err = nil
			m.lastQueryJSON = msg.queryJSON
			m.lastQueryIndex = msg.index
		}
		return m, nil

	case tickMsg:
		if m.autoRefresh && m.mode == viewLogs {
			cmds = append(cmds, m.fetchLogs())
		}
		cmds = append(cmds, m.tickCmd())
		return m, tea.Batch(cmds...)

	case fieldCapsMsg:
		m.fieldsLoading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.availableFields = msg.fields
		}
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil
	}

	// Update components based on mode
	switch m.mode {
	case viewSearch:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewIndex:
		var cmd tea.Cmd
		m.indexInput, cmd = m.indexInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

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
	}

	return m, nil
}

func (m Model) handleLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "down", "j":
		if m.selectedIndex < len(m.logs)-1 {
			m.selectedIndex++
		}
	case "home", "g":
		m.selectedIndex = 0
	case "end", "G":
		if len(m.logs) > 0 {
			m.selectedIndex = len(m.logs) - 1
		}
	case "pgup":
		m.selectedIndex -= 10
		if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
	case "pgdown":
		m.selectedIndex += 10
		if m.selectedIndex >= len(m.logs) {
			m.selectedIndex = len(m.logs) - 1
		}
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
	case "c":
		// Clear filters
		m.searchQuery = ""
		m.serviceFilter = ""
		m.levelFilter = ""
		m.loading = true
		return m, m.fetchLogs()
	case "1":
		m.levelFilter = "ERROR"
		m.loading = true
		return m, m.fetchLogs()
	case "2":
		m.levelFilter = "WARN"
		m.loading = true
		return m, m.fetchLogs()
	case "3":
		m.levelFilter = "INFO"
		m.loading = true
		return m, m.fetchLogs()
	case "4":
		m.levelFilter = "DEBUG"
		m.loading = true
		return m, m.fetchLogs()
	case "0":
		m.levelFilter = ""
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
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
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
		// Reset to defaults
		m.displayFields = DefaultFields()
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

// View renders the TUI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := HeaderStyle.Render("TurboDevLog")
	b.WriteString(header)
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// Main content based on mode
	switch m.mode {
	case viewLogs, viewSearch, viewIndex, viewQuery:
		b.WriteString(m.renderLogList())
		b.WriteString("\n")
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
	}

	// Help bar
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return AppStyle.Render(b.String())
}

func (m Model) renderStatusBar() string {
	var parts []string

	// Connection status
	if m.err != nil {
		parts = append(parts, ErrorStyle.Render("ES: disconnected"))
	} else {
		parts = append(parts, StatusKeyStyle.Render("ES: ")+StatusValueStyle.Render("connected"))
	}

	// Current index
	parts = append(parts, StatusKeyStyle.Render("Index: ")+StatusValueStyle.Render(m.client.GetIndex()+"*"))

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
	if m.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if len(m.logs) == 0 {
		return LoadingStyle.Render("No logs found. Waiting for data...")
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

	// Calculate visible range (account for header row + detail panel at bottom)
	detailPanelHeight := 6
	visibleHeight := m.height - 9 - detailPanelHeight
	if visibleHeight < 5 {
		visibleHeight = 5
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
	return LogListStyle.Width(m.width - 4).Render(content)
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
			DetailMutedStyle.Render("No log selected"),
		)
	}

	log := m.logs[m.selectedIndex]
	hl := m.Highlighter()
	var b strings.Builder

	// First line: timestamp, level, service, resource
	ts := log.Timestamp.Format("2006-01-02 15:04:05.000")
	level := log.GetLevel()
	service := log.ServiceName
	if service == "" {
		service = "unknown"
	}
	resource := log.GetResource()

	b.WriteString(DetailKeyStyle.Render("Time: "))
	b.WriteString(DetailValueStyle.Render(ts))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Level: "))
	b.WriteString(hl.ApplyToField(level, LevelStyle(level)))
	b.WriteString("  ")
	b.WriteString(DetailKeyStyle.Render("Service: "))
	b.WriteString(hl.ApplyToField(service, DetailValueStyle))
	if resource != "" {
		b.WriteString("  ")
		b.WriteString(DetailKeyStyle.Render("Resource: "))
		b.WriteString(hl.ApplyToField(resource, DetailValueStyle))
	}
	b.WriteString("\n")

	// Second line: full message with highlighting
	msg := log.GetMessage()
	msg = strings.ReplaceAll(msg, "\n", " ") // Flatten newlines
	maxMsgLen := m.width - 10
	b.WriteString(DetailKeyStyle.Render("Message: "))
	b.WriteString(hl.Apply(msg, maxMsgLen, DetailValueStyle))
	b.WriteString("\n")

	// Third line: key attributes (if any)
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

	return CompactDetailStyle.Width(m.width - 4).Render(b.String())
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
	var headerText string
	if m.mode == viewDetailJSON {
		headerText = "Raw JSON"
	} else {
		headerText = "Log Details"
	}

	header := DetailKeyStyle.Render(headerText)

	// Show status message if recent (within 2 seconds)
	if m.statusMessage != "" && time.Since(m.statusTime) < 2*time.Second {
		header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true).Render(m.statusMessage)
	}

	content := m.viewport.View()
	return DetailStyle.Width(m.width - 4).Height(m.height - 6).Render(header + "\n\n" + content)
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

func (m Model) renderLogDetail(log es.LogEntry) string {
	hl := m.Highlighter()
	var b strings.Builder

	// Basic fields
	b.WriteString(DetailKeyStyle.Render("Timestamp: "))
	b.WriteString(DetailValueStyle.Render(log.Timestamp.Format(time.RFC3339Nano)))
	b.WriteString("\n\n")

	b.WriteString(DetailKeyStyle.Render("Level: "))
	b.WriteString(hl.ApplyToField(log.GetLevel(), LevelStyle(log.GetLevel())))
	b.WriteString("\n\n")

	if log.ServiceName != "" {
		b.WriteString(DetailKeyStyle.Render("Service: "))
		b.WriteString(hl.ApplyToField(log.ServiceName, DetailValueStyle))
		b.WriteString("\n\n")
	}

	if log.ContainerID != "" {
		b.WriteString(DetailKeyStyle.Render("Container: "))
		b.WriteString(hl.ApplyToField(log.ContainerID, DetailValueStyle))
		b.WriteString("\n\n")
	}

	b.WriteString(DetailKeyStyle.Render("Message:"))
	b.WriteString("\n")
	b.WriteString(hl.ApplyToField(log.GetMessage(), DetailValueStyle))
	b.WriteString("\n\n")

	// Attributes
	if len(log.Attributes) > 0 {
		b.WriteString(DetailKeyStyle.Render("Attributes:"))
		b.WriteString("\n")
		for k, v := range log.Attributes {
			valStr := fmt.Sprintf("%v", v)
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, hl.ApplyToField(valStr, DetailValueStyle)))
		}
		b.WriteString("\n")
	}

	// Resource
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
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" search"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" details"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("s") + HelpDescStyle.Render(" sort"),
			HelpKeyStyle.Render("a") + HelpDescStyle.Render(" auto"),
			HelpKeyStyle.Render("t") + HelpDescStyle.Render(" time"),
			HelpKeyStyle.Render("1-4") + HelpDescStyle.Render(" level"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" clear"),
			HelpKeyStyle.Render("i") + HelpDescStyle.Render(" index"),
			HelpKeyStyle.Render("f") + HelpDescStyle.Render(" fields"),
			HelpKeyStyle.Render("Q") + HelpDescStyle.Render(" query"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
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
			HelpKeyStyle.Render("up/dn") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("j") + HelpDescStyle.Render(" JSON"),
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	case viewDetailJSON:
		keys = []string{
			HelpKeyStyle.Render("up/dn") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" details"),
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	}

	return HelpStyle.Render(strings.Join(keys, "  "))
}

// Commands

func (m Model) fetchLogs() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var result *es.SearchResult
		var err error
		var queryJSON string
		index := m.client.GetIndex()

		if m.searchQuery != "" {
			opts := es.SearchOptions{
				Size:         100,
				Service:      m.serviceFilter,
				Level:        m.levelFilter,
				SortAsc:      m.sortAscending,
				SearchFields: CollectSearchFields(m.displayFields),
			}
			result, err = m.client.Search(ctx, m.searchQuery, opts)
			queryJSON, _ = m.client.GetSearchQueryJSON(m.searchQuery, opts)
		} else {
			opts := es.TailOptions{
				Size:    100,
				Service: m.serviceFilter,
				Level:   m.levelFilter,
				SortAsc: m.sortAscending,
			}
			result, err = m.client.Tail(ctx, opts)
			queryJSON, _ = m.client.GetTailQueryJSON(opts)
		}

		if err != nil {
			return logsMsg{err: err}
		}

		return logsMsg{logs: result.Logs, total: result.Total, queryJSON: queryJSON, index: index}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchFieldCaps() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fields, err := m.client.GetFieldCaps(ctx)
		if err != nil {
			return fieldCapsMsg{err: err}
		}

		return fieldCapsMsg{fields: fields}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
