// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"testing"
	"time"
)

func TestNormalizeLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		// Standard levels
		{"TRACE", LevelTrace},
		{"trace", LevelTrace},
		{"DEBUG", LevelDebug},
		{"debug", LevelDebug},
		{"INFO", LevelInfo},
		{"info", LevelInfo},
		{"INFORMATION", LevelInfo},
		{"information", LevelInfo},
		{"WARN", LevelWarn},
		{"warn", LevelWarn},
		{"WARNING", LevelWarn},
		{"warning", LevelWarn},
		{"ERROR", LevelError},
		{"error", LevelError},
		{"ERR", LevelError},
		{"err", LevelError},
		{"FATAL", LevelFatal},
		{"fatal", LevelFatal},
		{"CRITICAL", LevelFatal},
		{"critical", LevelFatal},
		{"PANIC", LevelFatal},
		{"panic", LevelFatal},
		// Whitespace handling
		{"  INFO  ", LevelInfo},
		{"\tDEBUG\t", LevelDebug},
		// Unknown levels
		{"", LevelUnknown},
		{"VERBOSE", LevelUnknown},
		{"NOTICE", LevelUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeLevel(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeLevel(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		line     string
		expected LogLevel
	}{
		// Standard log formats
		{"2024-01-01 10:00:00 INFO Starting server", LevelInfo},
		{"2024-01-01 10:00:00 DEBUG Connecting to database", LevelDebug},
		{"2024-01-01 10:00:00 WARN Connection slow", LevelWarn},
		{"2024-01-01 10:00:00 WARNING Connection slow", LevelWarn},
		{"2024-01-01 10:00:00 ERROR Failed to connect", LevelError},
		{"2024-01-01 10:00:00 ERR Failed to connect", LevelError},
		{"2024-01-01 10:00:00 FATAL System crash", LevelFatal},
		{"2024-01-01 10:00:00 CRITICAL System crash", LevelFatal},
		{"2024-01-01 10:00:00 TRACE Entering function", LevelTrace},
		// Case insensitive
		{"info message", LevelInfo},
		{"Info Message", LevelInfo},
		{"[error] something failed", LevelError},
		// No level
		{"plain message without level", LevelUnknown},
		{"", LevelUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
			result := parseLevel(tc.line)
			if result != tc.expected {
				t.Errorf("parseLevel(%q) = %q, want %q", tc.line, result, tc.expected)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected time.Time
		fuzzy    bool // If true, just check that it's not zero (for "now" cases)
	}{
		{
			name:     "RFC3339Nano",
			line:     "2024-01-15T10:30:45.123456789Z INFO message",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC),
		},
		{
			name:     "RFC3339",
			line:     "2024-01-15T10:30:45Z INFO message",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:     "space separated with millis",
			line:     "2024-01-15 10:30:45.123 INFO message",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 123000000, time.UTC),
		},
		{
			name:     "space separated no millis",
			line:     "2024-01-15 10:30:45 INFO message",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:     "slash separated",
			line:     "2024/01/15 10:30:45 INFO message",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:  "no timestamp",
			line:  "just a plain message",
			fuzzy: true, // Will be time.Now()
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseTimestamp(tc.line)
			if tc.fuzzy {
				// For fuzzy tests, just ensure we got a recent timestamp
				if time.Since(result) > time.Second {
					t.Errorf("parseTimestamp(%q) returned non-recent time: %v", tc.line, result)
				}
			} else {
				if !result.Equal(tc.expected) {
					t.Errorf("parseTimestamp(%q) = %v, want %v", tc.line, result, tc.expected)
				}
			}
		})
	}
}

func TestParseTimestampString(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
		isZero   bool
	}{
		{
			input:    "2024-01-15T10:30:45.123456789Z",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC),
		},
		{
			input:    "2024-01-15T10:30:45Z",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			input:    "2024-01-15T10:30:45.000Z",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			input:    "2024-01-15 10:30:45.123",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 123000000, time.UTC),
		},
		{
			input:    "2024-01-15 10:30:45",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			input:    "2024/01/15 10:30:45",
			expected: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			input:  "invalid",
			isZero: true,
		},
		{
			input:  "",
			isZero: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := parseTimestampString(tc.input)
			if tc.isZero {
				if !result.IsZero() {
					t.Errorf("parseTimestampString(%q) = %v, want zero time", tc.input, result)
				}
			} else if !result.Equal(tc.expected) {
				t.Errorf("parseTimestampString(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestServiceFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		// Basic cases
		{"server.log", "server"},
		{"api.log", "api"},
		{"app.txt", "app"},
		// With path
		{"/var/log/server.log", "server"},
		{"/home/user/logs/api.log", "api"},
		{"./logs/service.log", "service"},
		// With common suffixes
		{"server-err.log", "server"},
		{"server-error.log", "server"},
		{"server-out.log", "server"},
		{"server-info.log", "server"},
		{"server-debug.log", "server"},
		{"server-log.log", "server"},
		// Case insensitive suffix removal
		{"Server-ERR.log", "Server"},
		{"API-ERROR.log", "API"},
		// Edge cases
		{".log", "unknown"},
		{"", "unknown"},
		{"noextension", "noextension"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := ServiceFromFilename(tc.filename)
			if result != tc.expected {
				t.Errorf("ServiceFromFilename(%q) = %q, want %q", tc.filename, result, tc.expected)
			}
		})
	}
}

func TestParseJSONLog(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantNil   bool
		wantMsg   string
		wantLevel LogLevel
		wantAttrs map[string]interface{}
		checkTime bool
		wantTime  time.Time
		fuzzyTime bool // Check that time is recent, not zero
	}{
		{
			name:      "message field",
			line:      `{"message": "hello world", "level": "info"}`,
			wantMsg:   "hello world",
			wantLevel: LevelInfo,
		},
		{
			name:      "msg field",
			line:      `{"msg": "hello world", "level": "debug"}`,
			wantMsg:   "hello world",
			wantLevel: LevelDebug,
		},
		{
			name:      "log field",
			line:      `{"log": "hello world", "severity": "error"}`,
			wantMsg:   "hello world",
			wantLevel: LevelError,
		},
		{
			name:      "text field",
			line:      `{"text": "hello world", "lvl": "warn"}`,
			wantMsg:   "hello world",
			wantLevel: LevelWarn,
		},
		{
			name:      "body field",
			line:      `{"body": "hello world", "loglevel": "fatal"}`,
			wantMsg:   "hello world",
			wantLevel: LevelFatal,
		},
		{
			name:      "timestamp as string RFC3339",
			line:      `{"message": "test", "timestamp": "2024-01-15T10:30:45Z"}`,
			wantMsg:   "test",
			wantLevel: LevelUnknown,
			checkTime: true,
			wantTime:  time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:      "time field",
			line:      `{"message": "test", "time": "2024-01-15T10:30:45Z"}`,
			wantMsg:   "test",
			wantLevel: LevelUnknown,
			checkTime: true,
			wantTime:  time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:      "@timestamp field",
			line:      `{"message": "test", "@timestamp": "2024-01-15T10:30:45Z"}`,
			wantMsg:   "test",
			wantLevel: LevelUnknown,
			checkTime: true,
			wantTime:  time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:      "timestamp as epoch seconds",
			line:      `{"message": "test", "ts": 1705315845}`,
			wantMsg:   "test",
			wantLevel: LevelUnknown,
			checkTime: true,
			wantTime:  time.Unix(1705315845, 0),
		},
		{
			name:      "timestamp as epoch milliseconds",
			line:      `{"message": "test", "ts": 1705315845000}`,
			wantMsg:   "test",
			wantLevel: LevelUnknown,
			checkTime: true,
			wantTime:  time.UnixMilli(1705315845000),
		},
		{
			name:      "extra attributes preserved",
			line:      `{"message": "test", "level": "info", "request_id": "abc123", "user": "john"}`,
			wantMsg:   "test",
			wantLevel: LevelInfo,
			wantAttrs: map[string]interface{}{"request_id": "abc123", "user": "john"},
		},
		{
			name:      "no message field",
			line:      `{"level": "info", "data": "something"}`,
			wantMsg:   "",
			wantLevel: LevelInfo,
			wantAttrs: map[string]interface{}{"data": "something"},
		},
		{
			name:    "invalid JSON",
			line:    `{invalid json}`,
			wantNil: true,
		},
		{
			name:    "not JSON",
			line:    `plain text log line`,
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseJSONLog(tc.line)

			if tc.wantNil {
				if result != nil {
					t.Errorf("parseJSONLog(%q) = %v, want nil", tc.line, result)
				}
				return
			}

			if result == nil {
				t.Fatalf("parseJSONLog(%q) = nil, want non-nil", tc.line)
			}

			if result.Message != tc.wantMsg {
				t.Errorf("Message = %q, want %q", result.Message, tc.wantMsg)
			}

			if result.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", result.Level, tc.wantLevel)
			}

			if tc.checkTime {
				if !result.Timestamp.Equal(tc.wantTime) {
					t.Errorf("Timestamp = %v, want %v", result.Timestamp, tc.wantTime)
				}
			} else if tc.fuzzyTime {
				if time.Since(result.Timestamp) > time.Second {
					t.Errorf("Timestamp = %v, want recent time", result.Timestamp)
				}
			}

			if tc.wantAttrs != nil {
				for k, v := range tc.wantAttrs {
					if result.Attributes[k] != v {
						t.Errorf("Attributes[%q] = %v, want %v", k, result.Attributes[k], v)
					}
				}
			}
		})
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		filename  string
		service   string
		wantJSON  bool
		wantMsg   string
		wantLevel LogLevel
	}{
		{
			name:      "JSON log",
			line:      `{"message": "request processed", "level": "info"}`,
			filename:  "server.log",
			service:   "api",
			wantJSON:  true,
			wantMsg:   "request processed",
			wantLevel: LevelInfo,
		},
		{
			name:      "plain text log with level",
			line:      "2024-01-15 10:30:45 INFO Starting server on port 8080",
			filename:  "server.log",
			service:   "api",
			wantJSON:  false,
			wantMsg:   "2024-01-15 10:30:45 INFO Starting server on port 8080",
			wantLevel: LevelInfo,
		},
		{
			name:      "plain text error",
			line:      "[ERROR] Connection refused",
			filename:  "app.log",
			service:   "myapp",
			wantJSON:  false,
			wantMsg:   "[ERROR] Connection refused",
			wantLevel: LevelError,
		},
		{
			name:      "plain text no level",
			line:      "just a message",
			filename:  "output.log",
			service:   "worker",
			wantJSON:  false,
			wantMsg:   "just a message",
			wantLevel: LevelUnknown,
		},
		{
			name:      "whitespace before JSON",
			line:      `  {"message": "test", "level": "debug"}`,
			filename:  "server.log",
			service:   "api",
			wantJSON:  true,
			wantMsg:   "test",
			wantLevel: LevelDebug,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseLine(tc.line, tc.filename, tc.service)

			if result.IsJSON != tc.wantJSON {
				t.Errorf("IsJSON = %v, want %v", result.IsJSON, tc.wantJSON)
			}

			if result.Message != tc.wantMsg {
				t.Errorf("Message = %q, want %q", result.Message, tc.wantMsg)
			}

			if result.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", result.Level, tc.wantLevel)
			}

			if result.Source != tc.filename {
				t.Errorf("Source = %q, want %q", result.Source, tc.filename)
			}

			if result.Service != tc.service {
				t.Errorf("Service = %q, want %q", result.Service, tc.service)
			}

			if result.RawLine != tc.line {
				t.Errorf("RawLine = %q, want %q", result.RawLine, tc.line)
			}
		})
	}
}

func TestLogLevelColor(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LevelTrace, "\033[90m"},
		{LevelDebug, "\033[36m"},
		{LevelInfo, "\033[32m"},
		{LevelWarn, "\033[33m"},
		{LevelError, "\033[31m"},
		{LevelFatal, "\033[35m"},
		{LevelUnknown, "\033[0m"},
	}

	for _, tc := range tests {
		t.Run(string(tc.level), func(t *testing.T) {
			result := tc.level.Color()
			if result != tc.expected {
				t.Errorf("%s.Color() = %q, want %q", tc.level, result, tc.expected)
			}
		})
	}
}

func TestColorReset(t *testing.T) {
	expected := "\033[0m"
	result := ColorReset()
	if result != expected {
		t.Errorf("ColorReset() = %q, want %q", result, expected)
	}
}
