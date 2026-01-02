package es

import (
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

// Client wraps the Elasticsearch client with telasticat-specific functionality
type Client struct {
	es    *elasticsearch.Client
	index string
}

// LogEntry represents a single log/trace/metric entry from Elasticsearch
type LogEntry struct {
	Timestamp   time.Time              `json:"@timestamp"`
	Body        string                 `json:"body,omitempty"`
	Message     string                 `json:"message,omitempty"`
	EventName   string                 `json:"event_name,omitempty"`
	Level       string                 `json:"level,omitempty"`
	ServiceName string                 `json:"service.name,omitempty"`
	ContainerID string                 `json:"container_id,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Resource    map[string]interface{} `json:"resource,omitempty"`
	RawJSON     string                 `json:"-"` // Original JSON from ES (not serialized)

	// Trace-specific fields
	TraceID  string                 `json:"trace_id,omitempty"`
	SpanID   string                 `json:"span_id,omitempty"`
	Name     string                 `json:"name,omitempty"` // Span name
	Duration int64                  `json:"duration,omitempty"` // Duration in nanoseconds
	Kind     string                 `json:"kind,omitempty"`
	Status   map[string]interface{} `json:"status,omitempty"`

	// Metrics-specific fields
	Metrics map[string]interface{} `json:"metrics,omitempty"`
	Scope   map[string]interface{} `json:"scope,omitempty"`
}

// SearchResult contains the results of a log search
type SearchResult struct {
	Logs     []LogEntry
	Total    int64
	ScrollID string
}

// TailOptions configures the tail query
type TailOptions struct {
	Size            int
	Service         string
	Resource        string // Filter on resource.attributes.deployment.environment
	Level           string
	Since           time.Time
	ContainerID     string
	SortAsc         bool   // true = oldest first, false = newest first (default)
	Lookback        string // ES time range string like "now-1h", "now-24h", or "" for no filter
	ProcessorEvent  string // Filter on attributes.processor.event (e.g., "transaction" for traces)
	TransactionName string // Filter on transaction name (for traces)
	TraceID         string // Filter on trace_id (for viewing spans)
}

// SearchOptions configures the search query
type SearchOptions struct {
	Size            int
	Service         string
	Resource        string   // Filter on resource.attributes.deployment.environment
	Level           string
	From            time.Time
	To              time.Time
	SortAsc         bool     // true = oldest first, false = newest first (default)
	SearchFields    []string // ES fields to search (if empty, uses default body/message fields)
	Lookback        string   // ES time range string like "now-1h", "now-24h", or "" for no filter
	ProcessorEvent  string   // Filter on attributes.processor.event (e.g., "transaction" for traces)
	TransactionName string   // Filter on transaction name (for traces)
	TraceID         string   // Filter on trace_id (for viewing spans)
}

// FieldInfo represents metadata about an Elasticsearch field
type FieldInfo struct {
	Name         string // Field name (e.g., "body.text", "severity_text")
	Type         string // Field type (e.g., "keyword", "text", "long")
	Searchable   bool   // Whether the field is searchable
	Aggregatable bool   // Whether the field can be aggregated
	DocCount     int64  // Number of documents with this field (0 = unknown)
}

// Domain-specific types have been moved to subdirectories:
// - Trace types: traces/types.go (TransactionNameAgg, ESQLResult, ESQLColumn)
// - Metric types: metrics/types.go (MetricFieldInfo, MetricBucket, AggregatedMetric, etc.)
// - Perspective types: perspectives/types.go (PerspectiveAgg)
