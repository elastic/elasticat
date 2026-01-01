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

// Client wraps the Elasticsearch client with turbodevlog-specific functionality
type Client struct {
	es    *elasticsearch.Client
	index string
}

// LogEntry represents a single log entry from Elasticsearch
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
}

// SearchResult contains the results of a log search
type SearchResult struct {
	Logs     []LogEntry
	Total    int64
	ScrollID string
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
	Size        int
	Service     string
	Level       string
	Since       time.Time
	ContainerID string
	SortAsc     bool // true = oldest first, false = newest first (default)
}

// SearchOptions configures the search query
type SearchOptions struct {
	Size         int
	Service      string
	Level        string
	From         time.Time
	To           time.Time
	SortAsc      bool     // true = oldest first, false = newest first (default)
	SearchFields []string // ES fields to search (if empty, uses default body/message fields)
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

	// Time range filter
	timeRange := map[string]interface{}{
		"gte": "now-1h",
	}
	if !opts.Since.IsZero() {
		timeRange["gte"] = opts.Since.Format(time.RFC3339)
	}
	must = append(must, map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": timeRange,
		},
	})

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

	// Time range
	timeRange := map[string]interface{}{}
	if !opts.From.IsZero() {
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
