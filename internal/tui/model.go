// Copyright 2026 Elasticsearch B.V. and contributors
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
	esAPIKey    string           // ES/Kibana API key for Agent Builder auth
	esUsername  string           // ES/Kibana username for Agent Builder auth
	esPassword  string           // ES/Kibana password for Agent Builder auth

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

	// === Chat State ===
	chatMessages       []ChatMessage   // Conversation history
	chatLoading        bool            // Waiting for AI response
	chatConversationID string          // Agent Builder conversation ID
	chatInput          textinput.Model // Chat message input
	chatViewport       viewport.Model  // Chat message history viewport

	// === Credentials Modal State ===
	hideCredsModal bool   // Don't show creds modal after Kibana open (session preference)
	lastKibanaURL  string // The Kibana URL that was just opened (for display in modal)

	// === OTel Config Modal State ===
	otelConfigPath       string    // Path to the OTel config file being watched
	otelLastReload       time.Time // Time of last successful reload
	otelReloadCount      int       // Number of successful reloads this session
	otelWatchingConfig   bool      // Whether we're actively watching for changes
	otelReloadError      error     // Last reload error (nil if successful)
	otelValidationStatus string    // Last validation status message
	otelValidationValid  bool      // Whether last validation passed

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

// Chat returns the current chat state as a struct.
func (m Model) Chat() ChatState {
	return ChatState{
		Messages:       m.chatMessages,
		Loading:        m.chatLoading,
		ConversationID: m.chatConversationID,
		Input:          m.chatInput,
		Viewport:       m.chatViewport,
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
// NewModelOpts holds optional configuration for NewModel.
type NewModelOpts struct {
	ESAPIKey   string
	ESUsername string
	ESPassword string
}

func NewModel(ctx context.Context, client DataSource, signal SignalType, tuiCfg config.TUIConfig, kibanaURL, kibanaSpace string) Model {
	return NewModelWithOpts(ctx, client, signal, tuiCfg, kibanaURL, kibanaSpace, NewModelOpts{})
}

func NewModelWithOpts(ctx context.Context, client DataSource, signal SignalType, tuiCfg config.TUIConfig, kibanaURL, kibanaSpace string, opts NewModelOpts) Model {
	ti := textinput.New()
	ti.Placeholder = "Search... (supports ES query syntax)"
	ti.CharLimit = 256
	ti.Width = 50

	ii := textinput.New()
	ii.Placeholder = "Index pattern (e.g., logs, traces, metrics)"
	ii.CharLimit = 128
	ii.Width = 50

	ci := textinput.New()
	ci.Placeholder = "Ask a question about your o11y data..."
	ci.CharLimit = 1024
	ci.Width = 70

	// Set the client's index pattern based on the signal type
	if signal != SignalChat {
		client.SetIndex(signal.IndexPattern())
	}
	ii.SetValue(client.GetIndex())

	vp := viewport.New(80, 20)
	errorVp := viewport.New(70, 15) // Viewport for error modal
	helpVp := viewport.New(70, 15)  // Viewport for help overlay
	chatVp := viewport.New(80, 15)  // Viewport for chat history

	// Determine initial view mode based on signal type
	var initialMode viewMode
	switch signal {
	case SignalTraces:
		initialMode = viewTraceNames
	case SignalMetrics:
		initialMode = viewMetricsDashboard
	case SignalChat:
		initialMode = viewChat
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
		esAPIKey:        opts.ESAPIKey,
		esUsername:      opts.ESUsername,
		esPassword:      opts.ESPassword,
		logs:            []es.LogEntry{},
		mode:            initialMode,
		autoRefresh:     true,
		signalType:      signal,
		lookback:        lookback24h,
		timeDisplayMode: timeDisplayRelative,
		displayFields:   DefaultFields(signal),
		searchInput:     ti,
		indexInput:      ii,
		chatInput:       ci,
		viewport:        vp,
		errorViewport:   errorVp,
		helpViewport:    helpVp,
		chatViewport:    chatVp,
		chatMessages:    []ChatMessage{},
		width:           80,
		height:          24,
		traceViewLevel:  traceViewNames,        // Start at transaction names for traces
		metricsViewMode: metricsViewAggregated, // Start at aggregated view for metrics
		requests:        newRequestManager(),
	}
}
