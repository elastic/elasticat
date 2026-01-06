// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package es

import "testing"

func TestLookbackToBucketInterval(t *testing.T) {
	tests := []struct {
		lookback string
		expected string
	}{
		{"now-5m", "10s"},
		{"now-1h", "1m"},
		{"now-24h", "5m"},
		{"now-1w", "30m"},
		{"", "1h"},        // All time default
		{"now-30d", "1h"}, // Unknown defaults to 1h
		{"invalid", "1h"}, // Invalid defaults to 1h
	}

	for _, tc := range tests {
		t.Run(tc.lookback, func(t *testing.T) {
			result := LookbackToBucketInterval(tc.lookback)
			if result != tc.expected {
				t.Errorf("LookbackToBucketInterval(%q) = %q, want %q", tc.lookback, result, tc.expected)
			}
		})
	}
}

func TestParseUnsupportedFieldTypeFromESQLResponse(t *testing.T) {
	body := []byte(`{"error":{"root_cause":[{"type":"verification_exception","reason":"Found 1 problem\nline 2:42: Cannot use field [metrics.transaction.duration.histogram] with unsupported type [histogram]"}],"type":"verification_exception","reason":"Found 1 problem\nline 2:42: Cannot use field [metrics.transaction.duration.histogram] with unsupported type [histogram]"},"status":400}`)

	field, typ, ok := parseUnsupportedFieldTypeFromESQLResponse(body)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if field != "metrics.transaction.duration.histogram" {
		t.Fatalf("field=%q, want %q", field, "metrics.transaction.duration.histogram")
	}
	if typ != "histogram" {
		t.Fatalf("type=%q, want %q", typ, "histogram")
	}
}
