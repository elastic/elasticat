package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
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
	TraceID    string                 `json:"trace_id,omitempty"`
	SpanID     string                 `json:"span_id,omitempty"`
	Name       string                 `json:"name,omitempty"` // Span name
	Duration   int64                  `json:"duration,omitempty"` // Duration in nanoseconds
	Kind       string                 `json:"kind,omitempty"`
	Status     map[string]interface{} `json:"status,omitempty"`

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

// MetricFieldInfo represents a discovered metric field from field_caps
type MetricFieldInfo struct {
	Name           string // Full field path (e.g., "metrics.raradio.session.active")
	ShortName      string // Display name (e.g., "raradio.session.active")
	Type           string // ES type: "long", "double", "histogram"
	TimeSeriesType string // "gauge", "counter", or ""
}

// MetricBucket represents a single time bucket for a metric
type MetricBucket struct {
	Timestamp time.Time
	Value     float64 // Aggregated value for this bucket
	Count     int64   // Number of data points in bucket
}

// AggregatedMetric represents aggregated statistics for a single metric
type AggregatedMetric struct {
	Name      string         // Metric field name
	ShortName string         // Display name
	Type      string         // "gauge", "counter", "histogram"
	Min       float64
	Max       float64
	Avg       float64
	Latest    float64
	Buckets   []MetricBucket // Time series data for sparkline
}

// MetricsAggResult contains all aggregated metrics
type MetricsAggResult struct {
	Metrics    []AggregatedMetric
	BucketSize string // ES interval (e.g., "10s", "1m")
}

// AggregateMetricsOptions configures the metrics aggregation query
type AggregateMetricsOptions struct {
	Lookback   string // ES time range (e.g., "now-5m", "now-1h")
	BucketSize string // ES interval (e.g., "10s", "1m", "5m")
}

// TransactionNameAgg represents aggregated statistics for a transaction name
type TransactionNameAgg struct {
	Name        string  // Transaction name (e.g., "GET /api/users")
	Count       int64   // Number of transactions
	AvgDuration float64 // Average duration in milliseconds
	ErrorRate   float64 // Percentage of errors (0-100)
}

// New creates a new Elasticsearch client
func New(addresses []string, index string) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: addresses,
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ES client: %w", err)
	}

	return &Client{
		es:    es,
		index: index,
	}, nil
}

// NewDefault creates a client with default localhost configuration
func NewDefault() (*Client, error) {
	return New([]string{"http://localhost:9200"}, "logs")
}

// SetIndex changes the index pattern
func (c *Client) SetIndex(index string) {
	c.index = index
}

// GetIndex returns the current index pattern
func (c *Client) GetIndex() string {
	return c.index
}

// GetTailQueryJSON returns the JSON query body for a tail operation
func (c *Client) GetTailQueryJSON(opts TailOptions) (string, error) {
	query := buildTailQuery(opts)
	queryJSON, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		return "", err
	}
	return string(queryJSON), nil
}

// GetSearchQueryJSON returns the JSON query body for a search operation
func (c *Client) GetSearchQueryJSON(queryStr string, opts SearchOptions) (string, error) {
	query := buildSearchQuery(queryStr, opts)
	queryJSON, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		return "", err
	}
	return string(queryJSON), nil
}

// Ping checks if Elasticsearch is reachable
func (c *Client) Ping(ctx context.Context) error {
	res, err := c.es.Ping(c.es.Ping.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to ping ES: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES ping failed: %s", res.Status())
	}

	return nil
}

// Tail retrieves the most recent logs, optionally filtered
func (c *Client) Tail(ctx context.Context, opts TailOptions) (*SearchResult, error) {
	query := buildTailQuery(opts)

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	sortOrder := "@timestamp:desc"
	if opts.SortAsc {
		sortOrder = "@timestamp:asc"
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(c.index+"*"),
		c.es.Search.WithBody(bytes.NewReader(queryJSON)),
		c.es.Search.WithSize(opts.Size),
		c.es.Search.WithSort(sortOrder),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search failed: %s - %s", res.Status(), string(body))
	}

	return parseSearchResponse(res.Body)
}

// GetSpansByTraceID fetches all spans for a given trace ID
func (c *Client) GetSpansByTraceID(ctx context.Context, traceID string) (*SearchResult, error) {
	opts := TailOptions{
		Size:           1000, // Get up to 1000 spans per trace (should be enough for most traces)
		TraceID:        traceID,
		ProcessorEvent: "span", // Only get spans, not transactions
		SortAsc:        true,   // Sort by timestamp ascending to show chronological order
	}

	return c.Tail(ctx, opts)
}

// Search performs a full-text search on logs
func (c *Client) Search(ctx context.Context, queryStr string, opts SearchOptions) (*SearchResult, error) {
	query := buildSearchQuery(queryStr, opts)

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	sortOrder := "@timestamp:desc"
	if opts.SortAsc {
		sortOrder = "@timestamp:asc"
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(c.index+"*"),
		c.es.Search.WithBody(bytes.NewReader(queryJSON)),
		c.es.Search.WithSize(opts.Size),
		c.es.Search.WithSort(sortOrder),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search failed: %s - %s", res.Status(), string(body))
	}

	return parseSearchResponse(res.Body)
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

// buildTailQuery constructs an ES query for tailing logs.
//
// DESIGN PRINCIPLE: Support Multiple Log Formats
// The query uses "should" clauses with minimum_should_match for fields that
// may appear in different locations depending on the log format (e.g., OTel
// semconv vs. ECS vs. custom formats). This ensures we find logs regardless
// of their structure.
func buildTailQuery(opts TailOptions) map[string]interface{} {
	must := []map[string]interface{}{}

	// Time range filter - use Lookback if set, otherwise Since, otherwise default 1h
	if opts.Lookback != "" || !opts.Since.IsZero() {
		timeRange := map[string]interface{}{}
		if opts.Lookback != "" {
			timeRange["gte"] = opts.Lookback
		} else if !opts.Since.IsZero() {
			timeRange["gte"] = opts.Since.Format(time.RFC3339)
		}
		must = append(must, map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": timeRange,
			},
		})
	}
	// If both Lookback is empty and Since is zero, no time filter is applied (query all time)

	// Service filter - check both OTel format (resource.attributes.service.name) and flat format
	if opts.Service != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"resource.attributes.service.name": opts.Service}},
					{"term": map[string]interface{}{"resource.service.name": opts.Service}},
				},
				"minimum_should_match": 1,
			},
		})
	}

	// Resource filter - filter on deployment environment
	if opts.Resource != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"resource.attributes.deployment.environment": opts.Resource,
			},
		})
	}

	// Level filter - check both severity_text (OTel) and level
	if opts.Level != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"severity_text": opts.Level}},
					{"term": map[string]interface{}{"level": opts.Level}},
				},
				"minimum_should_match": 1,
			},
		})
	}

	// Container ID filter
	if opts.ContainerID != "" {
		must = append(must, map[string]interface{}{
			"prefix": map[string]interface{}{
				"container_id": opts.ContainerID,
			},
		})
	}

	// Processor event filter (e.g., "transaction" for traces)
	if opts.ProcessorEvent != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"attributes.processor.event": opts.ProcessorEvent,
			},
		})
	}

	// Transaction name filter (for traces)
	if opts.TransactionName != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"transaction.name": opts.TransactionName}},
					{"term": map[string]interface{}{"name": opts.TransactionName}},
				},
				"minimum_should_match": 1,
			},
		})
	}

	// Trace ID filter (for viewing all spans in a trace)
	if opts.TraceID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"trace_id": opts.TraceID,
			},
		})
	}

	if opts.Size == 0 {
		opts.Size = 100
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
	}
}

// buildSearchQuery constructs an ES query for searching logs.
//
// DESIGN PRINCIPLE: Support Multiple Log Formats
// Uses multi_match across configured search fields to find text.
// If no search fields are provided, defaults to common message fields.
// Service/level filters check multiple possible field paths.
func buildSearchQuery(queryStr string, opts SearchOptions) map[string]interface{} {
	must := []map[string]interface{}{}

	// Full-text search across configured fields
	if queryStr != "" {
		// Use provided search fields, or fall back to defaults
		searchFields := opts.SearchFields
		if len(searchFields) == 0 {
			searchFields = []string{"body.text", "body", "message", "event_name"}
		}

		// Use query_string for flexible searching that works with both
		// analyzed (text) and non-analyzed (keyword) fields
		// Wrap the query in wildcards for partial matching on keyword fields
		wildcardQuery := "*" + queryStr + "*"

		must = append(must, map[string]interface{}{
			"query_string": map[string]interface{}{
				"query":            wildcardQuery,
				"fields":           searchFields,
				"default_operator": "AND",
				"analyze_wildcard": true,
			},
		})
	}

	// Time range - prefer Lookback, then From/To
	timeRange := map[string]interface{}{}
	if opts.Lookback != "" {
		timeRange["gte"] = opts.Lookback
	} else if !opts.From.IsZero() {
		timeRange["gte"] = opts.From.Format(time.RFC3339)
	}
	if !opts.To.IsZero() {
		timeRange["lte"] = opts.To.Format(time.RFC3339)
	}
	if len(timeRange) > 0 {
		must = append(must, map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": timeRange,
			},
		})
	}

	// Service filter - check both OTel format and flat format
	if opts.Service != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"resource.attributes.service.name": opts.Service}},
					{"term": map[string]interface{}{"resource.service.name": opts.Service}},
				},
				"minimum_should_match": 1,
			},
		})
	}

	// Resource filter - filter on deployment environment
	if opts.Resource != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"resource.attributes.deployment.environment": opts.Resource,
			},
		})
	}

	// Level filter - check both severity_text (OTel) and level
	if opts.Level != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"severity_text": opts.Level}},
					{"term": map[string]interface{}{"level": opts.Level}},
				},
				"minimum_should_match": 1,
			},
		})
	}

	// Processor event filter (e.g., "transaction" for traces)
	if opts.ProcessorEvent != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"attributes.processor.event": opts.ProcessorEvent,
			},
		})
	}

	// Transaction name filter (for traces)
	if opts.TransactionName != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"transaction.name": opts.TransactionName}},
					{"term": map[string]interface{}{"name": opts.TransactionName}},
				},
				"minimum_should_match": 1,
			},
		})
	}

	// Trace ID filter (for viewing all spans in a trace)
	if opts.TraceID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"trace_id": opts.TraceID,
			},
		})
	}

	if opts.Size == 0 {
		opts.Size = 100
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
	}
}

func parseSearchResponse(body io.Reader) (*SearchResult, error) {
	var response struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
		ScrollID string `json:"_scroll_id"`
	}

	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := &SearchResult{
		Total:    response.Hits.Total.Value,
		ScrollID: response.ScrollID,
		Logs:     make([]LogEntry, 0, len(response.Hits.Hits)),
	}

	for _, hit := range response.Hits.Hits {
		// Always use extractLogEntry for robust parsing of various log formats
		// Go's json.Unmarshal doesn't handle epoch milliseconds for time.Time
		var raw map[string]interface{}
		if err := json.Unmarshal(hit.Source, &raw); err == nil {
			entry := extractLogEntry(raw)
			// Store pretty-printed raw JSON for display
			if prettyJSON, err := json.MarshalIndent(raw, "", "  "); err == nil {
				entry.RawJSON = string(prettyJSON)
			} else {
				entry.RawJSON = string(hit.Source)
			}
			result.Logs = append(result.Logs, entry)
		}
	}

	return result, nil
}

// extractLogEntry extracts a LogEntry from raw Elasticsearch document.
//
// DESIGN PRINCIPLE: Progressive Enhancement with Graceful Degradation
// This function handles logs in ANY format - from completely unstructured to fully
// semconv-compliant OTel logs. It should NEVER fail or panic. In the worst case,
// it returns a minimal LogEntry with just the raw data preserved in Attributes.
//
// Enhancement levels (from least to most structured):
// 1. Raw/Unknown: Just preserve everything in Attributes, use current time
// 2. Basic: Has @timestamp and some message field
// 3. Standard: Has level/severity, service name in common locations
// 4. OTel Semconv: Full resource.attributes.service.name, severity_text, body.text
//
// The function tries multiple field locations for each piece of data, preferring
// the most semantically correct location but falling back to alternatives.
func extractLogEntry(raw map[string]interface{}) LogEntry {
	entry := LogEntry{
		Attributes: make(map[string]interface{}),
		Timestamp:  time.Now(), // Default to now if no timestamp found
	}

	// === TIMESTAMP EXTRACTION ===
	// Try multiple formats: OTel epoch millis, epoch micros, ISO strings
	if ts := raw["@timestamp"]; ts != nil {
		switch v := ts.(type) {
		case float64:
			// Numeric epoch milliseconds with fractional microseconds
			millis := int64(v)
			micros := int64((v - float64(millis)) * 1000)
			entry.Timestamp = time.UnixMilli(millis).Add(time.Duration(micros) * time.Microsecond)
		case string:
			// First, try parsing as epoch milliseconds string (OTel format: "1767303679749.488427")
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 1000000000000 {
				// Looks like epoch millis (> year 2001)
				millis := int64(f)
				micros := int64((f - float64(millis)) * 1000)
				entry.Timestamp = time.UnixMilli(millis).Add(time.Duration(micros) * time.Microsecond)
			} else {
				// Try multiple string date formats
				for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
					if t, err := time.Parse(layout, v); err == nil {
						entry.Timestamp = t
						break
					}
				}
			}
		}
	}

	// === MESSAGE/BODY EXTRACTION ===
	// Priority: body.text (OTel) > body (string) > message > event_name > _source as string
	extracted := false

	// OTel format: body is an object with text field
	if body, ok := raw["body"].(map[string]interface{}); ok {
		if text, ok := body["text"].(string); ok && text != "" {
			entry.Body = text
			extracted = true
		}
	}
	// Simple body string
	if !extracted {
		if body, ok := raw["body"].(string); ok && body != "" {
			entry.Body = body
			extracted = true
		}
	}
	// Standard message field
	if msg, ok := raw["message"].(string); ok && msg != "" {
		entry.Message = msg
		if !extracted {
			entry.Body = msg
			extracted = true
		}
	}
	// OTel event_name as fallback
	if !extracted {
		if eventName, ok := raw["event_name"].(string); ok && eventName != "" {
			entry.Body = eventName
		}
	}

	// === LEVEL/SEVERITY EXTRACTION ===
	// Priority: severity_text (OTel) > log.level > level > infer from severity_number
	if severity, ok := raw["severity_text"].(string); ok && severity != "" {
		entry.Level = severity
	} else if level, ok := raw["log.level"].(string); ok && level != "" {
		entry.Level = level
	} else if level, ok := raw["level"].(string); ok && level != "" {
		entry.Level = level
	} else if sevNum, ok := raw["severity_number"].(float64); ok {
		// OTel severity numbers: 1-4=TRACE, 5-8=DEBUG, 9-12=INFO, 13-16=WARN, 17-20=ERROR, 21-24=FATAL
		switch {
		case sevNum <= 4:
			entry.Level = "TRACE"
		case sevNum <= 8:
			entry.Level = "DEBUG"
		case sevNum <= 12:
			entry.Level = "INFO"
		case sevNum <= 16:
			entry.Level = "WARN"
		case sevNum <= 20:
			entry.Level = "ERROR"
		default:
			entry.Level = "FATAL"
		}
	}

	// === SERVICE NAME EXTRACTION ===
	// Priority: resource.attributes.service.name (OTel) > resource.service.name > attributes.service.name > service.name
	if resource, ok := raw["resource"].(map[string]interface{}); ok {
		entry.Resource = resource
		// OTel semconv: resource.attributes.service.name
		if attrs, ok := resource["attributes"].(map[string]interface{}); ok {
			if svcName, ok := attrs["service.name"].(string); ok && svcName != "" {
				entry.ServiceName = svcName
			}
		}
		// Flat resource.service.name
		if entry.ServiceName == "" {
			if svcName, ok := resource["service.name"].(string); ok && svcName != "" {
				entry.ServiceName = svcName
			}
		}
	}
	// Attributes-level service name
	if entry.ServiceName == "" {
		if attrs, ok := raw["attributes"].(map[string]interface{}); ok {
			if svcName, ok := attrs["service.name"].(string); ok && svcName != "" {
				entry.ServiceName = svcName
			}
		}
	}
	// Top-level service.name
	if entry.ServiceName == "" {
		if svcName, ok := raw["service.name"].(string); ok && svcName != "" {
			entry.ServiceName = svcName
		}
	}

	// === CONTAINER ID ===
	if containerID, ok := raw["container_id"].(string); ok {
		entry.ContainerID = containerID
	} else if containerID, ok := raw["container.id"].(string); ok {
		entry.ContainerID = containerID
	}

	// === ATTRIBUTES ===
	// Preserve all attributes for detailed view
	if attrs, ok := raw["attributes"].(map[string]interface{}); ok {
		entry.Attributes = attrs
	}

	// === TRACE-SPECIFIC FIELDS ===
	if traceID, ok := raw["trace_id"].(string); ok {
		entry.TraceID = traceID
	}
	if spanID, ok := raw["span_id"].(string); ok {
		entry.SpanID = spanID
	}
	if name, ok := raw["name"].(string); ok {
		entry.Name = name
	}
	if kind, ok := raw["kind"].(string); ok {
		entry.Kind = kind
	}
	// Duration can be float64 or int
	if dur, ok := raw["duration"].(float64); ok {
		entry.Duration = int64(dur)
	}
	// Status is a nested object
	if status, ok := raw["status"].(map[string]interface{}); ok {
		entry.Status = status
	}

	// === METRICS-SPECIFIC FIELDS ===
	if metrics, ok := raw["metrics"].(map[string]interface{}); ok {
		entry.Metrics = metrics
	}
	if scope, ok := raw["scope"].(map[string]interface{}); ok {
		entry.Scope = scope
	}

	return entry
}

// GetMessage returns the log message, checking multiple possible fields
func (l *LogEntry) GetMessage() string {
	if l.Body != "" {
		return l.Body
	}
	if l.Message != "" {
		return l.Message
	}
	if l.EventName != "" {
		return l.EventName
	}
	// For traces, use span name
	if l.Name != "" {
		return l.Name
	}
	return ""
}

// GetLevel returns the log level with a default
func (l *LogEntry) GetLevel() string {
	if l.Level != "" {
		return l.Level
	}
	return "INFO"
}

// GetResource returns a displayable resource identifier
// Prioritizes: service.namespace > deployment.environment > host.name > first available attribute
func (l *LogEntry) GetResource() string {
	if len(l.Resource) == 0 {
		return ""
	}

	// Check for attributes sub-map (OTel format)
	if attrs, ok := l.Resource["attributes"].(map[string]interface{}); ok {
		// Priority list of resource attributes to display
		priorities := []string{
			"service.namespace",
			"deployment.environment",
			"host.name",
			"k8s.namespace.name",
			"cloud.region",
		}
		for _, key := range priorities {
			if val, ok := attrs[key].(string); ok && val != "" {
				return val
			}
		}
		// Fallback: return first string attribute that isn't service.name
		for k, v := range attrs {
			if k == "service.name" {
				continue
			}
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}

	// Check flat resource fields
	priorities := []string{
		"service.namespace",
		"deployment.environment",
		"host.name",
	}
	for _, key := range priorities {
		if val, ok := l.Resource[key].(string); ok && val != "" {
			return val
		}
	}

	return ""
}

// FieldInfo represents metadata about a field from field_caps API
type FieldInfo struct {
	Name         string // Field name (e.g., "body.text", "severity_text")
	Type         string // Field type (e.g., "keyword", "text", "long")
	Searchable   bool   // Whether the field is searchable
	Aggregatable bool   // Whether the field can be aggregated
	DocCount     int64  // Number of documents with this field (0 = unknown)
}

// GetFieldCaps retrieves available fields from the index using the field_caps API
// and enriches them with document counts
func (c *Client) GetFieldCaps(ctx context.Context) ([]FieldInfo, error) {
	res, err := c.es.FieldCaps(
		c.es.FieldCaps.WithContext(ctx),
		c.es.FieldCaps.WithIndex(c.index+"*"),
		c.es.FieldCaps.WithFields("*"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get field caps: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("field caps failed: %s - %s", res.Status(), string(body))
	}

	var response struct {
		Fields map[string]map[string]struct {
			Type         string `json:"type"`
			Searchable   bool   `json:"searchable"`
			Aggregatable bool   `json:"aggregatable"`
		} `json:"fields"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode field caps: %w", err)
	}

	fields := make([]FieldInfo, 0, len(response.Fields))
	for name, typeMap := range response.Fields {
		// Skip internal fields
		if len(name) > 0 && name[0] == '_' {
			continue
		}
		// Get the first type info (fields can have multiple types across indices)
		for _, info := range typeMap {
			fields = append(fields, FieldInfo{
				Name:         name,
				Type:         info.Type,
				Searchable:   info.Searchable,
				Aggregatable: info.Aggregatable,
			})
			break
		}
	}

	// Enrich with document counts
	c.enrichFieldCounts(ctx, fields)

	return fields, nil
}

// enrichFieldCounts adds document counts to fields using exists queries
func (c *Client) enrichFieldCounts(ctx context.Context, fields []FieldInfo) {
	// Build aggregation query with value_count for each field
	// Limit to first 50 fields to avoid huge queries
	maxFields := 50
	if len(fields) < maxFields {
		maxFields = len(fields)
	}

	aggs := make(map[string]interface{})
	for i := 0; i < maxFields; i++ {
		field := fields[i]
		// For text fields, we need to use exists query approach
		// For keyword/numeric fields, use value_count
		if field.Aggregatable {
			aggs[fmt.Sprintf("f%d", i)] = map[string]interface{}{
				"value_count": map[string]interface{}{
					"field": field.Name,
				},
			}
		} else {
			// Use filter aggregation with exists for non-aggregatable fields
			aggs[fmt.Sprintf("f%d", i)] = map[string]interface{}{
				"filter": map[string]interface{}{
					"exists": map[string]interface{}{
						"field": field.Name,
					},
				},
			}
		}
	}

	query := map[string]interface{}{
		"size": 0,
		"aggs": aggs,
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return // Silently fail - counts are optional
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(c.index+"*"),
		c.es.Search.WithBody(bytes.NewReader(queryJSON)),
		c.es.Search.WithSize(0),
	)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		return
	}

	var aggResponse struct {
		Aggregations map[string]struct {
			Value    int64 `json:"value"`     // For value_count
			DocCount int64 `json:"doc_count"` // For filter
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&aggResponse); err != nil {
		return
	}

	// Map counts back to fields
	for i := 0; i < maxFields; i++ {
		key := fmt.Sprintf("f%d", i)
		if agg, ok := aggResponse.Aggregations[key]; ok {
			if agg.Value > 0 {
				fields[i].DocCount = agg.Value
			} else {
				fields[i].DocCount = agg.DocCount
			}
		}
	}
}

// GetFieldValue extracts a field value from a log entry by field path
func (l *LogEntry) GetFieldValue(fieldPath string) string {
	// Handle built-in fields first
	switch fieldPath {
	case "@timestamp":
		return l.Timestamp.Format("15:04:05")
	case "level", "severity_text":
		return l.GetLevel()
	case "service.name", "resource.attributes.service.name":
		return l.ServiceName
	case "body", "body.text", "message":
		return l.GetMessage()
	case "container_id", "container.id":
		return l.ContainerID
	// Trace-specific fields
	case "trace_id":
		return l.TraceID
	case "span_id":
		return l.SpanID
	case "name":
		if l.Name != "" {
			return l.Name
		}
		return l.GetMessage() // Fallback to message for logs
	case "duration":
		if l.Duration > 0 {
			return fmt.Sprintf("%d", l.Duration)
		}
		return ""
	case "duration_ms":
		if l.Duration > 0 {
			// Duration is in nanoseconds, convert to milliseconds
			ms := float64(l.Duration) / 1_000_000.0
			if ms < 1 {
				return fmt.Sprintf("%.3f", ms)
			}
			return fmt.Sprintf("%.1f", ms)
		}
		return ""
	case "kind":
		return l.Kind
	case "status.code":
		if l.Status != nil {
			if code, ok := l.Status["code"].(string); ok {
				return code
			}
		}
		return ""
	case "scope.name":
		if l.Scope != nil {
			if name, ok := l.Scope["name"].(string); ok {
				return name
			}
		}
		return ""
	case "_metrics":
		// Format metrics as key=value pairs
		if len(l.Metrics) == 0 {
			return ""
		}
		var parts []string
		for k, v := range l.Metrics {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		return strings.Join(parts, ", ")
	}

	// Check attributes - try multiple approaches
	if val, ok := l.Attributes[fieldPath]; ok {
		return fmt.Sprintf("%v", val)
	}

	// Check nested attribute paths (e.g., "attributes.http.method" or "attributes.event.name")
	if len(fieldPath) > 11 && fieldPath[:11] == "attributes." {
		key := fieldPath[11:]
		// First, try direct key lookup (for keys with dots like "event.name")
		if val, ok := l.Attributes[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// Then try traversing nested structure
		if val := getNestedValue(l.Attributes, key); val != "" {
			return val
		}
	}

	// Try the field path directly as a nested path in attributes
	if val := getNestedValue(l.Attributes, fieldPath); val != "" {
		return val
	}

	// Check resource attributes
	if len(l.Resource) > 0 {
		if attrs, ok := l.Resource["attributes"].(map[string]interface{}); ok {
			// Handle "resource.attributes.X" paths
			if len(fieldPath) > 20 && fieldPath[:20] == "resource.attributes." {
				key := fieldPath[20:]
				if val, ok := attrs[key]; ok {
					return fmt.Sprintf("%v", val)
				}
			}
			// Direct key lookup
			if val, ok := attrs[fieldPath]; ok {
				return fmt.Sprintf("%v", val)
			}
		}
		// Check flat resource fields
		if val, ok := l.Resource[fieldPath]; ok {
			return fmt.Sprintf("%v", val)
		}
	}

	return ""
}

// getNestedValue traverses a nested map using a dot-separated path
func getNestedValue(data map[string]interface{}, path string) string {
	if data == nil || path == "" {
		return ""
	}

	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return ""
			}
		default:
			return ""
		}
	}

	if current == nil {
		return ""
	}
	return fmt.Sprintf("%v", current)
}

// Clear deletes all logs from the index
func (c *Client) Clear(ctx context.Context) (int64, error) {
	// Build match_all query
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := c.es.DeleteByQuery(
		[]string{c.index + "*"},
		bytes.NewReader(queryJSON),
		c.es.DeleteByQuery.WithContext(ctx),
		c.es.DeleteByQuery.WithRefresh(true),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete logs: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return 0, fmt.Errorf("delete failed: %s - %s", res.Status(), string(body))
	}

	// Parse response to get deleted count
	var response struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Deleted, nil
}

// LookbackToBucketInterval returns an appropriate ES interval for date_histogram
// based on the lookback duration
func LookbackToBucketInterval(lookback string) string {
	switch lookback {
	case "now-5m":
		return "10s" // 5 min / 30 buckets = 10s
	case "now-1h":
		return "1m" // 1 hour / 60 buckets = 1m
	case "now-24h":
		return "5m" // 24 hours / 288 buckets = 5m
	case "now-1w":
		return "30m" // 1 week / 336 buckets = 30m
	default:
		return "1h" // All time = hourly buckets
	}
}

// GetMetricFieldNames discovers metric field names from field_caps API
// Returns only aggregatable numeric fields under the "metrics.*" namespace
func (c *Client) GetMetricFieldNames(ctx context.Context) ([]MetricFieldInfo, error) {
	res, err := c.es.FieldCaps(
		c.es.FieldCaps.WithContext(ctx),
		c.es.FieldCaps.WithIndex(c.index+"*"),
		c.es.FieldCaps.WithFields("metrics.*"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric field caps: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("field caps failed: %s - %s", res.Status(), string(body))
	}

	var response struct {
		Fields map[string]map[string]struct {
			Type             string `json:"type"`
			Aggregatable     bool   `json:"aggregatable"`
			TimeSeriesMetric string `json:"time_series_metric,omitempty"`
		} `json:"fields"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode field caps: %w", err)
	}

	var metrics []MetricFieldInfo
	for name, typeMap := range response.Fields {
		for _, info := range typeMap {
			// Skip object types (not actual metric values)
			if info.Type == "object" {
				continue
			}
			// Only include aggregatable numeric types
			if !info.Aggregatable {
				continue
			}
			// Filter to long, double, float, histogram, aggregate_metric_double types
			switch info.Type {
			case "long", "double", "float", "half_float", "scaled_float", "histogram", "aggregate_metric_double":
				shortName := name
				if strings.HasPrefix(name, "metrics.") {
					shortName = name[8:] // Remove "metrics." prefix
				}
				metrics = append(metrics, MetricFieldInfo{
					Name:           name,
					ShortName:      shortName,
					Type:           info.Type,
					TimeSeriesType: info.TimeSeriesMetric,
				})
			}
			break // Only process first type
		}
	}

	// Sort by name for consistent display
	sortMetricFields(metrics)

	return metrics, nil
}

// sortMetricFields sorts metric fields by short name
func sortMetricFields(fields []MetricFieldInfo) {
	for i := 0; i < len(fields)-1; i++ {
		for j := i + 1; j < len(fields); j++ {
			if fields[i].ShortName > fields[j].ShortName {
				fields[i], fields[j] = fields[j], fields[i]
			}
		}
	}
}

// AggregateMetrics retrieves aggregated statistics for all discovered metrics
func (c *Client) AggregateMetrics(ctx context.Context, opts AggregateMetricsOptions) (*MetricsAggResult, error) {
	// Discover metrics
	metricFields, err := c.GetMetricFieldNames(ctx)
	if err != nil {
		return nil, err
	}

	if len(metricFields) == 0 {
		return &MetricsAggResult{Metrics: []AggregatedMetric{}, BucketSize: opts.BucketSize}, nil
	}

	// Limit to 50 metrics to avoid huge queries
	maxMetrics := 50
	if len(metricFields) > maxMetrics {
		metricFields = metricFields[:maxMetrics]
	}

	// Build aggregation query
	aggs := make(map[string]interface{})
	for i, mf := range metricFields {
		aggName := fmt.Sprintf("m%d", i)
		aggs[aggName] = map[string]interface{}{
			"filter": map[string]interface{}{
				"exists": map[string]interface{}{
					"field": mf.Name,
				},
			},
			"aggs": map[string]interface{}{
				"stats": map[string]interface{}{
					"extended_stats": map[string]interface{}{
						"field": mf.Name,
					},
				},
				"over_time": map[string]interface{}{
					"date_histogram": map[string]interface{}{
						"field":          "@timestamp",
						"fixed_interval": opts.BucketSize,
					},
					"aggs": map[string]interface{}{
						"value": map[string]interface{}{
							"avg": map[string]interface{}{
								"field": mf.Name,
							},
						},
					},
				},
				"latest": map[string]interface{}{
					"top_hits": map[string]interface{}{
						"size": 1,
						"sort": []map[string]interface{}{
							{"@timestamp": "desc"},
						},
						"_source": []string{mf.Name},
					},
				},
			},
		}
	}

	query := map[string]interface{}{
		"size": 0,
		"aggs": aggs,
	}

	// Add time range filter if specified
	if opts.Lookback != "" {
		query["query"] = map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": opts.Lookback,
				},
			},
		}
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal aggregation query: %w", err)
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(c.index+"*"),
		c.es.Search.WithBody(bytes.NewReader(queryJSON)),
		c.es.Search.WithSize(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("aggregation failed: %s - %s", res.Status(), string(body))
	}

	return parseMetricsAggResponse(res.Body, metricFields, opts.BucketSize)
}

func parseMetricsAggResponse(body io.Reader, fields []MetricFieldInfo, bucketSize string) (*MetricsAggResult, error) {
	// Parse the complex nested aggregation response
	var raw map[string]interface{}
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	aggs, ok := raw["aggregations"].(map[string]interface{})
	if !ok {
		return &MetricsAggResult{Metrics: []AggregatedMetric{}, BucketSize: bucketSize}, nil
	}

	result := &MetricsAggResult{
		Metrics:    make([]AggregatedMetric, 0, len(fields)),
		BucketSize: bucketSize,
	}

	for i, mf := range fields {
		aggName := fmt.Sprintf("m%d", i)
		metricAgg, ok := aggs[aggName].(map[string]interface{})
		if !ok {
			continue
		}

		am := AggregatedMetric{
			Name:      mf.Name,
			ShortName: mf.ShortName,
			Type:      mf.TimeSeriesType,
		}

		// Extract stats
		if stats, ok := metricAgg["stats"].(map[string]interface{}); ok {
			if min, ok := stats["min"].(float64); ok {
				am.Min = min
			}
			if max, ok := stats["max"].(float64); ok {
				am.Max = max
			}
			if avg, ok := stats["avg"].(float64); ok {
				am.Avg = avg
			}
		}

		// Extract time series buckets
		if overTime, ok := metricAgg["over_time"].(map[string]interface{}); ok {
			if buckets, ok := overTime["buckets"].([]interface{}); ok {
				am.Buckets = make([]MetricBucket, 0, len(buckets))
				for _, b := range buckets {
					bucket, ok := b.(map[string]interface{})
					if !ok {
						continue
					}
					mb := MetricBucket{}
					if keyMs, ok := bucket["key"].(float64); ok {
						mb.Timestamp = time.UnixMilli(int64(keyMs))
					}
					if count, ok := bucket["doc_count"].(float64); ok {
						mb.Count = int64(count)
					}
					if value, ok := bucket["value"].(map[string]interface{}); ok {
						if v, ok := value["value"].(float64); ok {
							mb.Value = v
						}
					}
					am.Buckets = append(am.Buckets, mb)
				}
			}
		}

		// Extract latest value from top_hits
		if latest, ok := metricAgg["latest"].(map[string]interface{}); ok {
			if hits, ok := latest["hits"].(map[string]interface{}); ok {
				if hitsList, ok := hits["hits"].([]interface{}); ok && len(hitsList) > 0 {
					if hit, ok := hitsList[0].(map[string]interface{}); ok {
						if source, ok := hit["_source"].(map[string]interface{}); ok {
							am.Latest = extractNestedFloat(source, mf.Name)
						}
					}
				}
			}
		}

		result.Metrics = append(result.Metrics, am)
	}

	return result, nil
}

// extractNestedFloat extracts a float64 from a nested map using dot notation
func extractNestedFloat(data map[string]interface{}, path string) float64 {
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return 0
		}
	}

	if f, ok := current.(float64); ok {
		return f
	}
	return 0
}

// GetTransactionNames returns aggregated transaction names with statistics
func (c *Client) GetTransactionNames(ctx context.Context, lookback string) ([]TransactionNameAgg, error) {
	// Build the aggregation query
	query := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"attributes.processor.event": "transaction",
						},
					},
				},
			},
		},
		"aggs": map[string]interface{}{
			"tx_names": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "name",
					"size":  100,
					"order": map[string]interface{}{
						"_count": "desc",
					},
				},
				"aggs": map[string]interface{}{
					"avg_duration": map[string]interface{}{
						"avg": map[string]interface{}{
							"field": "duration",
						},
					},
					"errors": map[string]interface{}{
						"filter": map[string]interface{}{
							"bool": map[string]interface{}{
								"should": []map[string]interface{}{
									{"term": map[string]interface{}{"status.code": "Error"}},
									{"term": map[string]interface{}{"status.code": "STATUS_CODE_ERROR"}},
									{"range": map[string]interface{}{"status.code": map[string]interface{}{"gte": 2}}},
								},
								"minimum_should_match": 1,
							},
						},
					},
				},
			},
		},
	}

	// Add time range filter if specified
	if lookback != "" {
		filters := query["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"].([]map[string]interface{})
		filters = append(filters, map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": lookback,
				},
			},
		})
		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"] = filters
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(c.index+"*"),
		c.es.Search.WithBody(bytes.NewReader(queryJSON)),
		c.es.Search.WithSize(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("aggregation failed: %s - %s", res.Status(), string(body))
	}

	// Parse response
	var raw map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	aggs, ok := raw["aggregations"].(map[string]interface{})
	if !ok {
		return []TransactionNameAgg{}, nil
	}

	txNames, ok := aggs["tx_names"].(map[string]interface{})
	if !ok {
		return []TransactionNameAgg{}, nil
	}

	buckets, ok := txNames["buckets"].([]interface{})
	if !ok {
		return []TransactionNameAgg{}, nil
	}

	result := make([]TransactionNameAgg, 0, len(buckets))
	for _, b := range buckets {
		bucket, ok := b.(map[string]interface{})
		if !ok {
			continue
		}

		agg := TransactionNameAgg{}

		if name, ok := bucket["key"].(string); ok {
			agg.Name = name
		}
		if count, ok := bucket["doc_count"].(float64); ok {
			agg.Count = int64(count)
		}

		// Extract average duration (convert from nanoseconds to milliseconds)
		if avgDur, ok := bucket["avg_duration"].(map[string]interface{}); ok {
			if v, ok := avgDur["value"].(float64); ok {
				agg.AvgDuration = v / 1_000_000 // nano to ms
			}
		}

		// Calculate error rate
		if errors, ok := bucket["errors"].(map[string]interface{}); ok {
			if errorCount, ok := errors["doc_count"].(float64); ok {
				if agg.Count > 0 {
					agg.ErrorRate = (errorCount / float64(agg.Count)) * 100
				}
			}
		}

		result = append(result, agg)
	}

	return result, nil
}

// PerspectiveAgg represents aggregate counts for a service or resource
type PerspectiveAgg struct {
	Name        string
	LogCount    int64
	TraceCount  int64
	MetricCount int64
}

// GetServices returns aggregated counts per service
func (c *Client) GetServices(ctx context.Context, lookback string) ([]PerspectiveAgg, error) {
	return c.getPerspective(ctx, lookback, "service.name")
}

// GetResources returns aggregated counts per resource environment
func (c *Client) GetResources(ctx context.Context, lookback string) ([]PerspectiveAgg, error) {
	return c.getPerspective(ctx, lookback, "resource.attributes.deployment.environment")
}

// getPerspective aggregates counts of logs, traces, and metrics for a given field
func (c *Client) getPerspective(ctx context.Context, lookback string, field string) ([]PerspectiveAgg, error) {
	index := c.index

	// Build time range filter
	timeFilter := map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"gte": fmt.Sprintf("now-%s", lookback),
			},
		},
	}

	// Build aggregation
	agg := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []interface{}{timeFilter},
			},
		},
		"aggs": map[string]interface{}{
			"items": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": field,
					"size":  100, // Top 100 services/resources
				},
				"aggs": map[string]interface{}{
					// Count logs (excluding transactions and spans)
					"logs": map[string]interface{}{
						"filter": map[string]interface{}{
							"bool": map[string]interface{}{
								"must_not": []interface{}{
									map[string]interface{}{
										"term": map[string]interface{}{
											"processor.event": "transaction",
										},
									},
									map[string]interface{}{
										"term": map[string]interface{}{
											"processor.event": "span",
										},
									},
								},
							},
						},
					},
					// Count traces (transactions only)
					"traces": map[string]interface{}{
						"filter": map[string]interface{}{
							"term": map[string]interface{}{
								"processor.event": "transaction",
							},
						},
					},
					// Count metrics (documents with metrics fields)
					"metrics": map[string]interface{}{
						"filter": map[string]interface{}{
							"exists": map[string]interface{}{
								"field": "metrics",
							},
						},
					},
				},
			},
		},
	}

	// Execute search
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(agg); err != nil {
		return nil, fmt.Errorf("encode query: %w", err)
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(index),
		c.es.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search error: %s: %s", res.Status(), string(body))
	}

	// Parse response
	var result struct {
		Aggregations struct {
			Items struct {
				Buckets []map[string]interface{} `json:"buckets"`
			} `json:"items"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract perspective data
	perspectives := []PerspectiveAgg{}
	for _, bucket := range result.Aggregations.Items.Buckets {
		name, ok := bucket["key"].(string)
		if !ok || name == "" {
			continue
		}

		agg := PerspectiveAgg{Name: name}

		// Extract log count
		if logs, ok := bucket["logs"].(map[string]interface{}); ok {
			if count, ok := logs["doc_count"].(float64); ok {
				agg.LogCount = int64(count)
			}
		}

		// Extract trace count
		if traces, ok := bucket["traces"].(map[string]interface{}); ok {
			if count, ok := traces["doc_count"].(float64); ok {
				agg.TraceCount = int64(count)
			}
		}

		// Extract metric count
		if metrics, ok := bucket["metrics"].(map[string]interface{}); ok {
			if count, ok := metrics["doc_count"].(float64); ok {
				agg.MetricCount = int64(count)
			}
		}

		perspectives = append(perspectives, agg)
	}

	return perspectives, nil
}
