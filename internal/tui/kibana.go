// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/elastic/elasticat/internal/config"
)

// buildKibanaDiscoverURL constructs a Kibana Discover URL with the given ES|QL query.
// The URL format opens Kibana Discover in ES|QL mode with the query pre-populated.
// If space is non-empty, the URL will include the space path prefix (e.g., /s/elasticat/app/discover).
// Note: HTTP basic auth in URLs doesn't work with Kibana's session-based login.
func buildKibanaDiscoverURL(kibanaBaseURL, space, esqlQuery string, lookback LookbackDuration) string {
	if kibanaBaseURL == "" {
		kibanaBaseURL = config.DefaultKibanaURL
	}

	// URL-encode the ES|QL query for embedding in the Kibana state
	// Kibana uses Rison format in the URL, but for the esql field it expects URL-encoded query
	encodedQuery := url.QueryEscape(esqlQuery)

	// Convert lookback to Kibana time format (e.g., "now-24h")
	kibanaFrom := lookback.KibanaTimeFrom()

	// Build the app path, including space prefix if specified
	appPath := "/app/discover"
	if space != "" {
		appPath = fmt.Sprintf("/s/%s/app/discover", space)
	}

	// Build the Kibana Discover URL with ES|QL mode
	// Format: /s/{space}/app/discover#/?_g=(time:(...))&_a=(dataSource:(type:esql),query:(esql:'...'))
	//
	// The _g parameter contains global state (time range)
	// The _a parameter contains app state (data source type, query, columns)
	kibanaURL := fmt.Sprintf(
		"%s%s#/?_g=(filters:!(),refreshInterval:(pause:!t,value:60000),time:(from:%s,to:now))&_a=(columns:!('@timestamp'),dataSource:(type:esql),filters:!(),interval:auto,query:(esql:'%s'),sort:!())",
		strings.TrimSuffix(kibanaBaseURL, "/"),
		appPath,
		kibanaFrom,
		encodedQuery,
	)

	return kibanaURL
}

// buildSimpleKibanaURL builds a basic Kibana URL (for the elasticat space).
// Used when opening Kibana without a specific query.
func buildSimpleKibanaURL(kibanaBaseURL, space string) string {
	if kibanaBaseURL == "" {
		kibanaBaseURL = config.DefaultKibanaURL
	}

	// Build the app path, including space prefix if specified
	if space != "" {
		return fmt.Sprintf("%s/s/%s", strings.TrimSuffix(kibanaBaseURL, "/"), space)
	}
	return kibanaBaseURL
}

// openURLInBrowser opens the given URL in the system's default browser.
// Works on macOS, Linux, and Windows.
func openURLInBrowser(rawURL string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", rawURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// KibanaTimeFrom returns the Kibana-formatted time range start for this lookback.
// Kibana uses format like "now-24h", "now-1h", etc.
func (l LookbackDuration) KibanaTimeFrom() string {
	switch l {
	case lookback5m:
		return "now-5m"
	case lookback1h:
		return "now-1h"
	case lookback24h:
		return "now-24h"
	case lookback1w:
		return "now-7d"
	case lookbackAll:
		return "now-30d" // Kibana doesn't have an "all" time, use 30 days
	default:
		return "now-24h"
	}
}

// prepareKibanaURL builds the Kibana Discover URL for the current query and stores it.
// Does not open the browser - call openLastKibanaURL() for that.
func (m *Model) prepareKibanaURL() bool {
	if m.lastQueryJSON == "" {
		m.statusMessage = "No query to open in Kibana"
		m.statusTime = time.Now()
		return false
	}

	m.lastKibanaURL = buildKibanaDiscoverURL(m.kibanaURL, m.kibanaSpace, m.lastQueryJSON, m.lookback)
	return true
}

// openLastKibanaURL opens the lastKibanaURL in the system browser.
func (m *Model) openLastKibanaURL() {
	if m.lastKibanaURL == "" {
		m.statusMessage = "No Kibana URL to open"
		m.statusTime = time.Now()
		return
	}

	if err := openURLInBrowser(m.lastKibanaURL); err != nil {
		m.statusMessage = fmt.Sprintf("Failed to open browser: %v", err)
	} else {
		m.statusMessage = "Opened in Kibana"
	}
	m.statusTime = time.Now()
}

// prepareMetricKibanaURL builds a Kibana URL for a specific metric and stores it.
// For ES|QL-compatible types, it generates a time-series STATS query using DATE_TRUNC
// that Kibana can render as a chart. For counter/histogram types, it falls back to
// a simple document query.
// metricType is the time series type: "gauge", "counter", or "histogram"
// Does not open the browser - call openLastKibanaURL() for that.
func (m *Model) prepareMetricKibanaURL(metricName, metricType string) {
	index := m.client.GetIndex()
	esqlInterval := m.lookback.ToESQLInterval()
	bucketInterval := m.lookback.ToESQLBucketInterval()

	var query string

	// Check if the metric type is ES|QL compatible for aggregations
	// ES|QL cannot aggregate counter types or histogram fields with AVG/MIN/MAX,
	// but it CAN filter on them with IS NOT NULL and count documents
	isESQLCompatible := metricType != "histogram" && metricType != "counter"

	if isESQLCompatible {
		// Generate a time-series STATS query with DATE_TRUNC for Kibana visualization
		// Kibana will render this as a nice time-series chart
		query = fmt.Sprintf(`FROM %s
| WHERE @timestamp >= NOW() - %s
| STATS
    doc_count = COUNT(*),
    avg_val = AVG(`+"`%s`"+`)
  BY bucket = DATE_TRUNC(%s, @timestamp)
| SORT bucket`,
			index, esqlInterval, metricName, bucketInterval)
	} else {
		// For counter/histogram types, ES|QL can filter with IS NOT NULL but
		// cannot aggregate the values. Show document counts over time instead.
		query = fmt.Sprintf(`FROM %s
| WHERE @timestamp >= NOW() - %s AND `+"`%s`"+` IS NOT NULL
| STATS
    doc_count = COUNT(*)
  BY bucket = DATE_TRUNC(%s, @timestamp)
| SORT bucket`,
			index, esqlInterval, metricName, bucketInterval)
	}

	m.lastKibanaURL = buildKibanaDiscoverURL(m.kibanaURL, m.kibanaSpace, query, m.lookback)
}

// prepareTraceKibanaURL builds a Kibana URL for a specific trace ID and stores it.
// Shows all spans/events for the trace within the current lookback period.
// Does not open the browser - call openLastKibanaURL() for that.
func (m *Model) prepareTraceKibanaURL(traceID string) bool {
	if traceID == "" {
		m.statusMessage = "No trace ID to open in Kibana"
		m.statusTime = time.Now()
		return false
	}

	index := m.client.GetIndex()
	esqlInterval := m.lookback.ToESQLInterval()

	// Query with both trace.id and trace_id field variants for compatibility
	query := fmt.Sprintf(`FROM %s
| WHERE @timestamp >= NOW() - %s AND (trace.id == "%s" OR trace_id == "%s")
| SORT @timestamp ASC
| LIMIT 1000`,
		index, esqlInterval, traceID, traceID)

	m.lastKibanaURL = buildKibanaDiscoverURL(m.kibanaURL, m.kibanaSpace, query, m.lookback)
	return true
}

// ToESQLInterval returns the ES|QL time interval string for the lookback duration.
// ES|QL uses format like "24 hours", "1 hour", "5 minutes"
func (l LookbackDuration) ToESQLInterval() string {
	switch l {
	case lookback5m:
		return "5 minutes"
	case lookback1h:
		return "1 hour"
	case lookback24h:
		return "24 hours"
	case lookback1w:
		return "7 days"
	case lookbackAll:
		return "30 days"
	default:
		return "24 hours"
	}
}

// ToESQLBucketInterval returns the appropriate DATE_TRUNC bucket interval
// for time-series visualization based on the lookback duration.
// Aims for roughly 20-60 buckets for good chart visualization.
func (l LookbackDuration) ToESQLBucketInterval() string {
	switch l {
	case lookback5m:
		return "10 seconds" // 5min / 10s = 30 buckets
	case lookback1h:
		return "1 minute" // 1h / 1min = 60 buckets
	case lookback24h:
		return "30 minutes" // 24h / 30min = 48 buckets
	case lookback1w:
		return "6 hours" // 7d / 6h = 28 buckets
	case lookbackAll:
		return "1 day" // 30d / 1d = 30 buckets
	default:
		return "30 minutes"
	}
}
