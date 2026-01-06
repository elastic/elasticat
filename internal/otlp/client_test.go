// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package otlp

import (
	"testing"

	"github.com/elastic/elasticat/internal/watch"
	"go.opentelemetry.io/otel/log"
)

func TestLevelToSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		level    watch.LogLevel
		expected log.Severity
	}{
		{name: "TRACE maps to SeverityTrace", level: watch.LevelTrace, expected: log.SeverityTrace},
		{name: "DEBUG maps to SeverityDebug", level: watch.LevelDebug, expected: log.SeverityDebug},
		{name: "INFO maps to SeverityInfo", level: watch.LevelInfo, expected: log.SeverityInfo},
		{name: "WARN maps to SeverityWarn", level: watch.LevelWarn, expected: log.SeverityWarn},
		{name: "ERROR maps to SeverityError", level: watch.LevelError, expected: log.SeverityError},
		{name: "FATAL maps to SeverityFatal", level: watch.LevelFatal, expected: log.SeverityFatal},
		{name: "UNKNOWN defaults to SeverityInfo", level: watch.LevelUnknown, expected: log.SeverityInfo},
		{name: "empty string defaults to SeverityInfo", level: watch.LogLevel(""), expected: log.SeverityInfo},
		{name: "arbitrary string defaults to SeverityInfo", level: watch.LogLevel("CUSTOM"), expected: log.SeverityInfo},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := levelToSeverity(tc.level)
			if got != tc.expected {
				t.Errorf("levelToSeverity(%q) = %v, want %v", tc.level, got, tc.expected)
			}
		})
	}
}

func TestLevelToSeverity_AllWatchLevels(t *testing.T) {
	t.Parallel()

	// Ensure all defined LogLevel constants in watch package are handled
	levels := []watch.LogLevel{
		watch.LevelTrace,
		watch.LevelDebug,
		watch.LevelInfo,
		watch.LevelWarn,
		watch.LevelError,
		watch.LevelFatal,
		watch.LevelUnknown,
	}

	for _, level := range levels {
		severity := levelToSeverity(level)
		// Just verify it doesn't panic and returns a valid severity
		if severity < log.SeverityTrace1 && severity != 0 {
			t.Errorf("levelToSeverity(%q) returned invalid severity: %v", level, severity)
		}
	}
}

