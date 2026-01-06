// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/elastic/elasticat/internal/es/metrics"
)

// renderLargeChart renders a large ASCII chart for metric visualization
func (m Model) renderLargeChart(buckets []metrics.MetricBucket, minVal, maxVal float64, width, height int) string {
	if len(buckets) == 0 {
		return DetailMutedStyle.Render("No data points")
	}

	var b strings.Builder

	// Y-axis labels width
	yLabelWidth := 10

	// Chart area dimensions
	chartWidth := width - yLabelWidth - 2
	if chartWidth < 10 {
		chartWidth = 10
	}

	// Calculate actual min/max from bucket values for better scaling
	// This is important for histogram types where min/max can be extreme outliers
	// but the bucket values (averages) are much more constrained
	actualMin, actualMax := buckets[0].Value, buckets[0].Value
	for _, bucket := range buckets {
		if bucket.Value < actualMin {
			actualMin = bucket.Value
		}
		if bucket.Value > actualMax {
			actualMax = bucket.Value
		}
	}

	// Use actual bucket range for chart scaling (better visualization)
	// Fall back to provided min/max if bucket range is zero
	if actualMax > actualMin {
		minVal = actualMin
		maxVal = actualMax
	}

	// Handle constant values
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
		minVal = minVal - 0.5
		// maxVal adjustment not needed since we use valRange for normalization
	}

	// Sample buckets to fit chart width
	sampleBuckets := make([]float64, chartWidth)
	step := float64(len(buckets)) / float64(chartWidth)
	for i := 0; i < chartWidth; i++ {
		idx := int(float64(i) * step)
		if idx >= len(buckets) {
			idx = len(buckets) - 1
		}
		sampleBuckets[i] = buckets[idx].Value
	}

	// Render chart rows (top to bottom)
	for row := height - 1; row >= 0; row-- {
		// Y-axis label
		rowValue := minVal + (valRange * float64(row) / float64(height-1))
		yLabel := formatMetricValue(rowValue)
		b.WriteString(DetailMutedStyle.Render(PadLeft(yLabel, yLabelWidth)))
		b.WriteString(" │")

		// Chart row
		for col := 0; col < chartWidth; col++ {
			val := sampleBuckets[col]
			// Normalize to 0..height-1
			normalized := (val - minVal) / valRange * float64(height-1)
			valRow := int(normalized)

			if valRow == row {
				// This is the data point
				b.WriteString(SparklineStyle.Render("█"))
			} else if valRow > row {
				// Value is above this row - show bar
				b.WriteString(SparklineStyle.Render("│"))
			} else {
				// Value is below this row - empty
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}

	// X-axis
	b.WriteString(strings.Repeat(" ", yLabelWidth))
	b.WriteString(" └")
	b.WriteString(strings.Repeat("─", chartWidth))
	b.WriteString("\n")

	// X-axis labels (start and end times)
	if len(buckets) > 0 {
		startTime := buckets[0].Timestamp.Format("15:04:05")
		endTime := buckets[len(buckets)-1].Timestamp.Format("15:04:05")
		padding := chartWidth - len(startTime) - len(endTime)
		if padding < 0 {
			padding = 0
		}
		b.WriteString(strings.Repeat(" ", yLabelWidth+2))
		b.WriteString(DetailMutedStyle.Render(startTime))
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(DetailMutedStyle.Render(endTime))
	}

	return b.String()
}
