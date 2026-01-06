// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package otlp

import (
	"testing"

	"github.com/elastic/elasticat/internal/watch"
	"go.opentelemetry.io/otel/log"
)

func TestLevelToSeverity(t *testing.T) {
	tests := []struct {
		level    watch.LogLevel
		expected log.Severity
	}{
		{watch.LevelTrace, log.SeverityTrace},
		{watch.LevelDebug, log.SeverityDebug},
		{watch.LevelInfo, log.SeverityInfo},
		{watch.LevelWarn, log.SeverityWarn},
		{watch.LevelError, log.SeverityError},
		{watch.LevelFatal, log.SeverityFatal},
		{watch.LogLevel("UNKNOWN"), log.SeverityInfo}, // default
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.level), func(t *testing.T) {
			got := levelToSeverity(tc.level)
			if got != tc.expected {
				t.Fatalf("levelToSeverity(%q) = %v, want %v", tc.level, got, tc.expected)
			}
		})
	}
}

