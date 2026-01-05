// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/es/metrics"
	"github.com/elastic/elasticat/internal/es/traces"
)

// Model is the main TUI model containing all application state
type Model struct {
	// ES client
	client *es.Client

	// State
	logs            []es.LogEntry
	selectedIndex   int
	userHasScrolled bool // Track if user manually scrolled (for tail -f behavior)
	mode            viewMode
	previousMode    viewMode // Store previous mode when showing error modal
	err             error
	loading         bool
	total           int64

	// Filters
	levelFilter    string
	searchQuery    string
	filterService  string // Active service filter (from perspectives or manual)
	negateService  bool   // If true, exclude filterService instead of filtering to it
	filterResource string // Active resource filter (from perspectives or manual)
	negateResource bool   // If true, exclude filterResource instead of filtering to it

	// Auto-refresh
	autoRefresh bool

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
	displayFields    []DisplayField // Currently configured display fields
	availableFields  []es.FieldInfo // All fields from field_caps
	fieldsCursor     int            // Cursor position in field selector
	fieldsLoading    bool           // Loading field caps
	fieldsSearchMode bool           // Whether we're in search mode within fields view
	fieldsSearch     string         // Search filter for fields

	// Components
	searchInput   textinput.Model
	indexInput    textinput.Model
	viewport      viewport.Model
	errorViewport viewport.Model // For scrollable error modal

	// Dimensions
	width  int
	height int

	// Last refresh time
	lastRefresh time.Time

	// Metrics dashboard state
	metricsViewMode   MetricsViewMode
	aggregatedMetrics *metrics.MetricsAggResult
	metricsLoading    bool
	metricsCursor     int // Selected metric in dashboard

	// Traces navigation state
	traceViewLevel   TraceViewLevel              // Current navigation level
	transactionNames []traces.TransactionNameAgg // Aggregated transaction names
	traceNamesCursor int                         // Cursor in transaction names list
	selectedTxName   string                      // Selected transaction name filter
	selectedTraceID  string                      // Selected trace_id for spans view
	tracesLoading    bool                        // Loading transaction names
	spans            []es.LogEntry               // Child spans for selected trace
	spansLoading     bool                        // Loading spans for trace

	// Perspective filtering state
	currentPerspective PerspectiveType   // Current perspective being viewed
	perspectiveItems   []PerspectiveItem // List of services or resources with counts
	perspectiveCursor  int               // Cursor in perspective list
	perspectiveLoading bool              // Loading perspective data
}

// Highlighter returns a Highlighter configured with the current search query
func (m Model) Highlighter() *Highlighter {
	return NewHighlighter(m.searchQuery)
}

// NewModel creates a new TUI model
func NewModel(client *es.Client, signal SignalType) Model {
	ti := textinput.New()
	ti.Placeholder = "Search... (supports ES query syntax)"
	ti.CharLimit = 256
	ti.Width = 50

	ii := textinput.New()
	ii.Placeholder = "Index pattern (e.g., logs, traces, metrics)"
	ii.CharLimit = 128
	ii.Width = 50

	// Set the client's index pattern based on the signal type
	client.SetIndex(signal.IndexPattern())
	ii.SetValue(client.GetIndex())

	vp := viewport.New(80, 20)
	errorVp := viewport.New(70, 15) // Viewport for error modal

	// Determine initial view mode based on signal type
	var initialMode viewMode
	switch signal {
	case SignalTraces:
		initialMode = viewTraceNames
	case SignalMetrics:
		initialMode = viewMetricsDashboard
	default:
		initialMode = viewLogs
	}

	return Model{
		client:          client,
		logs:            []es.LogEntry{},
		mode:            initialMode,
		autoRefresh:     true,
		signalType:      signal,
		lookback:        lookback24h,
		displayFields:   DefaultFields(signal),
		searchInput:     ti,
		indexInput:      ii,
		viewport:        vp,
		errorViewport:   errorVp,
		width:           80,
		height:          24,
		traceViewLevel:  traceViewNames,        // Start at transaction names for traces
		metricsViewMode: metricsViewAggregated, // Start at aggregated view for metrics
	}
}
