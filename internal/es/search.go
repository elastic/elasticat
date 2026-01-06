// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/elasticat/internal/es/errfmt"
)

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

	body, status, isError, err := doSearch(ctx, c.es, c.index, queryJSON, opts.Size, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer body.Close()

	if isError {
		respBody, _ := io.ReadAll(body)
		return nil, errfmt.FormatQueryError(status, respBody, queryJSON)
	}

	return parseSearchResponse(body)
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

	body, status, isError, err := doSearch(ctx, c.es, c.index, queryJSON, opts.Size, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer body.Close()

	if isError {
		respBody, _ := io.ReadAll(body)
		return nil, errfmt.FormatQueryError(status, respBody, queryJSON)
	}

	return parseSearchResponse(body)
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
	mustNot := []map[string]interface{}{}

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
		serviceClause := map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"resource.attributes.service.name": opts.Service}},
					{"term": map[string]interface{}{"resource.service.name": opts.Service}},
				},
				"minimum_should_match": 1,
			},
		}
		if opts.NegateService {
			mustNot = append(mustNot, serviceClause)
		} else {
			must = append(must, serviceClause)
		}
	}

	// Resource filter - filter on deployment environment
	if opts.Resource != "" {
		resourceClause := map[string]interface{}{
			"term": map[string]interface{}{
				"resource.attributes.deployment.environment": opts.Resource,
			},
		}
		if opts.NegateResource {
			mustNot = append(mustNot, resourceClause)
		} else {
			must = append(must, resourceClause)
		}
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

	// Metric field filter (for metric detail view - filter docs containing this metric)
	if opts.MetricField != "" {
		must = append(must, map[string]interface{}{
			"exists": map[string]interface{}{
				"field": opts.MetricField,
			},
		})
	}

	if opts.Size == 0 {
		opts.Size = 100
	}

	boolQuery := map[string]interface{}{
		"must": must,
	}
	if len(mustNot) > 0 {
		boolQuery["must_not"] = mustNot
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
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
	mustNot := []map[string]interface{}{}

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
		serviceClause := map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"resource.attributes.service.name": opts.Service}},
					{"term": map[string]interface{}{"resource.service.name": opts.Service}},
				},
				"minimum_should_match": 1,
			},
		}
		if opts.NegateService {
			mustNot = append(mustNot, serviceClause)
		} else {
			must = append(must, serviceClause)
		}
	}

	// Resource filter - filter on deployment environment
	if opts.Resource != "" {
		resourceClause := map[string]interface{}{
			"term": map[string]interface{}{
				"resource.attributes.deployment.environment": opts.Resource,
			},
		}
		if opts.NegateResource {
			mustNot = append(mustNot, resourceClause)
		} else {
			must = append(must, resourceClause)
		}
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

	boolQuery := map[string]interface{}{
		"must": must,
	}
	if len(mustNot) > 0 {
		boolQuery["must_not"] = mustNot
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
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
			// Store raw JSON (compact) for NDJSON output
			entry.RawJSON = string(hit.Source)
			result.Logs = append(result.Logs, entry)
		}
	}

	return result, nil
}
