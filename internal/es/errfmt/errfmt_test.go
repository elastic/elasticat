// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package errfmt

import (
	"strings"
	"testing"
)

func TestFormatQueryError(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		body      []byte
		queryJSON []byte
		checks    []string // Substrings that should be in the error
	}{
		{
			name:      "basic error formatting",
			status:    "400 Bad Request",
			body:      []byte(`{"error": "parsing exception"}`),
			queryJSON: []byte(`{"query": {"match_all": {}}}`),
			checks: []string{
				"search failed: 400 Bad Request",
				"parsing exception",
				"match_all",
			},
		},
		{
			name:      "pretty prints valid JSON query",
			status:    "500 Internal Server Error",
			body:      []byte(`server error`),
			queryJSON: []byte(`{"query":{"bool":{"must":[]}}}`),
			checks: []string{
				"search failed: 500 Internal Server Error",
				"server error",
				"\"query\":", // Indicates pretty printing added quotes on new lines
				"\"bool\":",
			},
		},
		{
			name:      "handles invalid JSON query gracefully",
			status:    "400 Bad Request",
			body:      []byte(`error`),
			queryJSON: []byte(`{invalid json`),
			checks: []string{
				"search failed: 400 Bad Request",
				"{invalid json", // Should include raw query
			},
		},
		{
			name:      "handles empty query",
			status:    "400 Bad Request",
			body:      []byte(`error`),
			queryJSON: []byte(``),
			checks: []string{
				"search failed: 400 Bad Request",
			},
		},
		{
			name:      "handles empty body",
			status:    "404 Not Found",
			body:      []byte(``),
			queryJSON: []byte(`{"query": {}}`),
			checks: []string{
				"search failed: 404 Not Found",
				"Error: \n", // Empty error body
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatQueryError(tc.status, tc.body, tc.queryJSON)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			errMsg := err.Error()
			for _, check := range tc.checks {
				if !strings.Contains(errMsg, check) {
					t.Errorf("Error message should contain %q\nGot: %s", check, errMsg)
				}
			}
		})
	}
}

func TestFormatQueryError_PrettyPrintsQuery(t *testing.T) {
	// Verify that a compact JSON query gets pretty-printed
	status := "400 Bad Request"
	body := []byte(`error`)
	compactQuery := []byte(`{"query":{"bool":{"must":[{"match":{"field":"value"}}]}}}`)

	err := FormatQueryError(status, body, compactQuery)
	errMsg := err.Error()

	// Pretty-printed JSON should have newlines and indentation
	if !strings.Contains(errMsg, "\n  ") {
		t.Error("Expected pretty-printed query with newlines and indentation")
	}

	// Should still contain the query content
	if !strings.Contains(errMsg, "match") || !strings.Contains(errMsg, "field") {
		t.Error("Expected query content to be preserved")
	}
}
