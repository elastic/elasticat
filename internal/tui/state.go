// Copyright 2026 Elasticsearch B.V. and contributors
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

// FilterState holds all active filters.
type FilterState struct {
	Level          string // Level/severity filter
	Query          string // Free-text search query
	Service        string // Active service filter
	NegateService  bool   // If true, exclude Service
	Resource       string // Active resource filter
	NegateResource bool   // If true, exclude Resource
	Signal         SignalType
	Lookback       LookbackDuration
}

// UIState holds general UI state shared across views.
type UIState struct {
	Mode            viewMode        // Current view mode
	ViewStack       []ViewContext   // Navigation history for back navigation
	Err             error           // Current error (if any)
	Loading         bool            // General loading indicator
	Width           int             // Terminal width
	Height          int             // Terminal height
	AutoRefresh     bool            // Auto-refresh enabled
	TimeDisplayMode TimeDisplayMode // How timestamps are displayed
	SortAscending   bool            // Sort order (true = oldest first)
	StatusMessage   string          // Temporary status message
	StatusTime      time.Time       // When status was set
	LastRefresh     time.Time       // Last data refresh time
}

// QueryState holds query display state.
type QueryState struct {
	LastJSON  string      // Last query body as JSON
	LastIndex string      // Index pattern used
	Format    queryFormat // Kibana or curl format
}

// LogsState holds log list state.
type LogsState struct {
	Entries         []es.LogEntry // Current log entries
	SelectedIndex   int           // Selected log index
	UserHasScrolled bool          // User manually scrolled
	Total           int64         // Total matching documents
}

// FieldsState holds field selection state.
type FieldsState struct {
	Display    []DisplayField // Configured display fields
	Available  []es.FieldInfo // Available fields from ES
	Cursor     int            // Cursor in field selector
	Loading    bool           // Loading field caps
	SearchMode bool           // In search mode within fields
	Search     string         // Search filter for fields
}

// MetricsState holds metrics dashboard state.
type MetricsState struct {
	ViewMode          MetricsViewMode           // Aggregated vs documents view
	Aggregated        *metrics.MetricsAggResult // Aggregation results
	Loading           bool                      // Loading metrics
	Cursor            int                       // Selected metric
	DetailDocs        []es.LogEntry             // Detail view documents
	DetailDocCursor   int                       // Current detail doc index
	DetailDocsLoading bool                      // Loading detail docs
}

// TracesState holds traces navigation state.
type TracesState struct {
	ViewLevel          TraceViewLevel              // Current navigation level
	TransactionNames   []traces.TransactionNameAgg // Aggregated transaction names
	NamesCursor        int                         // Cursor in names list
	SelectedTxName     string                      // Selected transaction name
	SelectedTraceID    string                      // Selected trace ID for spans
	Loading            bool                        // Loading transaction names
	Spans              []es.LogEntry               // Child spans
	SpansLoading       bool                        // Loading spans
	LastFetchedTraceID string                      // De-dupe span fetches
}

// PerspectiveState holds perspective filtering state.
type PerspectiveState struct {
	Current PerspectiveType   // Current perspective type
	Items   []PerspectiveItem // List items with counts
	Cursor  int               // Cursor position
	Loading bool              // Loading perspective data
}

// ChatState holds AI chat state.
type ChatState struct {
	Messages        []ChatMessage   // Conversation history
	Loading         bool            // Waiting for AI response
	ConversationID  string          // Agent Builder conversation ID
	Input           textinput.Model // Chat message input
	Viewport        viewport.Model  // Chat message history viewport
	InsertMode      bool            // Vim-style insert mode for chat input
	AnalysisContext string          // What's being analyzed
	RequestStart    time.Time       // When the current chat request started
}

// CredsState holds credentials modal state.
type CredsState struct {
	HideModal     bool   // Don't show creds modal after Kibana open
	LastKibanaURL string // The Kibana URL that was just opened
}

// OtelState holds OTel config modal state.
type OtelState struct {
	ConfigPath       string    // Path to the OTel config file
	LastReload       time.Time // Time of last successful reload
	ReloadCount      int       // Number of successful reloads
	WatchingConfig   bool      // Whether watching for changes
	ReloadError      error     // Last reload error
	ValidationStatus string    // Last validation status message
	ValidationValid  bool      // Whether last validation passed
}

// UIComponents holds UI component instances.
type UIComponents struct {
	SearchInput   textinput.Model // Search text input
	IndexInput    textinput.Model // Index pattern input
	Viewport      viewport.Model  // Main content viewport
	ErrorViewport viewport.Model  // Error modal viewport
	HelpViewport  viewport.Model  // Help overlay viewport
}
