// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/elasticat/internal/es/errfmt"
	"github.com/elastic/elasticat/internal/es/shared"
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
// Uses shared.FilterBuilder for common filter clauses.
func buildTailQuery(opts TailOptions) map[string]interface{} {
	fb := shared.NewFilterBuilder()

	// Time range filter - use Lookback if set, otherwise Since
	if opts.Lookback != "" {
		fb.AddTimeRangeFilter(opts.Lookback, "")
	} else if !opts.Since.IsZero() {
		fb.AddTimeRangeFilter(opts.Since.Format(time.RFC3339), "")
	}
	// If both Lookback is empty and Since is zero, no time filter is applied (query all time)

	// Common filters
	fb.AddServiceFilter(opts.Service, opts.NegateService)
	fb.AddResourceFilter(opts.Resource, opts.NegateResource)
	fb.AddLevelFilter(opts.Level)
	fb.AddProcessorEventFilter(opts.ProcessorEvent)
	fb.AddTransactionNameFilter(opts.TransactionName)
	fb.AddTraceIDFilter(opts.TraceID)

	// Tail-specific filters
	fb.AddPrefixFilter("container_id", opts.ContainerID)
	fb.AddExistsFilter(opts.MetricField)

	if opts.Size == 0 {
		opts.Size = 100
	}

	return fb.Build()
}

// buildSearchQuery constructs an ES query for searching logs.
//
// Uses shared.FilterBuilder for common filter clauses.
func buildSearchQuery(queryStr string, opts SearchOptions) map[string]interface{} {
	fb := shared.NewFilterBuilder()

	// Full-text search
	fb.AddQueryString(queryStr, opts.SearchFields)

	// Time range - prefer Lookback, then From/To
	gte := ""
	lte := ""
	if opts.Lookback != "" {
		gte = opts.Lookback
	} else if !opts.From.IsZero() {
		gte = opts.From.Format(time.RFC3339)
	}
	if !opts.To.IsZero() {
		lte = opts.To.Format(time.RFC3339)
	}
	fb.AddTimeRangeFilter(gte, lte)

	// Common filters
	fb.AddServiceFilter(opts.Service, opts.NegateService)
	fb.AddResourceFilter(opts.Resource, opts.NegateResource)
	fb.AddLevelFilter(opts.Level)
	fb.AddProcessorEventFilter(opts.ProcessorEvent)
	fb.AddTransactionNameFilter(opts.TransactionName)
	fb.AddTraceIDFilter(opts.TraceID)

	if opts.Size == 0 {
		opts.Size = 100
	}

	return fb.Build()
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
