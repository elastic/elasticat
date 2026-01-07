// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildTailQuery_TimeRange(t *testing.T) {
	tests := []struct {
		name    string
		opts    TailOptions
		checkFn func(t *testing.T, query map[string]interface{})
	}{
		{
			name: "lookback filter",
			opts: TailOptions{Lookback: "now-1h"},
			checkFn: func(t *testing.T, query map[string]interface{}) {
				must := getMust(t, query)
				found := false
				for _, clause := range must {
					if rangeClause, ok := clause["range"].(map[string]interface{}); ok {
						if ts, ok := rangeClause["@timestamp"].(map[string]interface{}); ok {
							if ts["gte"] == "now-1h" {
								found = true
							}
						}
					}
				}
				if !found {
					t.Error("Expected time range filter with gte=now-1h")
				}
			},
		},
		{
			name: "since time filter",
			opts: TailOptions{Since: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
			checkFn: func(t *testing.T, query map[string]interface{}) {
				must := getMust(t, query)
				found := false
				for _, clause := range must {
					if rangeClause, ok := clause["range"].(map[string]interface{}); ok {
						if ts, ok := rangeClause["@timestamp"].(map[string]interface{}); ok {
							if gte, ok := ts["gte"].(string); ok && gte == "2024-01-15T10:00:00Z" {
								found = true
							}
						}
					}
				}
				if !found {
					t.Error("Expected time range filter with RFC3339 timestamp")
				}
			},
		},
		{
			name: "no time filter when empty",
			opts: TailOptions{},
			checkFn: func(t *testing.T, query map[string]interface{}) {
				must := getMust(t, query)
				for _, clause := range must {
					if _, ok := clause["range"]; ok {
						t.Error("Expected no time range filter when both Lookback and Since are empty")
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := buildTailQuery(tc.opts)
			tc.checkFn(t, query)
		})
	}
}

func TestBuildTailQuery_ServiceFilter(t *testing.T) {
	tests := []struct {
		name        string
		opts        TailOptions
		shouldMatch bool
		negate      bool
	}{
		{
			name:        "service filter included",
			opts:        TailOptions{Service: "my-service"},
			shouldMatch: true,
			negate:      false,
		},
		{
			name:        "service filter negated",
			opts:        TailOptions{Service: "my-service", NegateService: true},
			shouldMatch: true,
			negate:      true,
		},
		{
			name:        "no service filter when empty",
			opts:        TailOptions{},
			shouldMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := buildTailQuery(tc.opts)

			if tc.negate {
				mustNot := getMustNot(t, query)
				found := hasServiceFilter(mustNot, "my-service")
				if tc.shouldMatch && !found {
					t.Error("Expected service filter in must_not")
				}
			} else {
				must := getMust(t, query)
				found := hasServiceFilter(must, tc.opts.Service)
				if tc.shouldMatch && !found {
					t.Error("Expected service filter in must")
				}
				if !tc.shouldMatch && found {
					t.Error("Expected no service filter")
				}
			}
		})
	}
}

func TestBuildTailQuery_ResourceFilter(t *testing.T) {
	tests := []struct {
		name        string
		opts        TailOptions
		shouldMatch bool
		negate      bool
	}{
		{
			name:        "resource filter included",
			opts:        TailOptions{Resource: "production"},
			shouldMatch: true,
			negate:      false,
		},
		{
			name:        "resource filter negated",
			opts:        TailOptions{Resource: "staging", NegateResource: true},
			shouldMatch: true,
			negate:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := buildTailQuery(tc.opts)

			if tc.negate {
				mustNot := getMustNot(t, query)
				found := hasResourceFilter(mustNot, tc.opts.Resource)
				if tc.shouldMatch && !found {
					t.Error("Expected resource filter in must_not")
				}
			} else {
				must := getMust(t, query)
				found := hasResourceFilter(must, tc.opts.Resource)
				if tc.shouldMatch && !found {
					t.Error("Expected resource filter in must")
				}
			}
		})
	}
}

func TestBuildTailQuery_LevelFilter(t *testing.T) {
	opts := TailOptions{Level: "ERROR"}
	query := buildTailQuery(opts)
	must := getMust(t, query)

	found := false
	for _, clause := range must {
		if boolClause, ok := clause["bool"].(map[string]interface{}); ok {
			if should, ok := boolClause["should"].([]map[string]interface{}); ok {
				for _, s := range should {
					if term, ok := s["term"].(map[string]interface{}); ok {
						if term["severity_text"] == "ERROR" || term["level"] == "ERROR" {
							found = true
						}
					}
				}
			}
		}
	}

	if !found {
		t.Error("Expected level filter with severity_text or level = ERROR")
	}
}

func TestBuildTailQuery_ProcessorEvent(t *testing.T) {
	opts := TailOptions{ProcessorEvent: "transaction"}
	query := buildTailQuery(opts)
	must := getMust(t, query)

	found := false
	for _, clause := range must {
		if term, ok := clause["term"].(map[string]interface{}); ok {
			if term["attributes.processor.event"] == "transaction" {
				found = true
			}
		}
	}

	if !found {
		t.Error("Expected processor.event filter")
	}
}

func TestBuildTailQuery_TransactionName(t *testing.T) {
	opts := TailOptions{TransactionName: "GET /api/users"}
	query := buildTailQuery(opts)
	must := getMust(t, query)

	found := false
	for _, clause := range must {
		if boolClause, ok := clause["bool"].(map[string]interface{}); ok {
			if should, ok := boolClause["should"].([]map[string]interface{}); ok {
				for _, s := range should {
					if term, ok := s["term"].(map[string]interface{}); ok {
						if term["transaction.name"] == "GET /api/users" || term["name"] == "GET /api/users" {
							found = true
						}
					}
				}
			}
		}
	}

	if !found {
		t.Error("Expected transaction.name filter")
	}
}

func TestBuildTailQuery_TraceID(t *testing.T) {
	opts := TailOptions{TraceID: "abc123"}
	query := buildTailQuery(opts)
	must := getMust(t, query)

	found := false
	for _, clause := range must {
		if term, ok := clause["term"].(map[string]interface{}); ok {
			if term["trace_id"] == "abc123" {
				found = true
			}
		}
	}

	if !found {
		t.Error("Expected trace_id filter")
	}
}

func TestBuildTailQuery_ContainerID(t *testing.T) {
	opts := TailOptions{ContainerID: "container123"}
	query := buildTailQuery(opts)
	must := getMust(t, query)

	found := false
	for _, clause := range must {
		if prefix, ok := clause["prefix"].(map[string]interface{}); ok {
			if prefix["container_id"] == "container123" {
				found = true
			}
		}
	}

	if !found {
		t.Error("Expected container_id prefix filter")
	}
}

func TestBuildSearchQuery_FullText(t *testing.T) {
	opts := SearchOptions{}
	query := buildSearchQuery("error message", opts)
	must := getMust(t, query)

	found := false
	for _, clause := range must {
		if qs, ok := clause["query_string"].(map[string]interface{}); ok {
			if q, ok := qs["query"].(string); ok && q == "*error message*" {
				found = true
				// Check default fields
				if fields, ok := qs["fields"].([]string); ok {
					if len(fields) == 0 {
						t.Error("Expected default search fields")
					}
				}
			}
		}
	}

	if !found {
		t.Error("Expected query_string clause with wildcards")
	}
}

func TestBuildSearchQuery_CustomFields(t *testing.T) {
	opts := SearchOptions{
		SearchFields: []string{"custom.field", "another.field"},
	}
	query := buildSearchQuery("test", opts)
	must := getMust(t, query)

	for _, clause := range must {
		if qs, ok := clause["query_string"].(map[string]interface{}); ok {
			if fields, ok := qs["fields"].([]string); ok {
				if len(fields) != 2 || fields[0] != "custom.field" {
					t.Errorf("Expected custom search fields, got %v", fields)
				}
			}
		}
	}
}

func TestBuildSearchQuery_TimeRange(t *testing.T) {
	tests := []struct {
		name    string
		opts    SearchOptions
		checkFn func(t *testing.T, query map[string]interface{})
	}{
		{
			name: "lookback takes priority over From/To",
			opts: SearchOptions{
				Lookback: "now-1h",
				From:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			checkFn: func(t *testing.T, query map[string]interface{}) {
				must := getMust(t, query)
				for _, clause := range must {
					if rangeClause, ok := clause["range"].(map[string]interface{}); ok {
						if ts, ok := rangeClause["@timestamp"].(map[string]interface{}); ok {
							if ts["gte"] != "now-1h" {
								t.Errorf("Expected lookback to take priority, got gte=%v", ts["gte"])
							}
						}
					}
				}
			},
		},
		{
			name: "from and to time range",
			opts: SearchOptions{
				From: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				To:   time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			checkFn: func(t *testing.T, query map[string]interface{}) {
				must := getMust(t, query)
				for _, clause := range must {
					if rangeClause, ok := clause["range"].(map[string]interface{}); ok {
						if ts, ok := rangeClause["@timestamp"].(map[string]interface{}); ok {
							if ts["gte"] != "2024-01-15T10:00:00Z" {
								t.Errorf("Expected gte=2024-01-15T10:00:00Z, got %v", ts["gte"])
							}
							if ts["lte"] != "2024-01-15T11:00:00Z" {
								t.Errorf("Expected lte=2024-01-15T11:00:00Z, got %v", ts["lte"])
							}
						}
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := buildSearchQuery("", tc.opts)
			tc.checkFn(t, query)
		})
	}
}

func TestBuildSearchQuery_Filters(t *testing.T) {
	opts := SearchOptions{
		Service:         "my-service",
		Resource:        "production",
		Level:           "ERROR",
		ProcessorEvent:  "transaction",
		TransactionName: "GET /api",
		TraceID:         "trace123",
	}
	query := buildSearchQuery("test", opts)
	must := getMust(t, query)

	// Check service filter
	if !hasServiceFilter(must, "my-service") {
		t.Error("Expected service filter")
	}

	// Check resource filter
	if !hasResourceFilter(must, "production") {
		t.Error("Expected resource filter")
	}

	// Check that multiple filters are combined
	if len(must) < 5 {
		t.Errorf("Expected at least 5 must clauses, got %d", len(must))
	}
}

func TestBuildSearchQuery_NegatedFilters(t *testing.T) {
	opts := SearchOptions{
		Service:        "excluded-service",
		NegateService:  true,
		Resource:       "excluded-env",
		NegateResource: true,
	}
	query := buildSearchQuery("", opts)
	mustNot := getMustNot(t, query)

	if !hasServiceFilter(mustNot, "excluded-service") {
		t.Error("Expected service filter in must_not")
	}

	if !hasResourceFilter(mustNot, "excluded-env") {
		t.Error("Expected resource filter in must_not")
	}
}

func TestQueryStructure(t *testing.T) {
	// Test that the query has the expected top-level structure
	opts := TailOptions{Service: "test"}
	query := buildTailQuery(opts)

	// Check query > bool > must structure
	q, ok := query["query"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected query key")
	}

	boolQ, ok := q["bool"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected bool key under query")
	}

	_, ok = boolQ["must"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected must key under bool")
	}
}

func TestQueryJSON(t *testing.T) {
	// Test that the query can be serialized to valid JSON
	opts := TailOptions{
		Lookback:       "now-1h",
		Service:        "my-service",
		Level:          "ERROR",
		ProcessorEvent: "transaction",
	}
	query := buildTailQuery(opts)

	jsonBytes, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query to JSON: %v", err)
	}

	// Verify it's valid JSON by unmarshaling back
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal query JSON: %v", err)
	}
}

// Helper functions

func getMust(t *testing.T, query map[string]interface{}) []map[string]interface{} {
	t.Helper()
	q, ok := query["query"].(map[string]interface{})
	if !ok {
		return nil
	}
	boolQ, ok := q["bool"].(map[string]interface{})
	if !ok {
		return nil
	}
	must, ok := boolQ["must"].([]map[string]interface{})
	if !ok {
		return nil
	}
	return must
}

func getMustNot(t *testing.T, query map[string]interface{}) []map[string]interface{} {
	t.Helper()
	q, ok := query["query"].(map[string]interface{})
	if !ok {
		return nil
	}
	boolQ, ok := q["bool"].(map[string]interface{})
	if !ok {
		return nil
	}
	mustNot, ok := boolQ["must_not"].([]map[string]interface{})
	if !ok {
		return nil
	}
	return mustNot
}

func hasServiceFilter(clauses []map[string]interface{}, service string) bool {
	for _, clause := range clauses {
		if boolClause, ok := clause["bool"].(map[string]interface{}); ok {
			if should, ok := boolClause["should"].([]map[string]interface{}); ok {
				for _, s := range should {
					if term, ok := s["term"].(map[string]interface{}); ok {
						if term["resource.attributes.service.name"] == service ||
							term["resource.service.name"] == service {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func hasResourceFilter(clauses []map[string]interface{}, resource string) bool {
	for _, clause := range clauses {
		if term, ok := clause["term"].(map[string]interface{}); ok {
			if term["resource.attributes.deployment.environment"] == resource {
				return true
			}
		}
	}
	return false
}
