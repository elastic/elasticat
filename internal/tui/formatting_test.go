// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"
	"testing"
	"time"
)

func TestPadLeft(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"abc", 5, "  abc"},
		{"abc", 3, "abc"},
		{"abcdef", 3, "abc"}, // Truncates
		{"", 3, "   "},
		{"x", 1, "x"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := PadLeft(tc.input, tc.width)
			if result != tc.expected {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tc.input, tc.width, result, tc.expected)
			}
		})
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello world", 11, "hello world"},
		{"hello world", 10, "hello w..."},
		{"hello world", 5, "he..."},
		{"hello", 3, "hel"}, // Too short for ellipsis
		{"hi", 2, "hi"},
		{"hello", 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := TruncateWithEllipsis(tc.input, tc.maxLen)
			if result != tc.expected {
				t.Errorf("TruncateWithEllipsis(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
			}
		})
	}
}

func TestPadOrTruncate(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello     "}, // Pad
		{"hello world", 5, "he..."}, // Truncate with ellipsis
		{"hello", 5, "hello"},       // Exact
		{"hi", 3, "hi "},            // Pad
		{"abcd", 2, "ab"},           // Truncate without ellipsis (too short)
		{"hello", 0, "hello"},       // Zero width returns original
		{"", 5, "     "},            // Empty string pads
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := PadOrTruncate(tc.input, tc.width)
			if result != tc.expected {
				t.Errorf("PadOrTruncate(%q, %d) = %q, want %q", tc.input, tc.width, result, tc.expected)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		width   int
		checkFn func(t *testing.T, result string)
	}{
		{
			name:  "no wrap needed",
			text:  "short",
			width: 10,
			checkFn: func(t *testing.T, result string) {
				if result != "short" {
					t.Errorf("Expected 'short', got %q", result)
				}
			},
		},
		{
			name:  "wraps long line",
			text:  "this is a long line that needs wrapping",
			width: 10,
			checkFn: func(t *testing.T, result string) {
				lines := strings.Split(result, "\n")
				if len(lines) < 2 {
					t.Errorf("Expected multiple lines, got %d", len(lines))
				}
			},
		},
		{
			name:  "preserves existing newlines",
			text:  "line1\nline2\nline3",
			width: 50,
			checkFn: func(t *testing.T, result string) {
				lines := strings.Split(result, "\n")
				if len(lines) != 3 {
					t.Errorf("Expected 3 lines, got %d", len(lines))
				}
			},
		},
		{
			name:  "zero width returns original",
			text:  "test",
			width: 0,
			checkFn: func(t *testing.T, result string) {
				if result != "test" {
					t.Errorf("Expected 'test', got %q", result)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := WrapText(tc.text, tc.width)
			tc.checkFn(t, result)
		})
	}
}

func TestFormatClockTime(t *testing.T) {
	tests := []struct {
		time     time.Time
		expected string
	}{
		{time.Date(2024, 1, 15, 9, 5, 3, 0, time.UTC), "09:05:03"},
		{time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC), "23:59:59"},
		{time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "00:00:00"},
		{time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC), "12:30:45"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := formatClockTime(tc.time)
			if result != tc.expected {
				t.Errorf("formatClockTime() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		contains string
	}{
		{"now", now, "now"},
		{"seconds ago", now.Add(-30 * time.Second), "s ago"},
		{"minutes ago", now.Add(-5 * time.Minute), "m ago"},
		{"hours ago", now.Add(-3 * time.Hour), "h ago"},
		{"days ago", now.Add(-2 * 24 * time.Hour), "d ago"},
		{"weeks ago", now.Add(-2 * 7 * 24 * time.Hour), "w ago"},
		{"months ago", now.Add(-2 * 30 * 24 * time.Hour), "mo ago"},
		{"years ago", now.Add(-2 * 365 * 24 * time.Hour), "y ago"},
		{"future seconds", now.Add(30 * time.Second), "+"},
		{"future minutes", now.Add(5 * time.Minute), "+"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatRelativeTime(tc.time)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("formatRelativeTime() = %q, want to contain %q", result, tc.contains)
			}
		})
	}
}

func TestFormatFullTime(t *testing.T) {
	ti := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	expected := "2024-01-15 10:30:45"
	result := formatFullTime(ti)
	if result != expected {
		t.Errorf("formatFullTime() = %q, want %q", result, expected)
	}
}

// Test the truncateWithEllipsis helper from helpers_test.go (lowercase version)
func TestTruncateWithEllipsisLowercase(t *testing.T) {
	// Test the unexported version used by formatting code
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hi", 2, "hi"},
	}

	for _, tc := range tests {
		result := truncateWithEllipsis(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncateWithEllipsis(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

func TestSingleLine(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello\nworld", "hello world"},
		{"line1\r\nline2", "line1  line2"},
		{"no newlines", "no newlines"},
		{"", ""},
		{"\n\n", "  "},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := singleLine(tc.input)
			if result != tc.expected {
				t.Errorf("singleLine(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
