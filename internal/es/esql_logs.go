// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0
//
// ES|QL helpers for log and trace retrieval. These mirror Tail/Search behavior
// but execute ES|QL pipelines instead of Query DSL, returning the same
// SearchResult structure used by the TUI.
package es

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/elasticat/internal/es/traces"
)

// TailESQL retrieves the most recent documents via ES|QL using TailOptions.
// Returns the SearchResult plus the rendered ES|QL query string for display.
func (c *Client) TailESQL(ctx context.Context, opts TailOptions) (*SearchResult, string, error) {
	filters := buildCommonFilters(commonFilterOptions{
		indexPattern:    c.index,
		lookback:        opts.Lookback,
		service:         opts.Service,
		negateService:   opts.NegateService,
		resource:        opts.Resource,
		negateResource:  opts.NegateResource,
		level:           opts.Level,
		containerID:     opts.ContainerID,
		processorEvent:  opts.ProcessorEvent,
		transactionName: opts.TransactionName,
		traceID:         opts.TraceID,
		metricField:     opts.MetricField,
	})

	query := buildESQLDocsQuery(c.index, filters, opts.Size, opts.SortAsc)

	result, total, err := c.executeESQLDocs(ctx, query, filters.countQuery)
	if err != nil {
		return nil, query, err
	}

	result.Total = total
	return result, query, nil
}

// SearchESQL performs a text search using ES|QL, preserving the TUI filters.
// Returns the SearchResult plus the ES|QL query string for display.
func (c *Client) SearchESQL(ctx context.Context, queryStr string, opts SearchOptions) (*SearchResult, string, error) {
	searchClause := buildSearchClause(queryStr, opts.SearchFields)
	filters := buildCommonFilters(commonFilterOptions{
		indexPattern:    c.index,
		lookback:        opts.Lookback,
		from:            opts.From,
		to:              opts.To,
		service:         opts.Service,
		negateService:   opts.NegateService,
		resource:        opts.Resource,
		negateResource:  opts.NegateResource,
		level:           opts.Level,
		processorEvent:  opts.ProcessorEvent,
		transactionName: opts.TransactionName,
		traceID:         opts.TraceID,
		searchClause:    searchClause,
	})

	query := buildESQLDocsQuery(c.index, filters, opts.Size, opts.SortAsc)

	result, total, err := c.executeESQLDocs(ctx, query, filters.countQuery)
	if err != nil {
		return nil, query, err
	}

	result.Total = total
	return result, query, nil
}

// CountESQL returns only the document count for the provided TailOptions.
// This is used by auto lookback detection to avoid full fetches.
func (c *Client) CountESQL(ctx context.Context, opts TailOptions) (int64, string, error) {
	filters := buildCommonFilters(commonFilterOptions{
		indexPattern:   c.index,
		lookback:       opts.Lookback,
		service:        opts.Service,
		negateService:  opts.NegateService,
		resource:       opts.Resource,
		negateResource: opts.NegateResource,
		level:          opts.Level,
		processorEvent: opts.ProcessorEvent,
		traceID:        opts.TraceID,
	})

	count, err := c.executeESQLCount(ctx, filters.countQuery)
	return count, filters.countQuery, err
}

// --- internal helpers ---

type commonFilterOptions struct {
	indexPattern    string
	lookback        string
	from            time.Time
	to              time.Time
	service         string
	negateService   bool
	resource        string
	negateResource  bool
	level           string
	containerID     string
	processorEvent  string
	transactionName string
	traceID         string
	metricField     string
	searchClause    string
}

type esqlFilters struct {
	whereParts []string
	countQuery string
}

func buildCommonFilters(opts commonFilterOptions) esqlFilters {
	whereParts := []string{}

	// Time filters
	if opts.lookback != "" {
		whereParts = append(whereParts, fmt.Sprintf("@timestamp >= NOW() - %s", traces.LookbackToESQLInterval(opts.lookback)))
	}
	if !opts.from.IsZero() {
		whereParts = append(whereParts, fmt.Sprintf("@timestamp >= TIMESTAMP(\"%s\")", opts.from.Format(time.RFC3339)))
	}
	if !opts.to.IsZero() {
		whereParts = append(whereParts, fmt.Sprintf("@timestamp <= TIMESTAMP(\"%s\")", opts.to.Format(time.RFC3339)))
	}

	// Service filter
	if opts.service != "" {
		op := "=="
		if opts.negateService {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("service.name %s \"%s\"", op, escapeESQLString(opts.service)))
	}

	// Resource filter
	if opts.resource != "" {
		op := "=="
		if opts.negateResource {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("resource.attributes.deployment.environment %s \"%s\"", op, escapeESQLString(opts.resource)))
	}

	// Level filter - use only severity_text and log.level (standard OTel fields)
	if opts.level != "" {
		lvl := escapeESQLString(opts.level)
		whereParts = append(whereParts, fmt.Sprintf("(COALESCE(severity_text, \"\") == \"%s\" OR COALESCE(log.level, \"\") == \"%s\")", lvl, lvl))
	}

	// Container ID prefix filter (use LIKE with prefix, COALESCE handles missing field)
	if opts.containerID != "" {
		whereParts = append(whereParts, fmt.Sprintf("COALESCE(container_id, \"\") LIKE \"%s*\"", escapeESQLString(opts.containerID)))
	}

	// Processor event filter
	if opts.processorEvent != "" {
		whereParts = append(whereParts, fmt.Sprintf("processor.event == \"%s\"", escapeESQLString(opts.processorEvent)))
	}

	// Transaction name filter (match either transaction.name or name)
	if opts.transactionName != "" {
		tn := escapeESQLString(opts.transactionName)
		whereParts = append(whereParts, fmt.Sprintf("(transaction.name == \"%s\" OR name == \"%s\")", tn, tn))
	}

	// Trace ID filter
	if opts.traceID != "" {
		whereParts = append(whereParts, fmt.Sprintf("(trace.id == \"%s\" OR trace_id == \"%s\")", escapeESQLString(opts.traceID), escapeESQLString(opts.traceID)))
	}

	// Metric field filter (for metric detail view - filter docs containing this metric)
	if opts.metricField != "" {
		// Use backticks to escape the field name which may contain dots
		whereParts = append(whereParts, fmt.Sprintf("`%s` IS NOT NULL", opts.metricField))
	}

	// Search clause from query string
	if opts.searchClause != "" {
		whereParts = append(whereParts, opts.searchClause)
	}

	countQuery := buildCountQuery(opts.indexPattern, whereParts)
	return esqlFilters{
		whereParts: whereParts,
		countQuery: countQuery,
	}
}

func buildCountQuery(indexPattern string, whereParts []string) string {
	where := "WHERE true"
	if len(whereParts) > 0 {
		where = "WHERE " + strings.Join(whereParts, " AND ")
	}
	return fmt.Sprintf(`FROM %s
| %s
| STATS total = COUNT(*)`, indexPattern, where)
}

func buildESQLDocsQuery(indexPattern string, filters esqlFilters, size int, sortAsc bool) string {
	order := "DESC"
	if sortAsc {
		order = "ASC"
	}
	if size == 0 {
		size = 100
	}

	where := "WHERE true"
	if len(filters.whereParts) > 0 {
		where = "WHERE " + strings.Join(filters.whereParts, " AND ")
	}

	return fmt.Sprintf(`FROM %s
| %s
| SORT @timestamp %s
| LIMIT %d
| KEEP *`, indexPattern, where, order, size)
}

func (c *Client) executeESQLDocs(ctx context.Context, query string, countQuery string) (*SearchResult, int64, error) {
	dataRes, err := c.ExecuteESQLQuery(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	entries := make([]LogEntry, 0, len(dataRes.Values))
	for _, row := range dataRes.Values {
		rowMap := esqlRowToMap(dataRes.Columns, row)
		normalizeESQLCompatibility(rowMap)
		entry := extractLogEntry(rowMap)
		if raw, err := json.Marshal(rowMap); err == nil {
			entry.RawJSON = string(raw)
		}
		entries = append(entries, entry)
	}

	total := int64(len(entries))
	if countQuery != "" {
		if count, err := c.executeESQLCount(ctx, countQuery); err == nil {
			total = count
		}
	}

	return &SearchResult{Logs: entries, Total: total}, total, nil
}

func (c *Client) executeESQLCount(ctx context.Context, query string) (int64, error) {
	res, err := c.ExecuteESQLQuery(ctx, query)
	if err != nil {
		return 0, err
	}
	if len(res.Values) == 0 || len(res.Values[0]) == 0 {
		return 0, nil
	}
	if v, ok := res.Values[0][0].(float64); ok {
		return int64(v), nil
	}
	return 0, fmt.Errorf("unexpected ES|QL count result shape")
}

func esqlRowToMap(columns []traces.ESQLColumn, row []interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		if i >= len(row) {
			continue
		}
		value := row[i]
		setPathValue(m, col.Name, value)
	}
	return m
}

// normalizeESQLCompatibility ensures key variants expected by extractLogEntry
// are present (e.g., trace.id -> trace_id).
func normalizeESQLCompatibility(m map[string]interface{}) {
	if v, ok := getNestedMapValue(m, "trace", "id"); ok {
		if _, exists := m["trace_id"]; !exists {
			m["trace_id"] = v
		}
	}
	if v, ok := getNestedMapValue(m, "span", "id"); ok {
		if _, exists := m["span_id"]; !exists {
			m["span_id"] = v
		}
	}
	if v, ok := getNestedMapValue(m, "transaction", "name"); ok {
		if _, exists := m["name"]; !exists {
			m["name"] = v
		}
	}
}

func setPathValue(dst map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := dst
	for i, p := range parts {
		if i == len(parts)-1 {
			current[p] = value
			return
		}
		next, ok := current[p].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[p] = next
		}
		current = next
	}
}

func getNestedMapValue(m map[string]interface{}, path ...string) (interface{}, bool) {
	cur := interface{}(m)
	for _, p := range path {
		asMap, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = asMap[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func buildSearchClause(query string, fields []string) string {
	if query == "" {
		return ""
	}
	// Default to fields that reliably exist in OTel logs indices.
	// Avoid "body" (use body.text), "level" (use severity_text), etc.
	if len(fields) == 0 {
		fields = []string{"body.text", "message", "event_name"}
	}

	q := escapeESQLString(query)
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		// Use COALESCE to handle null values gracefully (empty string won't match)
		parts = append(parts, fmt.Sprintf("COALESCE(%s, \"\") LIKE \"*%s*\"", f, q))
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

func escapeESQLString(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}
