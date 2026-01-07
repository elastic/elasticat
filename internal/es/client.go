// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/elastic/elasticat/internal/es/metrics"
	"github.com/elastic/elasticat/internal/es/perspectives"
	"github.com/elastic/elasticat/internal/es/shared"
	"github.com/elastic/elasticat/internal/es/traces"
)

// Type definitions have been organized:
// - Core types: types.go (Client, LogEntry, SearchResult, TailOptions, SearchOptions, FieldInfo,
//   SearchResponse, ESQLResult, ESQLColumn)
// - Trace types: traces/types.go (TransactionNameAgg)
// - Metric types: metrics/types.go (MetricFieldInfo, MetricBucket, AggregatedMetric, etc.)
// - Perspective types: perspectives/types.go (PerspectiveAgg)
//
// Operations have been organized:
// - Search operations: search.go (Tail, Search, buildTailQuery, buildSearchQuery, parseSearchResponse)
// - Log entry operations: log_entry.go (extractLogEntry, GetMessage, GetLevel, etc.)
// - Metrics operations: metrics/operations.go
// - Traces operations: traces/operations.go
// - Perspectives operations: perspectives/operations.go

// ClientOptions holds configuration for creating a new Elasticsearch client.
type ClientOptions struct {
	Addresses []string // Elasticsearch addresses (e.g., ["http://localhost:9200"])
	Index     string   // Default index pattern
	APIKey    string   // API key for authentication (base64 encoded)
	Username  string   // Username for basic auth
	Password  string   // Password for basic auth
}

// NewWithOptions creates a new Elasticsearch client with the given options.
// Supports API key authentication and basic auth.
func NewWithOptions(opts ClientOptions) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: opts.Addresses,
	}

	// Configure authentication
	if opts.APIKey != "" {
		cfg.APIKey = opts.APIKey
	} else if opts.Username != "" {
		cfg.Username = opts.Username
		cfg.Password = opts.Password
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ES client: %w", err)
	}

	return &Client{
		es:    es,
		index: opts.Index,
	}, nil
}

// New creates a new Elasticsearch client (for backwards compatibility).
func New(addresses []string, index string) (*Client, error) {
	return NewWithOptions(ClientOptions{
		Addresses: addresses,
		Index:     index,
	})
}

// NewDefault creates a client with default localhost configuration
func NewDefault() (*Client, error) {
	return New([]string{"http://localhost:9200"}, "logs")
}

// NewFromConfig creates a client from the application config.
// This is the recommended way to create clients as it includes auth settings.
func NewFromConfig(url, index, apiKey, username, password string) (*Client, error) {
	return NewWithOptions(ClientOptions{
		Addresses: []string{url},
		Index:     index,
		APIKey:    apiKey,
		Username:  username,
		Password:  password,
	})
}

// SetIndex changes the index pattern
func (c *Client) SetIndex(index string) {
	c.index = index
}

// GetIndex returns the current index pattern
func (c *Client) GetIndex() string {
	return c.index
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

// === Low-level methods for domain package interfaces ===

// FieldCaps returns field capabilities for the given index pattern and fields filter
// Implements metrics.Executor interface
func (c *Client) FieldCaps(ctx context.Context, index, fields string) (*FieldCapsResponse, error) {
	res, err := c.es.FieldCaps(
		c.es.FieldCaps.WithContext(ctx),
		c.es.FieldCaps.WithIndex(index),
		c.es.FieldCaps.WithFields(fields),
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
			Type             string `json:"type"`
			Aggregatable     bool   `json:"aggregatable"`
			TimeSeriesMetric string `json:"time_series_metric,omitempty"`
		} `json:"fields"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode field caps: %w", err)
	}

	// Convert to our response type
	result := &FieldCapsResponse{
		Fields: make(map[string]map[string]FieldCapsInfo),
	}
	for name, typeMap := range response.Fields {
		result.Fields[name] = make(map[string]FieldCapsInfo)
		for typeName, info := range typeMap {
			result.Fields[name][typeName] = FieldCapsInfo{
				Type:             info.Type,
				Aggregatable:     info.Aggregatable,
				TimeSeriesMetric: info.TimeSeriesMetric,
			}
		}
	}

	return result, nil
}

// SearchRaw executes a search query and returns the raw response
// This is the common implementation used by multiple executor interfaces
func (c *Client) SearchRaw(ctx context.Context, index string, body []byte, size int) (io.ReadCloser, int, string, bool, error) {
	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(index),
		c.es.Search.WithBody(bytes.NewReader(body)),
		c.es.Search.WithSize(size),
	)
	if err != nil {
		return nil, 0, "", false, fmt.Errorf("failed to execute search: %w", err)
	}

	return res.Body, res.StatusCode, res.Status(), res.IsError(), nil
}

// SearchForMetrics implements metrics.Executor interface
func (c *Client) SearchForMetrics(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error) {
	bodyReader, statusCode, status, isError, err := c.SearchRaw(ctx, index, body, size)
	if err != nil {
		return nil, err
	}
	return &SearchResponse{
		Body:       bodyReader,
		StatusCode: statusCode,
		Status:     status,
		IsError:    isError,
	}, nil
}

// SearchForTraces implements traces.Executor interface
func (c *Client) SearchForTraces(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error) {
	bodyReader, statusCode, status, isError, err := c.SearchRaw(ctx, index, body, size)
	if err != nil {
		return nil, err
	}
	return &SearchResponse{
		Body:       bodyReader,
		StatusCode: statusCode,
		Status:     status,
		IsError:    isError,
	}, nil
}

// SearchForPerspectives implements perspectives.Executor interface
func (c *Client) SearchForPerspectives(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error) {
	bodyReader, statusCode, status, isError, err := c.SearchRaw(ctx, index, body, size)
	if err != nil {
		return nil, err
	}
	return &SearchResponse{
		Body:       bodyReader,
		StatusCode: statusCode,
		Status:     status,
		IsError:    isError,
	}, nil
}

// ExecuteESQLQuery executes an ES|QL query and returns the structured result
// Implements the ESQLExecutor interface used by traces, metrics, and perspectives
func (c *Client) ExecuteESQLQuery(ctx context.Context, query string) (*ESQLResult, error) {
	// Build request body
	body := map[string]interface{}{
		"query": query,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ES|QL query: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "/_query", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create ES|QL request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute via ES transport
	res, err := c.es.Transport.Perform(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute ES|QL query: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(res.Body)
		if idx, ok := parseUnknownIndexFromESQLResponse(bodyBytes); ok {
			return nil, &shared.ESQLUnknownIndexError{
				Index:  idx,
				Status: res.Status,
				Body:   string(bodyBytes),
			}
		}
		if field, typ, ok := parseUnsupportedFieldTypeFromESQLResponse(bodyBytes); ok {
			return nil, &shared.ESQLUnsupportedFieldTypeError{
				Field:  field,
				Type:   typ,
				Status: res.Status,
				Body:   string(bodyBytes),
			}
		}
		return nil, fmt.Errorf("ES|QL query failed: %s\nError: %s\n\nQuery:\n%s", res.Status, string(bodyBytes), query)
	}

	// Parse response
	var result ESQLResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode ES|QL response: %w", err)
	}

	return &result, nil
}

func parseUnknownIndexFromESQLResponse(body []byte) (string, bool) {
	// Typical response:
	// {"error":{"root_cause":[{"type":"verification_exception","reason":"... Unknown index [traces-*]"}], ...}}
	var resp struct {
		Error struct {
			Reason    string `json:"reason"`
			RootCause []struct {
				Reason string `json:"reason"`
			} `json:"root_cause"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", false
	}

	check := func(s string) (string, bool) {
		const prefix = "Unknown index ["
		i := strings.Index(s, prefix)
		if i == -1 {
			return "", false
		}
		start := i + len(prefix)
		end := strings.Index(s[start:], "]")
		if end == -1 {
			return "", false
		}
		return s[start : start+end], true
	}

	if idx, ok := check(resp.Error.Reason); ok {
		return idx, true
	}
	for _, rc := range resp.Error.RootCause {
		if idx, ok := check(rc.Reason); ok {
			return idx, true
		}
	}
	return "", false
}

func parseUnsupportedFieldTypeFromESQLResponse(body []byte) (field string, typ string, ok bool) {
	// Typical response:
	// {"error":{"root_cause":[{"type":"verification_exception","reason":"... Cannot use field [X] with unsupported type [histogram]"}], ...}}
	var resp struct {
		Error struct {
			Reason    string `json:"reason"`
			RootCause []struct {
				Reason string `json:"reason"`
			} `json:"root_cause"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", false
	}

	check := func(s string) (string, string, bool) {
		const fieldPrefix = "Cannot use field ["
		const typePrefix = "] with unsupported type ["

		i := strings.Index(s, fieldPrefix)
		if i == -1 {
			return "", "", false
		}
		fieldStart := i + len(fieldPrefix)
		j := strings.Index(s[fieldStart:], typePrefix)
		if j == -1 {
			return "", "", false
		}
		f := s[fieldStart : fieldStart+j]

		typeStart := fieldStart + j + len(typePrefix)
		k := strings.Index(s[typeStart:], "]")
		if k == -1 {
			return "", "", false
		}
		t := s[typeStart : typeStart+k]
		return f, t, true
	}

	if f, t, ok := check(resp.Error.Reason); ok {
		return f, t, true
	}
	for _, rc := range resp.Error.RootCause {
		if f, t, ok := check(rc.Reason); ok {
			return f, t, true
		}
	}
	return "", "", false
}

// === Field capabilities ===

// GetFieldCaps retrieves available fields from the index using the field_caps API
// and enriches them with document counts
func (c *Client) GetFieldCaps(ctx context.Context) ([]FieldInfo, error) {
	res, err := c.es.FieldCaps(
		c.es.FieldCaps.WithContext(ctx),
		c.es.FieldCaps.WithIndex(c.index),
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
		c.es.Search.WithIndex(c.index),
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

// === Utility functions ===

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
		[]string{c.index},
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

// === Thin wrappers for domain operations ===

// GetMetricFieldNames discovers metric field names from field_caps API
// Returns only aggregatable numeric fields under the "metrics.*" namespace
func (c *Client) GetMetricFieldNames(ctx context.Context) ([]metrics.MetricFieldInfo, error) {
	return metrics.GetFieldNames(ctx, c, c.index)
}

// AggregateMetrics retrieves aggregated statistics for all discovered metrics.
// Uses Query DSL for full feature support (sparklines, histograms, counters),
// but generates an ES|QL query string for Kibana integration.
func (c *Client) AggregateMetrics(ctx context.Context, opts metrics.AggregateMetricsOptions) (*metrics.MetricsAggResult, error) {
	return metrics.Aggregate(ctx, c, opts)
}

// GetTransactionNames returns aggregated transaction names with statistics
func (c *Client) GetTransactionNames(ctx context.Context, lookback, service, resource string) ([]traces.TransactionNameAgg, error) {
	return traces.GetNames(ctx, c, lookback, service, resource)
}

// GetTransactionNamesESQL retrieves transaction aggregations using ES|QL
func (c *Client) GetTransactionNamesESQL(ctx context.Context, lookback, service, resource string, negateService, negateResource bool) ([]traces.TransactionNameAgg, error) {
	return traces.GetNamesESSQL(ctx, c, lookback, service, resource, negateService, negateResource)
}

// GetServices returns aggregated counts per service
func (c *Client) GetServices(ctx context.Context, lookback string) ([]perspectives.PerspectiveAgg, error) {
	return perspectives.GetServices(ctx, c, lookback)
}

// GetResources returns aggregated counts per resource environment
func (c *Client) GetResources(ctx context.Context, lookback string) ([]perspectives.PerspectiveAgg, error) {
	return perspectives.GetResources(ctx, c, lookback)
}
