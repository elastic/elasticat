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
	viewMetricsDashboard // Aggregated metrics dashboard
	viewMetricDetail     // Full-screen metric chart
	viewTraceNames       // Aggregated transaction names for traces
)

// MetricsViewMode toggles between aggregated and document views for metrics
type MetricsViewMode int

const (
	metricsViewAggregated MetricsViewMode = iota // Default: sparklines + stats
	metricsViewDocuments                         // Legacy: individual documents
)

// TraceViewLevel represents the navigation level in the traces hierarchy
type TraceViewLevel int

const (
	traceViewNames        TraceViewLevel = iota // Aggregated transaction names
	traceViewTransactions                       // List of transactions with selected name
	traceViewSpans                              // All spans for a specific trace_id
)

// Sparkline characters (ordered by height)
var sparklineChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Query display format
type queryFormat int

const (
	formatKibana queryFormat = iota
	formatCurl
)

// SignalType represents the OTel signal type
type SignalType int

const (
	signalLogs SignalType = iota
	signalTraces
	signalMetrics
)

// LookbackDuration represents preset time ranges
type LookbackDuration int

const (
	lookback5m LookbackDuration = iota
	lookback1h
	lookback24h
	lookback1w
	lookbackAll
)

var lookbackDurations = []LookbackDuration{lookback5m, lookback1h, lookback24h, lookback1w, lookbackAll}

func (l LookbackDuration) String() string {
	switch l {
	case lookback5m:
		return "5m"
	case lookback1h:
		return "1h"
	case lookback24h:
		return "24h"
	case lookback1w:
		return "1w"
	default:
		return "all"
	}
}

func (l LookbackDuration) Duration() time.Duration {
	switch l {
	case lookback5m:
		return 5 * time.Minute
	case lookback1h:
		return time.Hour
	case lookback24h:
		return 24 * time.Hour
	case lookback1w:
		return 7 * 24 * time.Hour
	default:
		return 0 // "all" means no time filter
	}
}

func (l LookbackDuration) ESRange() string {
	switch l {
	case lookback5m:
		return "now-5m"
	case lookback1h:
		return "now-1h"
	case lookback24h:
		return "now-24h"
	case lookback1w:
		return "now-1w"
	default:
		return "" // No time filter
	}
}

func (s SignalType) String() string {
	switch s {
	case signalLogs:
		return "Logs"
	case signalTraces:
		return "Traces"
	case signalMetrics:
		return "Metrics"
	default:
		return "Unknown"
	}
}

func (s SignalType) IndexPattern() string {
	switch s {
	case signalLogs:
		return "logs"
	case signalTraces:
		return "traces"
	case signalMetrics:
		return "metrics"
	default:
		return "logs"
	}
}

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

// DefaultFields returns the default field configuration for a signal type
func DefaultFields(signal SignalType) []DisplayField {
	switch signal {
	case signalTraces:
		return []DisplayField{
			{Name: "@timestamp", Label: "TIME", Width: 8, Selected: true, SearchFields: nil},
			{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"resource.attributes.service.name", "service.name"}},
			{Name: "name", Label: "NAME", Width: 25, Selected: true, SearchFields: []string{"name"}},
			{Name: "duration_ms", Label: "DUR(ms)", Width: 9, Selected: true, SearchFields: nil},
			{Name: "status.code", Label: "STATUS", Width: 6, Selected: true, SearchFields: []string{"status.code"}},
			{Name: "kind", Label: "KIND", Width: 8, Selected: true, SearchFields: []string{"kind"}},
			{Name: "trace_id", Label: "TRACE", Width: 0, Selected: true, SearchFields: []string{"trace_id"}},
		}
	case signalMetrics:
		return []DisplayField{
			{Name: "@timestamp", Label: "TIME", Width: 8, Selected: true, SearchFields: nil},
			{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"resource.attributes.service.name", "service.name", "attributes.service.name"}},
			{Name: "scope.name", Label: "SCOPE", Width: 20, Selected: true, SearchFields: []string{"scope.name"}},
			{Name: "attributes.span.name", Label: "SPAN", Width: 25, Selected: true, SearchFields: []string{"attributes.span.name"}},
			{Name: "_metrics", Label: "METRICS", Width: 0, Selected: true, SearchFields: nil},
		}
	default: // signalLogs
		return []DisplayField{
			{Name: "@timestamp", Label: "TIME", Width: 8, Selected: true, SearchFields: nil},
			{Name: "severity_text", Label: "LEVEL", Width: 7, Selected: true, SearchFields: []string{"severity_text", "level"}},
			{Name: "_resource", Label: "RESOURCE", Width: 12, Selected: true, SearchFields: []string{"resource.attributes.service.namespace", "resource.attributes.deployment.environment"}},
			{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"resource.attributes.service.name", "service.name"}},
			{Name: "body.text", Label: "MESSAGE", Width: 0, Selected: true, SearchFields: []string{"body.text", "body", "message", "event_name"}},
		}
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

	// Signal type (logs, traces, metrics)
	signalType SignalType

	// Lookback duration (time range relative to now)
	lookback LookbackDuration

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

	// Metrics dashboard state
	metricsViewMode   MetricsViewMode
	aggregatedMetrics *es.MetricsAggResult
	metricsLoading    bool
	metricsCursor     int // Selected metric in dashboard

	// Traces navigation state
	traceViewLevel     TraceViewLevel           // Current navigation level
	transactionNames   []es.TransactionNameAgg  // Aggregated transaction names
	traceNamesCursor   int                      // Cursor in transaction names list
	selectedTxName     string                   // Selected transaction name filter
	selectedTraceID    string                   // Selected trace_id for spans view
	tracesLoading      bool                     // Loading transaction names
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
	autoDetectMsg struct {
		lookback LookbackDuration
		total    int64
		err      error
	}
	metricsAggMsg struct {
		result *es.MetricsAggResult
		err    error
	}
	transactionNamesMsg struct {
		names []es.TransactionNameAgg
		err   error
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
	ti.Placeholder = "Search... (supports ES query syntax)"
	ti.CharLimit = 256
	ti.Width = 50

	ii := textinput.New()
	ii.Placeholder = "Index pattern (e.g., logs, traces, metrics)"
	ii.CharLimit = 128
	ii.Width = 50
	ii.SetValue(client.GetIndex())

	vp := viewport.New(80, 20)

	signal := signalLogs

	return Model{
		client:        client,
		logs:          []es.LogEntry{},
		mode:          viewLogs,
		autoRefresh:   true,
		signalType:    signal,
		lookback:      lookback24h,
		displayFields: DefaultFields(signal),
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
		func() tea.Msg { return tea.EnableMouseCellMotion() },
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

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

	case autoDetectMsg:
		if msg.err != nil {
			// Auto-detect failed, just use current lookback and fetch
			m.loading = true
			// Signal-specific fetch on error
			switch m.signalType {
			case signalMetrics:
				if m.metricsViewMode == metricsViewAggregated {
					m.metricsLoading = true
					return m, m.fetchAggregatedMetrics()
				}
			case signalTraces:
				if m.traceViewLevel == traceViewNames {
					m.tracesLoading = true
					return m, m.fetchTransactionNames()
				}
			}
			return m, m.fetchLogs()
		}
		// Set the detected lookback and fetch
		m.lookback = msg.lookback
		m.statusMessage = fmt.Sprintf("Found %d entries in %s", msg.total, msg.lookback.String())
		m.statusTime = time.Now()
		// Signal-specific fetch
		switch m.signalType {
		case signalMetrics:
			if m.metricsViewMode == metricsViewAggregated {
				m.metricsLoading = true
				return m, m.fetchAggregatedMetrics()
			}
		case signalTraces:
			if m.traceViewLevel == traceViewNames {
				m.tracesLoading = true
				return m, m.fetchTransactionNames()
			}
		}
		m.loading = true
		return m, m.fetchLogs()

	case metricsAggMsg:
		m.metricsLoading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.aggregatedMetrics = msg.result
			m.err = nil
		}
		return m, nil

	case transactionNamesMsg:
		m.tracesLoading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.transactionNames = msg.names
			m.err = nil
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
	if m.serviceFilter != "" {
		pos += len("Service: ") + len(m.serviceFilter) + 5
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
		// Switch signal type
		m.signalType = signalLogs
		m.client.SetIndex(m.signalType.IndexPattern())
		m.displayFields = DefaultFields(m.signalType)
		m.logs = []es.LogEntry{}
		m.selectedIndex = 0
		m.mode = viewLogs
		m.loading = true
		m.statusMessage = "Auto-detecting time range..."
		m.statusTime = time.Now()
		return m, m.autoDetectLookback()
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
		// Switch to next signal type (logs)
		m.signalType = signalLogs
		m.client.SetIndex(m.signalType.IndexPattern())
		m.displayFields = DefaultFields(m.signalType)
		m.logs = []es.LogEntry{}
		m.selectedIndex = 0
		m.mode = viewLogs
		m.loading = true
		m.statusMessage = "Auto-detecting time range..."
		m.statusTime = time.Now()
		return m, m.autoDetectLookback()
	case "/":
		m.mode = viewSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

// Layout constants
const (
	statusBarHeight    = 1
	helpBarHeight      = 1
	compactDetailHeight = 5 // 3 lines of content + 2 for border
	layoutPadding      = 2  // Top/bottom padding from AppStyle
)

// View renders the TUI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Calculate heights
	// Total available: m.height
	// Fixed elements: status bar (1) + help bar (1) + compact detail (5) + padding (2) + newlines (3)
	fixedHeight := statusBarHeight + helpBarHeight + compactDetailHeight + layoutPadding + 3
	logListHeight := m.height - fixedHeight
	if logListHeight < 3 {
		logListHeight = 3
	}

	var b strings.Builder

	// Status bar (top)
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
	}

	// Help bar (bottom)
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return AppStyle.Render(b.String())
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

func (m Model) fetchLogs() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var result *es.SearchResult
		var err error
		var queryJSON string
		index := m.client.GetIndex()

		lookbackRange := m.lookback.ESRange()

		// For traces, determine processor event filter based on view level
		processorEvent := ""
		transactionName := ""
		traceID := ""
		if m.signalType == signalTraces {
			switch m.traceViewLevel {
			case traceViewTransactions:
				processorEvent = "transaction"
				transactionName = m.selectedTxName
			case traceViewSpans:
				// When viewing spans, show all events for the trace (no processor filter)
				traceID = m.selectedTraceID
			default:
				processorEvent = "transaction"
			}
		}

		if m.searchQuery != "" {
			opts := es.SearchOptions{
				Size:            100,
				Service:         m.serviceFilter,
				Level:           m.levelFilter,
				SortAsc:         m.sortAscending,
				SearchFields:    CollectSearchFields(m.displayFields),
				Lookback:        lookbackRange,
				ProcessorEvent:  processorEvent,
				TransactionName: transactionName,
				TraceID:         traceID,
			}
			result, err = m.client.Search(ctx, m.searchQuery, opts)
			queryJSON, _ = m.client.GetSearchQueryJSON(m.searchQuery, opts)
		} else {
			opts := es.TailOptions{
				Size:            100,
				Service:         m.serviceFilter,
				Level:           m.levelFilter,
				SortAsc:         m.sortAscending,
				Lookback:        lookbackRange,
				ProcessorEvent:  processorEvent,
				TransactionName: transactionName,
				TraceID:         traceID,
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

func (m Model) fetchAggregatedMetrics() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		lookbackRange := m.lookback.ESRange()
		bucketInterval := es.LookbackToBucketInterval(lookbackRange)

		opts := es.AggregateMetricsOptions{
			Lookback:   lookbackRange,
			BucketSize: bucketInterval,
		}

		result, err := m.client.AggregateMetrics(ctx, opts)
		if err != nil {
			return metricsAggMsg{err: err}
		}

		return metricsAggMsg{result: result}
	}
}

func (m Model) fetchTransactionNames() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		lookbackRange := m.lookback.ESRange()

		names, err := m.client.GetTransactionNames(ctx, lookbackRange)
		if err != nil {
			return transactionNamesMsg{err: err}
		}

		return transactionNamesMsg{names: names}
	}
}

func (m Model) autoDetectLookback() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// For traces, filter to only count transactions
		processorEvent := ""
		if m.signalType == signalTraces {
			processorEvent = "transaction"
		}

		// Try progressively larger time windows until we find enough data
		// Stop at first one with >= 10,000 entries (or use the one with most data)
		targetCount := int64(10000)
		bestLookback := lookback5m
		bestTotal := int64(0)

		for _, lb := range lookbackDurations {
			opts := es.TailOptions{
				Size:           1, // We only need count, not actual results
				Lookback:       lb.ESRange(),
				ProcessorEvent: processorEvent,
			}

			result, err := m.client.Tail(ctx, opts)
			if err != nil {
				continue
			}

			// Track the best option we've found
			if result.Total > bestTotal {
				bestLookback = lb
				bestTotal = result.Total
			}

			// If we found enough data, stop here and use this lookback
			if result.Total >= targetCount {
				return autoDetectMsg{
					lookback: lb,
					total:    result.Total,
				}
			}
		}

		// Return the best we found (even if < target)
		return autoDetectMsg{
			lookback: bestLookback,
			total:    bestTotal,
		}
	}
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
