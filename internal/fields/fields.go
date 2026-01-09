// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

// Package fields provides field configuration shared between CLI and TUI.
package fields

import "github.com/elastic/elasticat/internal/index"

// SignalType represents the OTel signal type
type SignalType int

const (
	SignalLogs SignalType = iota
	SignalTraces
	SignalMetrics
	SignalChat // AI Chat with Agent Builder
)

func (s SignalType) String() string {
	switch s {
	case SignalLogs:
		return "Logs"
	case SignalTraces:
		return "Traces"
	case SignalMetrics:
		return "Metrics"
	case SignalChat:
		return "Chat"
	default:
		return "Unknown"
	}
}

func (s SignalType) IndexPattern() string {
	switch s {
	case SignalLogs:
		return index.Logs
	case SignalTraces:
		return index.Traces
	case SignalMetrics:
		return index.Metrics
	case SignalChat:
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
	case SignalTraces:
		return []DisplayField{
			{Name: "@timestamp", Label: "TIME", Width: 8, Selected: true, SearchFields: nil},
			// traces-* uses resource.attributes.service.name, not attributes.service.name
			{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"service.name"}},
			{Name: "name", Label: "NAME", Width: 25, Selected: true, SearchFields: []string{"name"}},
			{Name: "duration_ms", Label: "DUR(ms)", Width: 9, Selected: true, SearchFields: nil},
			{Name: "status.code", Label: "STATUS", Width: 6, Selected: true, SearchFields: []string{"status.code"}},
			{Name: "kind", Label: "KIND", Width: 8, Selected: true, SearchFields: []string{"kind"}},
			{Name: "trace_id", Label: "TRACE", Width: 0, Selected: true, SearchFields: []string{"trace_id"}},
		}
	case SignalMetrics:
		return []DisplayField{
			{Name: "@timestamp", Label: "TIME", Width: 8, Selected: true, SearchFields: nil},
			// metrics-* uses service.name directly
			{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"service.name"}},
			{Name: "scope.name", Label: "SCOPE", Width: 20, Selected: true, SearchFields: []string{"scope.name"}},
			{Name: "attributes.span.name", Label: "SPAN", Width: 25, Selected: true, SearchFields: []string{"attributes.span.name"}},
			{Name: "_metrics", Label: "METRICS", Width: 0, Selected: true, SearchFields: nil},
		}
	default: // SignalLogs
		return []DisplayField{
			{Name: "@timestamp", Label: "TIME", Width: 8, Selected: true, SearchFields: nil},
			// Use only severity_text and log.level - "level" doesn't exist in OTel indices
			{Name: "severity_text", Label: "LEVEL", Width: 7, Selected: true, SearchFields: []string{"severity_text", "log.level"}},
			// Use deployment.environment only - service.namespace is not a standard OTel field
			{Name: "_resource", Label: "RESOURCE", Width: 12, Selected: true, SearchFields: []string{"resource.attributes.deployment.environment"}},
			// logs-* uses attributes.service.name, not resource.attributes.service.name
			{Name: "service.name", Label: "SERVICE", Width: 15, Selected: true, SearchFields: []string{"service.name"}},
			// Use only body.text, message, event_name - "body" doesn't exist as a searchable field in OTel
			{Name: "body.text", Label: "MESSAGE", Width: 0, Selected: true, SearchFields: []string{"body.text", "message", "event_name"}},
		}
	}
}
