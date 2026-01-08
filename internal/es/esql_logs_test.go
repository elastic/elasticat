// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"strings"
	"testing"
	"time"
)

func TestBuildCommonFilters(t *testing.T) {
	t.Parallel()

	t.Run("empty options returns empty where parts", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
		})

		if len(filters.whereParts) != 0 {
			t.Errorf("expected 0 where parts, got %d", len(filters.whereParts))
		}
	})

	t.Run("lookback filter", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			lookback:     "now-15m",
		})

		if len(filters.whereParts) != 1 {
			t.Fatalf("expected 1 where part, got %d", len(filters.whereParts))
		}
		if !strings.Contains(filters.whereParts[0], "@timestamp >= NOW() -") {
			t.Errorf("expected lookback filter, got %q", filters.whereParts[0])
		}
	})

	t.Run("service filter positive", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			service:      "my-service",
		})

		if len(filters.whereParts) != 1 {
			t.Fatalf("expected 1 where part, got %d", len(filters.whereParts))
		}
		expected := `service.name == "my-service"`
		if filters.whereParts[0] != expected {
			t.Errorf("expected %q, got %q", expected, filters.whereParts[0])
		}
	})

	t.Run("service filter negated", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern:  "logs-*",
			service:       "my-service",
			negateService: true,
		})

		expected := `service.name != "my-service"`
		if filters.whereParts[0] != expected {
			t.Errorf("expected %q, got %q", expected, filters.whereParts[0])
		}
	})

	t.Run("resource filter positive", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			resource:     "production",
		})

		expected := `resource.attributes.deployment.environment == "production"`
		if filters.whereParts[0] != expected {
			t.Errorf("expected %q, got %q", expected, filters.whereParts[0])
		}
	})

	t.Run("resource filter negated", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern:   "logs-*",
			resource:       "staging",
			negateResource: true,
		})

		expected := `resource.attributes.deployment.environment != "staging"`
		if filters.whereParts[0] != expected {
			t.Errorf("expected %q, got %q", expected, filters.whereParts[0])
		}
	})

	t.Run("level filter", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			level:        "ERROR",
		})

		if !strings.Contains(filters.whereParts[0], "severity_text") {
			t.Error("expected level filter to check severity_text")
		}
		if !strings.Contains(filters.whereParts[0], "log.level") {
			t.Error("expected level filter to check log.level")
		}
		if !strings.Contains(filters.whereParts[0], `"ERROR"`) {
			t.Error("expected level filter to contain ERROR")
		}
	})

	t.Run("container ID filter", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			containerID:  "abc123",
		})

		if !strings.Contains(filters.whereParts[0], "container_id") {
			t.Error("expected container_id in filter")
		}
		if !strings.Contains(filters.whereParts[0], `"abc123*"`) {
			t.Error("expected container ID prefix pattern")
		}
	})

	t.Run("processor event filter", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern:   "traces-*",
			processorEvent: "transaction",
		})

		expected := `processor.event == "transaction"`
		if filters.whereParts[0] != expected {
			t.Errorf("expected %q, got %q", expected, filters.whereParts[0])
		}
	})

	t.Run("transaction name filter", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern:    "traces-*",
			transactionName: "GET /api/users",
		})

		if !strings.Contains(filters.whereParts[0], "transaction.name") {
			t.Error("expected transaction.name in filter")
		}
		if !strings.Contains(filters.whereParts[0], "OR name ==") {
			t.Error("expected name alternative in filter")
		}
	})

	t.Run("trace ID filter", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "traces-*",
			traceID:      "abc123def456",
		})

		if !strings.Contains(filters.whereParts[0], "trace.id") {
			t.Error("expected trace.id in filter")
		}
		if !strings.Contains(filters.whereParts[0], "trace_id") {
			t.Error("expected trace_id alternative in filter")
		}
	})

	t.Run("metric field filter", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "metrics-*",
			metricField:  "system.cpu.usage",
		})

		expected := "`system.cpu.usage` IS NOT NULL"
		if filters.whereParts[0] != expected {
			t.Errorf("expected %q, got %q", expected, filters.whereParts[0])
		}
	})

	t.Run("search clause is included", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			searchClause: `(COALESCE(body.text, "") LIKE "*error*")`,
		})

		if filters.whereParts[0] != `(COALESCE(body.text, "") LIKE "*error*")` {
			t.Errorf("expected search clause to be included, got %q", filters.whereParts[0])
		}
	})

	t.Run("from time filter", func(t *testing.T) {
		t.Parallel()

		from := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			from:         from,
		})

		if !strings.Contains(filters.whereParts[0], "@timestamp >= TIMESTAMP") {
			t.Error("expected timestamp filter")
		}
		if !strings.Contains(filters.whereParts[0], "2024-01-15") {
			t.Error("expected date in filter")
		}
	})

	t.Run("to time filter", func(t *testing.T) {
		t.Parallel()

		to := time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC)
		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			to:           to,
		})

		if !strings.Contains(filters.whereParts[0], "@timestamp <= TIMESTAMP") {
			t.Error("expected timestamp filter")
		}
	})

	t.Run("multiple filters combined", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			lookback:     "now-1h",
			service:      "api-service",
			level:        "WARN",
		})

		if len(filters.whereParts) != 3 {
			t.Errorf("expected 3 where parts, got %d", len(filters.whereParts))
		}
	})

	t.Run("count query is generated", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			service:      "test-service",
		})

		if !strings.Contains(filters.countQuery, "FROM logs-*") {
			t.Error("expected FROM clause in count query")
		}
		if !strings.Contains(filters.countQuery, "STATS total = COUNT(*)") {
			t.Error("expected STATS COUNT(*) in count query")
		}
		if !strings.Contains(filters.countQuery, "service.name") {
			t.Error("expected service filter in count query")
		}
	})

	t.Run("escapes special characters in service name", func(t *testing.T) {
		t.Parallel()

		filters := buildCommonFilters(commonFilterOptions{
			indexPattern: "logs-*",
			service:      `my-"service"`,
		})

		// Should escape the quotes
		if !strings.Contains(filters.whereParts[0], `\"`) {
			t.Errorf("expected escaped quotes in filter, got %q", filters.whereParts[0])
		}
	})
}

func TestBuildCountQuery(t *testing.T) {
	t.Parallel()

	t.Run("empty where parts", func(t *testing.T) {
		t.Parallel()

		query := buildCountQuery("logs-*", []string{})
		if !strings.Contains(query, "WHERE true") {
			t.Error("expected WHERE true for empty filters")
		}
		if !strings.Contains(query, "FROM logs-*") {
			t.Error("expected FROM clause")
		}
		if !strings.Contains(query, "STATS total = COUNT(*)") {
			t.Error("expected STATS clause")
		}
	})

	t.Run("with where parts", func(t *testing.T) {
		t.Parallel()

		parts := []string{`service.name == "test"`, `level == "ERROR"`}
		query := buildCountQuery("logs-*", parts)

		if strings.Contains(query, "WHERE true") {
			t.Error("should not have WHERE true when filters exist")
		}
		if !strings.Contains(query, "WHERE") {
			t.Error("expected WHERE clause")
		}
		if !strings.Contains(query, "AND") {
			t.Error("expected AND between conditions")
		}
	})
}

func TestBuildESQLDocsQuery(t *testing.T) {
	t.Parallel()

	t.Run("default options", func(t *testing.T) {
		t.Parallel()

		filters := esqlFilters{whereParts: []string{}}
		query := buildESQLDocsQuery("logs-*", filters, 0, false)

		if !strings.Contains(query, "FROM logs-*") {
			t.Error("expected FROM clause")
		}
		if !strings.Contains(query, "SORT @timestamp DESC") {
			t.Error("expected DESC sort by default")
		}
		if !strings.Contains(query, "LIMIT 100") {
			t.Error("expected default LIMIT 100")
		}
		if !strings.Contains(query, "KEEP *") {
			t.Error("expected KEEP *")
		}
	})

	t.Run("ascending sort", func(t *testing.T) {
		t.Parallel()

		filters := esqlFilters{whereParts: []string{}}
		query := buildESQLDocsQuery("logs-*", filters, 50, true)

		if !strings.Contains(query, "SORT @timestamp ASC") {
			t.Error("expected ASC sort")
		}
		if !strings.Contains(query, "LIMIT 50") {
			t.Error("expected LIMIT 50")
		}
	})

	t.Run("with where parts", func(t *testing.T) {
		t.Parallel()

		filters := esqlFilters{whereParts: []string{`service.name == "test"`}}
		query := buildESQLDocsQuery("logs-*", filters, 100, false)

		if !strings.Contains(query, "WHERE service.name") {
			t.Error("expected WHERE clause with filter")
		}
	})
}

func TestBuildSearchClause(t *testing.T) {
	t.Parallel()

	t.Run("empty query returns empty string", func(t *testing.T) {
		t.Parallel()

		result := buildSearchClause("", nil)
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("uses default fields", func(t *testing.T) {
		t.Parallel()

		result := buildSearchClause("error", nil)

		if !strings.Contains(result, "body.text") {
			t.Error("expected body.text in search clause")
		}
		if !strings.Contains(result, "message") {
			t.Error("expected message in search clause")
		}
		if !strings.Contains(result, "event_name") {
			t.Error("expected event_name in search clause")
		}
		if !strings.Contains(result, "*error*") {
			t.Error("expected wildcard search pattern")
		}
	})

	t.Run("uses custom fields", func(t *testing.T) {
		t.Parallel()

		result := buildSearchClause("test", []string{"custom.field", "another.field"})

		if !strings.Contains(result, "custom.field") {
			t.Error("expected custom.field")
		}
		if !strings.Contains(result, "another.field") {
			t.Error("expected another.field")
		}
		if strings.Contains(result, "body.text") {
			t.Error("should not contain default fields when custom fields provided")
		}
	})

	t.Run("uses COALESCE for null safety", func(t *testing.T) {
		t.Parallel()

		result := buildSearchClause("test", nil)

		if !strings.Contains(result, "COALESCE(") {
			t.Error("expected COALESCE for null safety")
		}
	})

	t.Run("uses OR between fields", func(t *testing.T) {
		t.Parallel()

		result := buildSearchClause("test", nil)

		if !strings.Contains(result, " OR ") {
			t.Error("expected OR between field conditions")
		}
	})

	t.Run("wraps in parentheses", func(t *testing.T) {
		t.Parallel()

		result := buildSearchClause("test", nil)

		if !strings.HasPrefix(result, "(") || !strings.HasSuffix(result, ")") {
			t.Error("expected result wrapped in parentheses")
		}
	})

	t.Run("escapes special characters", func(t *testing.T) {
		t.Parallel()

		result := buildSearchClause(`test"query`, nil)

		if !strings.Contains(result, `\"`) {
			t.Errorf("expected escaped quotes, got %q", result)
		}
	})
}

func TestEsqlRowToMap(t *testing.T) {
	t.Parallel()

	t.Run("simple columns", func(t *testing.T) {
		t.Parallel()

		columns := []ESQLColumn{
			{Name: "service.name", Type: "keyword"},
			{Name: "message", Type: "text"},
		}
		row := []interface{}{"my-service", "Hello world"}

		result := esqlRowToMap(columns, row)

		// service.name becomes nested
		serviceMap, ok := result["service"].(map[string]interface{})
		if !ok {
			t.Fatal("expected service to be a map")
		}
		if serviceMap["name"] != "my-service" {
			t.Errorf("expected service.name 'my-service', got %v", serviceMap["name"])
		}

		if result["message"] != "Hello world" {
			t.Errorf("expected message 'Hello world', got %v", result["message"])
		}
	})

	t.Run("deeply nested columns", func(t *testing.T) {
		t.Parallel()

		columns := []ESQLColumn{
			{Name: "resource.attributes.service.name", Type: "keyword"},
		}
		row := []interface{}{"deep-service"}

		result := esqlRowToMap(columns, row)

		// Navigate the nested structure
		resource, _ := result["resource"].(map[string]interface{})
		attrs, _ := resource["attributes"].(map[string]interface{})
		service, _ := attrs["service"].(map[string]interface{})

		if service["name"] != "deep-service" {
			t.Errorf("expected nested name 'deep-service', got %v", service["name"])
		}
	})

	t.Run("handles row shorter than columns", func(t *testing.T) {
		t.Parallel()

		columns := []ESQLColumn{
			{Name: "field1", Type: "keyword"},
			{Name: "field2", Type: "keyword"},
			{Name: "field3", Type: "keyword"},
		}
		row := []interface{}{"value1"}

		result := esqlRowToMap(columns, row)

		if result["field1"] != "value1" {
			t.Error("expected field1 to be set")
		}
		// field2 and field3 should not be set
		if _, exists := result["field2"]; exists {
			t.Error("field2 should not exist")
		}
	})

	t.Run("handles nil values", func(t *testing.T) {
		t.Parallel()

		columns := []ESQLColumn{
			{Name: "field", Type: "keyword"},
		}
		row := []interface{}{nil}

		result := esqlRowToMap(columns, row)

		if result["field"] != nil {
			t.Errorf("expected nil, got %v", result["field"])
		}
	})
}

func TestNormalizeESQLCompatibility(t *testing.T) {
	t.Parallel()

	t.Run("copies trace.id to trace_id", func(t *testing.T) {
		t.Parallel()

		m := map[string]interface{}{
			"trace": map[string]interface{}{
				"id": "trace-123",
			},
		}

		normalizeESQLCompatibility(m)

		if m["trace_id"] != "trace-123" {
			t.Errorf("expected trace_id to be set, got %v", m["trace_id"])
		}
	})

	t.Run("does not overwrite existing trace_id", func(t *testing.T) {
		t.Parallel()

		m := map[string]interface{}{
			"trace": map[string]interface{}{
				"id": "nested-id",
			},
			"trace_id": "existing-id",
		}

		normalizeESQLCompatibility(m)

		if m["trace_id"] != "existing-id" {
			t.Errorf("trace_id should not be overwritten, got %v", m["trace_id"])
		}
	})

	t.Run("copies span.id to span_id", func(t *testing.T) {
		t.Parallel()

		m := map[string]interface{}{
			"span": map[string]interface{}{
				"id": "span-123",
			},
		}

		normalizeESQLCompatibility(m)

		if m["span_id"] != "span-123" {
			t.Errorf("expected span_id to be set, got %v", m["span_id"])
		}
	})

	t.Run("copies transaction.name to name", func(t *testing.T) {
		t.Parallel()

		m := map[string]interface{}{
			"transaction": map[string]interface{}{
				"name": "GET /api/users",
			},
		}

		normalizeESQLCompatibility(m)

		if m["name"] != "GET /api/users" {
			t.Errorf("expected name to be set, got %v", m["name"])
		}
	})

	t.Run("handles missing nested keys", func(t *testing.T) {
		t.Parallel()

		m := map[string]interface{}{
			"other": "value",
		}

		// Should not panic
		normalizeESQLCompatibility(m)

		if _, exists := m["trace_id"]; exists {
			t.Error("trace_id should not be set")
		}
	})
}

func TestSetPathValue(t *testing.T) {
	t.Parallel()

	t.Run("single level path", func(t *testing.T) {
		t.Parallel()

		m := make(map[string]interface{})
		setPathValue(m, "field", "value")

		if m["field"] != "value" {
			t.Errorf("expected 'value', got %v", m["field"])
		}
	})

	t.Run("nested path", func(t *testing.T) {
		t.Parallel()

		m := make(map[string]interface{})
		setPathValue(m, "a.b.c", "nested")

		a, _ := m["a"].(map[string]interface{})
		b, _ := a["b"].(map[string]interface{})

		if b["c"] != "nested" {
			t.Errorf("expected 'nested', got %v", b["c"])
		}
	})

	t.Run("overwrites existing nested map", func(t *testing.T) {
		t.Parallel()

		m := map[string]interface{}{
			"a": map[string]interface{}{
				"b": "old",
			},
		}
		setPathValue(m, "a.b", "new")

		a, _ := m["a"].(map[string]interface{})
		if a["b"] != "new" {
			t.Errorf("expected 'new', got %v", a["b"])
		}
	})
}

func TestEsqlExtractFromPattern(t *testing.T) {
	t.Parallel()

	t.Run("extracts simple pattern", func(t *testing.T) {
		t.Parallel()

		query := "FROM logs-*\n| WHERE true"
		pattern, ok := esqlExtractFromPattern(query)

		if !ok {
			t.Fatal("expected ok to be true")
		}
		if pattern != "logs-*" {
			t.Errorf("expected 'logs-*', got %q", pattern)
		}
	})

	t.Run("extracts multi-index pattern", func(t *testing.T) {
		t.Parallel()

		query := "FROM logs-*,traces-*\n| WHERE true"
		pattern, ok := esqlExtractFromPattern(query)

		if !ok {
			t.Fatal("expected ok to be true")
		}
		if pattern != "logs-*,traces-*" {
			t.Errorf("expected 'logs-*,traces-*', got %q", pattern)
		}
	})

	t.Run("returns false for non-FROM query", func(t *testing.T) {
		t.Parallel()

		query := "SELECT * FROM logs"
		_, ok := esqlExtractFromPattern(query)

		if ok {
			t.Error("expected ok to be false")
		}
	})

	t.Run("handles query without newline", func(t *testing.T) {
		t.Parallel()

		query := "FROM logs-*"
		pattern, ok := esqlExtractFromPattern(query)

		if !ok {
			t.Fatal("expected ok to be true")
		}
		if pattern != "logs-*" {
			t.Errorf("expected 'logs-*', got %q", pattern)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Parallel()

		query := "  FROM   logs-*  \n| WHERE true"
		pattern, ok := esqlExtractFromPattern(query)

		if !ok {
			t.Fatal("expected ok to be true")
		}
		if pattern != "logs-*" {
			t.Errorf("expected 'logs-*', got %q", pattern)
		}
	})
}

func TestEsqlRewriteFromPattern(t *testing.T) {
	t.Parallel()

	t.Run("rewrites pattern", func(t *testing.T) {
		t.Parallel()

		query := "FROM logs-*,traces-*\n| WHERE true"
		result := esqlRewriteFromPattern(query, "logs-*")

		if !strings.HasPrefix(result, "FROM logs-*\n") {
			t.Errorf("expected FROM logs-*, got %q", result)
		}
		if !strings.Contains(result, "WHERE true") {
			t.Error("expected rest of query to be preserved")
		}
	})

	t.Run("returns original for non-FROM query", func(t *testing.T) {
		t.Parallel()

		query := "SELECT * FROM logs"
		result := esqlRewriteFromPattern(query, "new-pattern")

		if result != query {
			t.Errorf("expected original query, got %q", result)
		}
	})

	t.Run("handles query without newline", func(t *testing.T) {
		t.Parallel()

		query := "FROM old-*"
		result := esqlRewriteFromPattern(query, "new-*")

		if result != "FROM new-*" {
			t.Errorf("expected 'FROM new-*', got %q", result)
		}
	})
}

func TestRemoveIndexPattern(t *testing.T) {
	t.Parallel()

	t.Run("removes single pattern", func(t *testing.T) {
		t.Parallel()

		result := removeIndexPattern("logs-*,traces-*,metrics-*", "traces-*")
		if result != "logs-*,metrics-*" {
			t.Errorf("expected 'logs-*,metrics-*', got %q", result)
		}
	})

	t.Run("removes first pattern", func(t *testing.T) {
		t.Parallel()

		result := removeIndexPattern("logs-*,traces-*", "logs-*")
		if result != "traces-*" {
			t.Errorf("expected 'traces-*', got %q", result)
		}
	})

	t.Run("removes last pattern", func(t *testing.T) {
		t.Parallel()

		result := removeIndexPattern("logs-*,traces-*", "traces-*")
		if result != "logs-*" {
			t.Errorf("expected 'logs-*', got %q", result)
		}
	})

	t.Run("returns empty when all removed", func(t *testing.T) {
		t.Parallel()

		result := removeIndexPattern("logs-*", "logs-*")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("handles whitespace", func(t *testing.T) {
		t.Parallel()

		result := removeIndexPattern("logs-* , traces-* , metrics-*", "traces-*")
		if result != "logs-*,metrics-*" {
			t.Errorf("expected 'logs-*,metrics-*', got %q", result)
		}
	})

	t.Run("handles empty parts", func(t *testing.T) {
		t.Parallel()

		result := removeIndexPattern("logs-*,,traces-*", "")
		if result != "logs-*,traces-*" {
			t.Errorf("expected 'logs-*,traces-*', got %q", result)
		}
	})

	t.Run("pattern not found returns original", func(t *testing.T) {
		t.Parallel()

		result := removeIndexPattern("logs-*,traces-*", "metrics-*")
		if result != "logs-*,traces-*" {
			t.Errorf("expected 'logs-*,traces-*', got %q", result)
		}
	})
}
