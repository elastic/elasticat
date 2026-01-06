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

// FilterState holds all active filters.
type FilterState struct {
	LevelFilter    string // Level/severity filter
	SearchQuery    string // Free-text search query
	FilterService  string // Active service filter
	NegateService  bool   // If true, exclude FilterService
	FilterResource string // Active resource filter
	NegateResource bool   // If true, exclude FilterResource
}

// UIState holds general UI state shared across views.
type UIState struct {
	Mode            viewMode        // Current view mode
	PreviousMode    viewMode        // Previous mode (for modal returns)
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
	LastQueryJSON  string      // Last query body as JSON
	LastQueryIndex string      // Index pattern used
	QueryFormat    queryFormat // Kibana or curl format
}

// LogsState holds log list state.
type LogsState struct {
	Logs            []es.LogEntry // Current log entries
	SelectedIndex   int           // Selected log index
	UserHasScrolled bool          // User manually scrolled
	Total           int64         // Total matching documents
}

// FieldsState holds field selection state.
type FieldsState struct {
	DisplayFields    []DisplayField // Configured display fields
	AvailableFields  []es.FieldInfo // Available fields from ES
	FieldsCursor     int            // Cursor in field selector
	FieldsLoading    bool           // Loading field caps
	FieldsSearchMode bool           // In search mode within fields
	FieldsSearch     string         // Search filter for fields
}

// MetricsState holds metrics dashboard state.
type MetricsState struct {
	ViewMode          MetricsViewMode           // Aggregated vs documents view
	AggregatedMetrics *metrics.MetricsAggResult // Aggregation results
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

// UIComponents holds UI component instances.
type UIComponents struct {
	SearchInput   textinput.Model // Search text input
	IndexInput    textinput.Model // Index pattern input
	Viewport      viewport.Model  // Main content viewport
	ErrorViewport viewport.Model  // Error modal viewport
	HelpViewport  viewport.Model  // Help overlay viewport
}
