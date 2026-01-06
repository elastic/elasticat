// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/elastic/elasticat/internal/config"
	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/es/metrics"
	"github.com/elastic/elasticat/internal/es/traces"
)

// Model is the main TUI model containing all application state.
//
// The fields are organized into logical groups (see state.go for corresponding
// types that document this structure):
//   - Core: client, ctx, requests
//   - UI: mode, dimensions, loading, error state
//   - Filters: service, resource, level, search query
//   - Logs: log entries and selection
//   - Metrics: dashboard and detail state
//   - Traces: navigation hierarchy state
//   - Perspective: filtering by service/resource
//   - Fields: field selection state
//   - Components: text inputs and viewports
type Model struct {
	// === Core ===
	client      DataSource       // Data source (interface for testability)
	ctx         context.Context  // Parent context (canceled when app exits)
	requests    *requestManager  // In-flight request management
	tuiConfig   config.TUIConfig // TUI timing/config
	kibanaURL   string           // Kibana base URL for "Open in Kibana" feature
	kibanaSpace string           // Kibana space (e.g., "elasticat") for URL path prefix

	// === UI State ===
	mode            viewMode
	viewStack       []ViewContext   // Navigation history for back navigation
	err             error           // Current error
	loading         bool            // General loading indicator
	width           int             // Terminal width
	height          int             // Terminal height
	autoRefresh     bool            // Auto-refresh enabled
	timeDisplayMode TimeDisplayMode // How timestamps display
	sortAscending   bool            // Sort order
	statusMessage   string          // Temporary status
	statusTime      time.Time       // When status was set
	lastRefresh     time.Time       // Last data refresh

	// === Filters ===
	levelFilter    string
	searchQuery    string
	filterService  string // Service filter
	negateService  bool   // Exclude service
	filterResource string // Resource filter
	negateResource bool   // Exclude resource
	signalType     SignalType
	lookback       LookbackDuration

	// === Query Display ===
	lastQueryJSON  string      // Last ES query body
	lastQueryIndex string      // Index pattern used
	queryFormat    queryFormat // Kibana or curl

	// === Logs State ===
	logs            []es.LogEntry
	selectedIndex   int
	userHasScrolled bool
	total           int64

	// === Fields State ===
	displayFields    []DisplayField
	availableFields  []es.FieldInfo
	fieldsCursor     int
	fieldsLoading    bool
	fieldsSearchMode bool
	fieldsSearch     string

	// === Metrics State ===
	metricsViewMode         MetricsViewMode
	aggregatedMetrics       *metrics.MetricsAggResult
	metricsLoading          bool
	metricsCursor           int
	metricDetailDocs        []es.LogEntry
	metricDetailDocCursor   int
	metricDetailDocsLoading bool

	// === Traces State ===
	traceViewLevel     TraceViewLevel
	transactionNames   []traces.TransactionNameAgg
	traceNamesCursor   int
	selectedTxName     string
	selectedTraceID    string
	tracesLoading      bool
	spans              []es.LogEntry
	spansLoading       bool
	lastFetchedTraceID string

	// === Perspective State ===
	currentPerspective PerspectiveType
	perspectiveItems   []PerspectiveItem
	perspectiveCursor  int
	perspectiveLoading bool

	// === UI Components ===
	searchInput   textinput.Model
	indexInput    textinput.Model
	viewport      viewport.Model
	errorViewport viewport.Model
	helpViewport  viewport.Model
}

// Highlighter returns a Highlighter configured with the current search query
func (m Model) Highlighter() *Highlighter {
	return NewHighlighter(m.searchQuery)
}

// === State accessor methods ===
// These methods provide a cleaner interface for accessing grouped state,
// and will enable future refactoring to embedded structs.

// Filters returns the current filter state as a struct.
func (m Model) Filters() FilterState {
	return FilterState{
		LevelFilter:    m.levelFilter,
		SearchQuery:    m.searchQuery,
		FilterService:  m.filterService,
		NegateService:  m.negateService,
		FilterResource: m.filterResource,
		NegateResource: m.negateResource,
	}
}

// Logs returns the current logs state as a struct.
func (m Model) Logs() LogsState {
	return LogsState{
		Logs:            m.logs,
		SelectedIndex:   m.selectedIndex,
		UserHasScrolled: m.userHasScrolled,
		Total:           m.total,
	}
}

// Metrics returns the current metrics state as a struct.
func (m Model) Metrics() MetricsState {
	return MetricsState{
		ViewMode:          m.metricsViewMode,
		AggregatedMetrics: m.aggregatedMetrics,
		Loading:           m.metricsLoading,
		Cursor:            m.metricsCursor,
		DetailDocs:        m.metricDetailDocs,
		DetailDocCursor:   m.metricDetailDocCursor,
		DetailDocsLoading: m.metricDetailDocsLoading,
	}
}

// Traces returns the current traces state as a struct.
func (m Model) Traces() TracesState {
	return TracesState{
		ViewLevel:          m.traceViewLevel,
		TransactionNames:   m.transactionNames,
		NamesCursor:        m.traceNamesCursor,
		SelectedTxName:     m.selectedTxName,
		SelectedTraceID:    m.selectedTraceID,
		Loading:            m.tracesLoading,
		Spans:              m.spans,
		SpansLoading:       m.spansLoading,
		LastFetchedTraceID: m.lastFetchedTraceID,
	}
}

// Perspective returns the current perspective state as a struct.
func (m Model) Perspective() PerspectiveState {
	return PerspectiveState{
		Current: m.currentPerspective,
		Items:   m.perspectiveItems,
		Cursor:  m.perspectiveCursor,
		Loading: m.perspectiveLoading,
	}
}

// setViewportContent wraps content to the viewport width before rendering.
func (m *Model) setViewportContent(content string) {
	wrapped := WrapText(content, m.viewport.Width)
	m.viewport.SetContent(wrapped)
}

// NewModel creates a new TUI model.
// The client parameter accepts any DataSource implementation, enabling
// mock data sources for testing.
func NewModel(ctx context.Context, client DataSource, signal SignalType, tuiCfg config.TUIConfig, kibanaURL, kibanaSpace string) Model {
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
	helpVp := viewport.New(70, 15)  // Viewport for help overlay

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

	if ctx == nil {
		ctx = context.Background()
	}

	// Use default Kibana URL if not configured
	if kibanaURL == "" {
		kibanaURL = config.DefaultKibanaURL
	}

	return Model{
		ctx:             ctx,
		client:          client,
		tuiConfig:       tuiCfg,
		kibanaURL:       kibanaURL,
		kibanaSpace:     kibanaSpace,
		logs:            []es.LogEntry{},
		mode:            initialMode,
		autoRefresh:     true,
		signalType:      signal,
		lookback:        lookback24h,
		timeDisplayMode: timeDisplayClock,
		displayFields:   DefaultFields(signal),
		searchInput:     ti,
		indexInput:      ii,
		viewport:        vp,
		errorViewport:   errorVp,
		helpViewport:    helpVp,
		width:           80,
		height:          24,
		traceViewLevel:  traceViewNames,        // Start at transaction names for traces
		metricsViewMode: metricsViewAggregated, // Start at aggregated view for metrics
		requests:        newRequestManager(),
	}
}
