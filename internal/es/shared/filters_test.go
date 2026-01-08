// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"encoding/json"
	"testing"
)

func TestFilterBuilder_ServiceFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		service     string
		negate      bool
		wantMust    int
		wantMustNot int
	}{
		{
			name:     "empty service adds nothing",
			service:  "",
			wantMust: 0,
		},
		{
			name:     "service filter included",
			service:  "my-service",
			negate:   false,
			wantMust: 1,
		},
		{
			name:        "service filter negated",
			service:     "my-service",
			negate:      true,
			wantMustNot: 1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fb := NewFilterBuilder()
			fb.AddServiceFilter(tc.service, tc.negate)

			if len(fb.Must()) != tc.wantMust {
				t.Errorf("Must() len = %d, want %d", len(fb.Must()), tc.wantMust)
			}
			if len(fb.MustNot()) != tc.wantMustNot {
				t.Errorf("MustNot() len = %d, want %d", len(fb.MustNot()), tc.wantMustNot)
			}
		})
	}
}

func TestFilterBuilder_ResourceFilter(t *testing.T) {
	t.Parallel()

	fb := NewFilterBuilder()
	fb.AddResourceFilter("production", false)

	if len(fb.Must()) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(fb.Must()))
	}

	// Verify the structure
	clause := fb.Must()[0]
	term, ok := clause["term"].(map[string]interface{})
	if !ok {
		t.Fatal("expected term clause")
	}
	if _, ok := term["resource.attributes.deployment.environment"]; !ok {
		t.Error("expected resource.attributes.deployment.environment field")
	}
}

func TestFilterBuilder_LevelFilter(t *testing.T) {
	t.Parallel()

	fb := NewFilterBuilder()
	fb.AddLevelFilter("ERROR")

	if len(fb.Must()) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(fb.Must()))
	}

	// Verify it's a bool query with should clauses
	clause := fb.Must()[0]
	boolQ, ok := clause["bool"].(map[string]interface{})
	if !ok {
		t.Fatal("expected bool clause")
	}
	should, ok := boolQ["should"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected should array")
	}
	if len(should) != 2 {
		t.Errorf("expected 2 should clauses, got %d", len(should))
	}
}

func TestFilterBuilder_TimeRangeFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		gte      string
		lte      string
		wantMust int
	}{
		{
			name:     "empty adds nothing",
			gte:      "",
			lte:      "",
			wantMust: 0,
		},
		{
			name:     "gte only",
			gte:      "now-1h",
			lte:      "",
			wantMust: 1,
		},
		{
			name:     "both gte and lte",
			gte:      "now-1h",
			lte:      "now",
			wantMust: 1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fb := NewFilterBuilder()
			fb.AddTimeRangeFilter(tc.gte, tc.lte)

			if len(fb.Must()) != tc.wantMust {
				t.Errorf("Must() len = %d, want %d", len(fb.Must()), tc.wantMust)
			}
		})
	}
}

func TestFilterBuilder_QueryString(t *testing.T) {
	t.Parallel()

	t.Run("with custom fields", func(t *testing.T) {
		t.Parallel()
		fb := NewFilterBuilder()
		fb.AddQueryString("error", []string{"message", "body"})

		if len(fb.Must()) != 1 {
			t.Fatalf("expected 1 must clause, got %d", len(fb.Must()))
		}

		clause := fb.Must()[0]
		qs, ok := clause["query_string"].(map[string]interface{})
		if !ok {
			t.Fatal("expected query_string clause")
		}
		if qs["query"] != "*error*" {
			t.Errorf("expected query '*error*', got %v", qs["query"])
		}
	})

	t.Run("with default fields", func(t *testing.T) {
		t.Parallel()
		fb := NewFilterBuilder()
		fb.AddQueryString("test", nil)

		if len(fb.Must()) != 1 {
			t.Fatalf("expected 1 must clause, got %d", len(fb.Must()))
		}

		clause := fb.Must()[0]
		qs, ok := clause["query_string"].(map[string]interface{})
		if !ok {
			t.Fatal("expected query_string clause")
		}
		fields, ok := qs["fields"].([]string)
		if !ok {
			t.Fatal("expected fields array")
		}
		if len(fields) != 4 {
			t.Errorf("expected 4 default fields, got %d", len(fields))
		}
	})
}

func TestFilterBuilder_Build(t *testing.T) {
	t.Parallel()

	fb := NewFilterBuilder()
	fb.AddServiceFilter("my-service", false)
	fb.AddLevelFilter("ERROR")
	fb.AddResourceFilter("excluded-env", true)

	query := fb.Build()

	// Verify structure
	q, ok := query["query"].(map[string]interface{})
	if !ok {
		t.Fatal("expected query object")
	}
	boolQ, ok := q["bool"].(map[string]interface{})
	if !ok {
		t.Fatal("expected bool object")
	}

	must, ok := boolQ["must"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected must array")
	}
	if len(must) != 2 {
		t.Errorf("expected 2 must clauses, got %d", len(must))
	}

	mustNot, ok := boolQ["must_not"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected must_not array")
	}
	if len(mustNot) != 1 {
		t.Errorf("expected 1 must_not clause, got %d", len(mustNot))
	}
}

func TestFilterBuilder_ChainedCalls(t *testing.T) {
	t.Parallel()

	// Verify fluent interface works
	query := NewFilterBuilder().
		AddServiceFilter("svc", false).
		AddResourceFilter("prod", false).
		AddLevelFilter("WARN").
		AddTimeRangeFilter("now-1h", "").
		AddTraceIDFilter("trace-123").
		Build()

	// Should produce valid JSON
	_, err := json.Marshal(query)
	if err != nil {
		t.Errorf("Build() produced invalid JSON: %v", err)
	}
}

func TestFilterBuilder_EmptyFilters(t *testing.T) {
	t.Parallel()

	// All empty values should produce no clauses
	fb := NewFilterBuilder()
	fb.AddServiceFilter("", false)
	fb.AddResourceFilter("", false)
	fb.AddLevelFilter("")
	fb.AddTimeRangeFilter("", "")
	fb.AddProcessorEventFilter("")
	fb.AddTransactionNameFilter("")
	fb.AddTraceIDFilter("")
	fb.AddExistsFilter("")
	fb.AddPrefixFilter("", "")
	fb.AddQueryString("", nil)

	if len(fb.Must()) != 0 {
		t.Errorf("expected 0 must clauses for empty filters, got %d", len(fb.Must()))
	}
	if len(fb.MustNot()) != 0 {
		t.Errorf("expected 0 must_not clauses for empty filters, got %d", len(fb.MustNot()))
	}
}
