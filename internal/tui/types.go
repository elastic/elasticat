// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/es/metrics"
	"github.com/elastic/elasticat/internal/es/traces"
	"github.com/elastic/elasticat/internal/index"
)

// viewMode represents different UI views in the TUI
type viewMode int

// ViewContext captures state needed to restore a view when navigating back
type ViewContext struct {
	Mode viewMode
}

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
	viewPerspectiveList  // List of services or resources for perspective filtering
	viewErrorModal       // Error dialog with copy/close options
	viewQuitConfirm      // Quit confirmation modal
	viewHelp             // Hotkeys overlay
	viewChat             // AI chat with Agent Builder
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

// TimeDisplayMode controls how timestamps are shown in lists
type TimeDisplayMode int

const (
	timeDisplayClock    TimeDisplayMode = iota // HH:MM:SS
	timeDisplayRelative                        // "2m ago"
	timeDisplayFull                            // 2006-01-02 15:04:05
)

// SignalType represents the OTel signal type
type SignalType int

const (
	SignalLogs SignalType = iota
	SignalTraces
	SignalMetrics
	SignalChat // AI Chat with Agent Builder
)

// Unexported aliases for backward compatibility within this package
const (
	signalLogs    = SignalLogs
	signalTraces  = SignalTraces
	signalMetrics = SignalMetrics
	signalChat    = SignalChat
)

// PerspectiveType represents the type of perspective filter
type PerspectiveType int

const (
	PerspectiveServices PerspectiveType = iota
	PerspectiveResources
)

func (p PerspectiveType) String() string {
	switch p {
	case PerspectiveServices:
		return "Services"
	case PerspectiveResources:
		return "Resources"
	default:
		return "Unknown"
	}
}

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
		return "now-24h" // Default to 24h if uninitialized
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
	case signalChat:
		return "Chat"
	default:
		return "Unknown"
	}
}

func (s SignalType) IndexPattern() string {
	switch s {
	case signalLogs:
		return index.Logs
	case signalTraces:
		return index.Traces
	case signalMetrics:
		return index.Metrics
	case signalChat:
		return "" // Chat doesn't use a specific index
	default:
		return index.Logs
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
			// Use only severity_text and log.level - "level" doesn't exist in OTel indices
			{Name: "severity_text", Label: "LEVEL", Width: 7, Selected: true, SearchFields: []string{"severity_text", "log.level"}},
			{Name: "_resource", Label: "RESOURCE", Width: 12, Selected: true, SearchFields: []string{"resource.attributes.service.namespace", "resource.attributes.deployment.environment"}},
			{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"resource.attributes.service.name", "service.name"}},
			// Use only body.text, message, event_name - "body" doesn't exist as a searchable field in OTel
			{Name: "body.text", Label: "MESSAGE", Width: 0, Selected: true, SearchFields: []string{"body.text", "message", "event_name"}},
		}
	}
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

// PerspectiveItem represents a service or resource with aggregate stats
type PerspectiveItem struct {
	Name        string
	LogCount    int64
	TraceCount  int64
	MetricCount int64
}

// ChatMessage represents a message in the chat conversation
type ChatMessage struct {
	Role      string    // "user" or "assistant"
	Content   string    // Message content
	Timestamp time.Time // When the message was sent/received
	Error     bool      // Whether this message represents an error
}

// Message types for Bubble Tea
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
		result *metrics.MetricsAggResult
		err    error
	}
	metricDetailDocsMsg struct {
		docs []es.LogEntry
		err  error
	}
	transactionNamesMsg struct {
		names []traces.TransactionNameAgg
		err   error
	}
	spansMsg struct {
		spans []es.LogEntry
		err   error
	}
	perspectiveDataMsg struct {
		items []PerspectiveItem
		err   error
	}
	chatResponseMsg struct {
		conversationID string
		message        ChatMessage
		err            error
	}
	tickMsg time.Time
	errMsg  error
)
