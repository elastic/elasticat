// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package metrics

import "time"

// MetricFieldInfo represents a discovered metric field from field_caps
type MetricFieldInfo struct {
	Name           string // Full field path (e.g., "metrics.raradio.session.active")
	ShortName      string // Display name (e.g., "raradio.session.active")
	Type           string // ES type: "long", "double", "histogram"
	TimeSeriesType string // "gauge", "counter", or ""
}

// MetricBucket represents a single time bucket for a metric
type MetricBucket struct {
	Timestamp time.Time
	Value     float64 // Aggregated value for this bucket
	Count     int64   // Number of data points in bucket
}

// AggregatedMetric represents aggregated statistics for a single metric
type AggregatedMetric struct {
	Name      string // Metric field name
	ShortName string // Display name
	Type      string // "gauge", "counter", "histogram"
	Min       float64
	Max       float64
	Avg       float64
	Latest    float64
	Buckets   []MetricBucket // Time series data for sparkline
}

// MetricsAggResult contains all aggregated metrics
type MetricsAggResult struct {
	Metrics    []AggregatedMetric
	BucketSize string // ES interval (e.g., "10s", "1m")
}

// AggregateMetricsOptions configures the metrics aggregation query
type AggregateMetricsOptions struct {
	Lookback       string // ES time range (e.g., "now-5m", "now-1h")
	BucketSize     string // ES interval (e.g., "10s", "1m", "5m")
	Service        string // Filter by service name
	NegateService  bool   // If true, exclude Service instead of filtering to it
	Resource       string // Filter by resource environment
	NegateResource bool   // If true, exclude Resource instead of filtering to it
}
