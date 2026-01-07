// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/elastic/elasticat/internal/es/metrics"
)

// generateSparkline creates a sparkline chart from metric buckets
func generateSparkline(buckets []metrics.MetricBucket, width int) string {
	if len(buckets) == 0 {
		return strings.Repeat("-", width)
	}

	// Get min/max for scaling
	var minVal, maxVal float64 = buckets[0].Value, buckets[0].Value
	for _, b := range buckets {
		if b.Value < minVal {
			minVal = b.Value
		}
		if b.Value > maxVal {
			maxVal = b.Value
		}
	}

	// Handle constant values
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
	}

	// Sample or interpolate to fit width
	var result strings.Builder
	step := float64(len(buckets)) / float64(width)

	for i := 0; i < width; i++ {
		idx := int(float64(i) * step)
		if idx >= len(buckets) {
			idx = len(buckets) - 1
		}

		// Normalize to 0-7 range for sparkline chars
		normalized := (buckets[idx].Value - minVal) / valRange
		charIdx := int(normalized * 7)
		if charIdx > 7 {
			charIdx = 7
		}
		if charIdx < 0 {
			charIdx = 0
		}

		result.WriteRune(sparklineChars[charIdx])
	}

	return result.String()
}

// formatMetricValue formats a float64 for compact display
func formatMetricValue(v float64) string {
	if v != v { // NaN check
		return "-"
	}

	absV := v
	if absV < 0 {
		absV = -absV
	}

	switch {
	case absV == 0:
		return "0"
	case absV >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", v/1_000_000_000)
	case absV >= 1_000_000:
		return fmt.Sprintf("%.1fM", v/1_000_000)
	case absV >= 1_000:
		return fmt.Sprintf("%.1fK", v/1_000)
	case absV >= 1:
		return fmt.Sprintf("%.1f", v)
	case absV >= 0.01:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%.3f", v)
	}
}
