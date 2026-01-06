// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "empty", input: "", expected: []string{}},
		{name: "single line no newline", input: "line1", expected: []string{"line1"}},
		{name: "multiple with newline", input: "line1\nline2\n", expected: []string{"line1", "line2"}},
		{name: "windows newlines", input: "line1\r\nline2\r\n", expected: []string{"line1", "line2"}},
		{name: "trailing partial", input: "line1\npartial", expected: []string{"line1", "partial"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := splitLines(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("splitLines length = %d, want %d (%v)", len(got), len(tc.expected), got)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Fatalf("splitLines[%d] = %q, want %q", i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"toolong", 5, "to..."},
		{"abc", 3, "abc"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := truncate(tc.input, tc.max)
			if got != tc.expected {
				t.Fatalf("truncate(%q,%d) = %q, want %q", tc.input, tc.max, got, tc.expected)
			}
		})
	}
}

func TestFormatLog(t *testing.T) {
	base := ParsedLog{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "hello",
		Service:   "svc",
		Source:    "/var/log/app.log",
	}

	tests := []struct {
		name         string
		log          ParsedLog
		noColor      bool
		showFilename bool
		wantParts    []string
		notContains  []string
	}{
		{
			name:         "no color with filename",
			log:          base,
			noColor:      true,
			showFilename: true,
			wantParts:    []string{"[app.log", "10:30:45.000", "INFO", "hello"},
		},
		{
			name:         "color output",
			log:          base,
			noColor:      false,
			showFilename: false,
			wantParts:    []string{LevelInfo.Color(), "INFO", "hello"},
		},
		{
			name:         "different level",
			log:          ParsedLog{Timestamp: base.Timestamp, Level: LevelError, Message: "oops"},
			noColor:      true,
			showFilename: false,
			wantParts:    []string{"ERROR", "oops"},
			notContains:  []string{LevelInfo.Color()}, // ensure level used is ERROR
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out := FormatLog(tc.log, tc.noColor, tc.showFilename)
			for _, part := range tc.wantParts {
				if !strings.Contains(out, part) {
					t.Fatalf("formatted log missing %q: %s", part, out)
				}
			}
			for _, part := range tc.notContains {
				if strings.Contains(out, part) {
					t.Fatalf("formatted log should not contain %q: %s", part, out)
				}
			}
		})
	}
}

func TestWatcherNewAndAccessors(t *testing.T) {
	t.Run("rejects no files", func(t *testing.T) {
		_, err := New(Config{Files: []string{}})
		if err == nil {
			t.Fatalf("expected error for no files")
		}
	})

	t.Run("glob expansion and accessors", func(t *testing.T) {
		dir := t.TempDir()
		file1 := dir + "/a.log"
		file2 := dir + "/b.log"
		if err := writeFile(file1, "line1\n"); err != nil {
			t.Fatalf("setup file1: %v", err)
		}
		if err := writeFile(file2, "line2\n"); err != nil {
			t.Fatalf("setup file2: %v", err)
		}

		w, err := New(Config{Files: []string{dir + "/*.log"}})
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		if w.FileCount() != 2 {
			t.Fatalf("FileCount = %d, want 2", w.FileCount())
		}
		files := w.Files()
		if len(files) != 2 {
			t.Fatalf("Files length = %d, want 2", len(files))
		}
	})
}

func TestWatcherAddHandler(t *testing.T) {
	w := &Watcher{}
	called := 0
	w.AddHandler(func(ParsedLog) { called++ })
	w.callHandlers(ParsedLog{})
	if called != 1 {
		t.Fatalf("expected handler to be called once, got %d", called)
	}
}

// writeFile is a tiny helper to keep tests tidy.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

