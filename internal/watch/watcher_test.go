// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSplitLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "empty string", input: "", expected: nil},
		{name: "single line no newline", input: "line1", expected: []string{"line1"}},
		{name: "single line with newline", input: "line1\n", expected: []string{"line1"}},
		{name: "multiple lines", input: "line1\nline2\n", expected: []string{"line1", "line2"}},
		{name: "multiple lines no trailing newline", input: "line1\nline2", expected: []string{"line1", "line2"}},
		{name: "windows CRLF", input: "line1\r\nline2\r\n", expected: []string{"line1", "line2"}},
		{name: "mixed newlines", input: "line1\nline2\r\nline3", expected: []string{"line1", "line2", "line3"}},
		{name: "empty lines preserved", input: "line1\n\nline3\n", expected: []string{"line1", "", "line3"}},
		{name: "just newline", input: "\n", expected: []string{""}},
		{name: "multiple empty lines", input: "\n\n\n", expected: []string{"", "", ""}},
		{name: "trailing partial", input: "line1\npartial", expected: []string{"line1", "partial"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := splitLines(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("splitLines(%q) length = %d, want %d\ngot:  %#v\nwant: %#v",
					tc.input, len(got), len(tc.expected), got, tc.expected)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Fatalf("splitLines(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{name: "shorter than max", input: "short", max: 10, expected: "short"},
		{name: "exact length", input: "exact", max: 5, expected: "exact"},
		{name: "needs truncation", input: "toolong", max: 5, expected: "to..."},
		{name: "at ellipsis boundary", input: "abc", max: 3, expected: "abc"},
		{name: "longer truncation", input: "hello world", max: 8, expected: "hello..."},
		{name: "empty string", input: "", max: 5, expected: ""},
		{name: "max 4 truncates", input: "hello", max: 4, expected: "h..."},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncate(tc.input, tc.max)
			if got != tc.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.expected)
			}
		})
	}
}

func TestFormatLog(t *testing.T) {
	t.Parallel()

	base := ParsedLog{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "hello world",
		Service:   "myservice",
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
			wantParts:    []string{"[app.log", "10:30:45.000", "INFO", "hello world"},
			notContains:  []string{"\033["}, // No ANSI codes
		},
		{
			name:         "no color without filename",
			log:          base,
			noColor:      true,
			showFilename: false,
			wantParts:    []string{"10:30:45.000", "INFO", "hello world"},
			notContains:  []string{"[app.log", "\033["},
		},
		{
			name:         "color output includes ANSI",
			log:          base,
			noColor:      false,
			showFilename: false,
			wantParts:    []string{LevelInfo.Color(), "INFO", "hello world", ColorReset()},
		},
		{
			name:         "error level",
			log:          ParsedLog{Timestamp: base.Timestamp, Level: LevelError, Message: "oops"},
			noColor:      false,
			showFilename: false,
			wantParts:    []string{LevelError.Color(), "ERROR", "oops"},
		},
		{
			name:         "warn level no color",
			log:          ParsedLog{Timestamp: base.Timestamp, Level: LevelWarn, Message: "warning"},
			noColor:      true,
			showFilename: false,
			wantParts:    []string{"WARN", "warning"},
		},
		{
			name:         "debug level",
			log:          ParsedLog{Timestamp: base.Timestamp, Level: LevelDebug, Message: "debug msg"},
			noColor:      false,
			showFilename: false,
			wantParts:    []string{LevelDebug.Color(), "DEBUG", "debug msg"},
		},
		{
			name:         "long filename truncates",
			log:          ParsedLog{Timestamp: base.Timestamp, Level: LevelInfo, Message: "msg", Source: "/path/to/very-long-filename.log"},
			noColor:      true,
			showFilename: true,
			wantParts:    []string{"[very-long-fi...", "msg"}, // Truncated to 15 chars
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := FormatLog(tc.log, tc.noColor, tc.showFilename)
			for _, part := range tc.wantParts {
				if !strings.Contains(out, part) {
					t.Errorf("FormatLog output missing %q\ngot: %q", part, out)
				}
			}
			for _, part := range tc.notContains {
				if strings.Contains(out, part) {
					t.Errorf("FormatLog output should not contain %q\ngot: %q", part, out)
				}
			}
		})
	}
}

func TestWatcherNew(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty file list", func(t *testing.T) {
		t.Parallel()
		_, err := New(Config{Files: []string{}})
		if err == nil {
			t.Error("expected error for empty file list")
		}
		if !strings.Contains(err.Error(), "no files") {
			t.Errorf("error should mention 'no files', got: %v", err)
		}
	})

	t.Run("accepts literal file path", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "test.log")
		if err := writeFile(file, "content\n"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		w, err := New(Config{Files: []string{file}})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		if w.FileCount() != 1 {
			t.Errorf("FileCount() = %d, want 1", w.FileCount())
		}
	})

	t.Run("expands glob patterns", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		for _, name := range []string{"a.log", "b.log", "c.txt"} {
			if err := writeFile(filepath.Join(dir, name), "line\n"); err != nil {
				t.Fatalf("setup: %v", err)
			}
		}

		w, err := New(Config{Files: []string{filepath.Join(dir, "*.log")}})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		if w.FileCount() != 2 {
			t.Errorf("FileCount() = %d, want 2 (*.log matches a.log, b.log)", w.FileCount())
		}
	})

	t.Run("uses custom service name", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "app.log")
		if err := writeFile(file, "test\n"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		w, err := New(Config{Files: []string{file}, Service: "custom-service"})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		if w.service != "custom-service" {
			t.Errorf("service = %q, want %q", w.service, "custom-service")
		}
	})

	t.Run("respects context", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "test.log")
		if err := writeFile(file, "content\n"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Pre-cancel

		w, err := New(Config{Context: ctx, Files: []string{file}})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		// Context should be stored and cancellation should propagate
		if w.ctx.Err() == nil {
			t.Error("expected watcher context to be canceled")
		}
	})
}

func TestWatcherAccessors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "one.log"),
		filepath.Join(dir, "two.log"),
	}
	for _, f := range files {
		if err := writeFile(f, "line\n"); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	w, err := New(Config{Files: files})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Run("FileCount", func(t *testing.T) {
		if got := w.FileCount(); got != 2 {
			t.Errorf("FileCount() = %d, want 2", got)
		}
	})

	t.Run("Files returns copy", func(t *testing.T) {
		got := w.Files()
		if len(got) != 2 {
			t.Errorf("Files() length = %d, want 2", len(got))
		}
		// Verify it contains expected paths
		foundOne, foundTwo := false, false
		for _, f := range got {
			if strings.HasSuffix(f, "one.log") {
				foundOne = true
			}
			if strings.HasSuffix(f, "two.log") {
				foundTwo = true
			}
		}
		if !foundOne || !foundTwo {
			t.Errorf("Files() missing expected files: %v", got)
		}
	})
}

func TestWatcherAddHandler(t *testing.T) {
	t.Parallel()

	t.Run("single handler called", func(t *testing.T) {
		t.Parallel()
		w := &Watcher{handlers: make([]LogHandler, 0)}
		callCount := 0
		w.AddHandler(func(ParsedLog) { callCount++ })
		w.callHandlers(ParsedLog{})
		if callCount != 1 {
			t.Errorf("handler called %d times, want 1", callCount)
		}
	})

	t.Run("multiple handlers called in order", func(t *testing.T) {
		t.Parallel()
		w := &Watcher{handlers: make([]LogHandler, 0)}
		order := []int{}
		w.AddHandler(func(ParsedLog) { order = append(order, 1) })
		w.AddHandler(func(ParsedLog) { order = append(order, 2) })
		w.AddHandler(func(ParsedLog) { order = append(order, 3) })
		w.callHandlers(ParsedLog{})
		if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
			t.Errorf("handlers called in wrong order: %v", order)
		}
	})

	t.Run("handler receives log data", func(t *testing.T) {
		t.Parallel()
		w := &Watcher{handlers: make([]LogHandler, 0)}
		var received ParsedLog
		w.AddHandler(func(log ParsedLog) { received = log })

		sent := ParsedLog{Message: "test message", Level: LevelError}
		w.callHandlers(sent)

		if received.Message != sent.Message || received.Level != sent.Level {
			t.Errorf("handler received %+v, want %+v", received, sent)
		}
	})
}

func TestWatcherStop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "test.log")
	if err := writeFile(file, "content\n"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	w, err := New(Config{Files: []string{file}})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Stop should cancel the context
	w.Stop()
	if w.ctx.Err() == nil {
		t.Error("Stop() should cancel the context")
	}
}

// writeFile is a helper to create test files.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
