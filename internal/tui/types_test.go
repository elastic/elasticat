// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"testing"
	"time"

	"github.com/elastic/elasticat/internal/index"
)

func TestLookbackDuration_String(t *testing.T) {
	tests := []struct {
		lookback LookbackDuration
		expected string
	}{
		{lookback5m, "5m"},
		{lookback1h, "1h"},
		{lookback24h, "24h"},
		{lookback1w, "1w"},
		{lookbackAll, "all"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.lookback.String()
			if result != tc.expected {
				t.Errorf("String() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestLookbackDuration_Duration(t *testing.T) {
	tests := []struct {
		lookback LookbackDuration
		expected time.Duration
	}{
		{lookback5m, 5 * time.Minute},
		{lookback1h, time.Hour},
		{lookback24h, 24 * time.Hour},
		{lookback1w, 7 * 24 * time.Hour},
		{lookbackAll, 0},
	}

	for _, tc := range tests {
		t.Run(tc.lookback.String(), func(t *testing.T) {
			result := tc.lookback.Duration()
			if result != tc.expected {
				t.Errorf("Duration() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestLookbackDuration_ESRange(t *testing.T) {
	tests := []struct {
		lookback LookbackDuration
		expected string
	}{
		{lookback5m, "now-5m"},
		{lookback1h, "now-1h"},
		{lookback24h, "now-24h"},
		{lookback1w, "now-1w"},
		{lookbackAll, "now-24h"}, // Defaults to 24h
	}

	for _, tc := range tests {
		t.Run(tc.lookback.String(), func(t *testing.T) {
			result := tc.lookback.ESRange()
			if result != tc.expected {
				t.Errorf("ESRange() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestSignalType_String(t *testing.T) {
	tests := []struct {
		signal   SignalType
		expected string
	}{
		{SignalLogs, "Logs"},
		{SignalTraces, "Traces"},
		{SignalMetrics, "Metrics"},
		{SignalType(99), "Unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.signal.String()
			if result != tc.expected {
				t.Errorf("String() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestSignalType_IndexPattern(t *testing.T) {
	tests := []struct {
		signal   SignalType
		expected string
	}{
		{SignalLogs, index.Logs},
		{SignalTraces, index.Traces},
		{SignalMetrics, index.Metrics},
		{SignalType(99), index.Logs}, // Defaults to logs
	}

	for _, tc := range tests {
		t.Run(tc.signal.String(), func(t *testing.T) {
			result := tc.signal.IndexPattern()
			if result != tc.expected {
				t.Errorf("IndexPattern() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestPerspectiveType_String(t *testing.T) {
	tests := []struct {
		perspective PerspectiveType
		expected    string
	}{
		{PerspectiveServices, "Services"},
		{PerspectiveResources, "Resources"},
		{PerspectiveType(99), "Unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.perspective.String()
			if result != tc.expected {
				t.Errorf("String() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestDisplayField_GetSearchFields(t *testing.T) {
	tests := []struct {
		name     string
		field    DisplayField
		expected []string
	}{
		{
			name:     "nil search fields returns nil",
			field:    DisplayField{Name: "@timestamp", SearchFields: nil},
			expected: nil,
		},
		{
			name:     "empty search fields uses Name",
			field:    DisplayField{Name: "body.text", SearchFields: []string{}},
			expected: []string{"body.text"},
		},
		{
			name:     "explicit search fields returned",
			field:    DisplayField{Name: "severity_text", SearchFields: []string{"severity_text", "log.level"}},
			expected: []string{"severity_text", "log.level"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.field.GetSearchFields()
			if tc.expected == nil {
				if result != nil {
					t.Errorf("GetSearchFields() = %v, want nil", result)
				}
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("GetSearchFields() length = %d, want %d", len(result), len(tc.expected))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("GetSearchFields()[%d] = %q, want %q", i, v, tc.expected[i])
				}
			}
		})
	}
}

func TestCollectSearchFields(t *testing.T) {
	fields := []DisplayField{
		{Name: "@timestamp", SearchFields: nil},                                    // Not searchable
		{Name: "severity_text", SearchFields: []string{"severity_text", "level"}},  // Two fields
		{Name: "service.name", SearchFields: []string{"service.name", "level"}},    // One overlap (level)
		{Name: "body.text", SearchFields: []string{}},                              // Uses Name
	}

	result := CollectSearchFields(fields)

	// Should have: severity_text, level, service.name, body.text (unique)
	expected := map[string]bool{
		"severity_text": true,
		"level":         true,
		"service.name":  true,
		"body.text":     true,
	}

	if len(result) != len(expected) {
		t.Errorf("CollectSearchFields() length = %d, want %d", len(result), len(expected))
	}

	for _, f := range result {
		if !expected[f] {
			t.Errorf("Unexpected field in result: %q", f)
		}
	}
}

func TestDefaultFields(t *testing.T) {
	tests := []struct {
		signal      SignalType
		minFields   int
		firstLabel  string
	}{
		{SignalLogs, 4, "TIME"},
		{SignalTraces, 5, "TIME"},
		{SignalMetrics, 4, "TIME"},
	}

	for _, tc := range tests {
		t.Run(tc.signal.String(), func(t *testing.T) {
			fields := DefaultFields(tc.signal)
			if len(fields) < tc.minFields {
				t.Errorf("DefaultFields() returned %d fields, want at least %d", len(fields), tc.minFields)
			}
			if fields[0].Label != tc.firstLabel {
				t.Errorf("First field label = %q, want %q", fields[0].Label, tc.firstLabel)
			}
			// All fields should be selected by default
			for _, f := range fields {
				if !f.Selected {
					t.Errorf("Field %q not selected by default", f.Name)
				}
			}
		})
	}
}

func TestHighlighter_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		h        *Highlighter
		expected bool
	}{
		{"nil highlighter", nil, false},
		{"empty query", NewHighlighter(""), false},
		{"with query", NewHighlighter("error"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.h.IsActive()
			if result != tc.expected {
				t.Errorf("IsActive() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestNewHighlighter(t *testing.T) {
	h := NewHighlighter("test query")
	if h == nil {
		t.Fatal("NewHighlighter returned nil")
	}
	if h.Query != "test query" {
		t.Errorf("Query = %q, want %q", h.Query, "test query")
	}
}

