// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package shared

import "strings"

// LookbackToESQLInterval converts a lookback string (e.g., "now-5m") to ES|QL format (e.g., "5 minutes").
// ES|QL requires full unit names, not abbreviations.
func LookbackToESQLInterval(lookback string) string {
	switch lookback {
	case "now-5m":
		return "5 minutes"
	case "now-10m":
		return "10 minutes"
	case "now-15m":
		return "15 minutes"
	case "now-30m":
		return "30 minutes"
	case "now-1h":
		return "1 hour"
	case "now-3h":
		return "3 hours"
	case "now-6h":
		return "6 hours"
	case "now-12h":
		return "12 hours"
	case "now-24h":
		return "24 hours"
	case "now-1d":
		return "24 hours"
	case "now-1w":
		return "7 days"
	default:
		return "24 hours" // Default
	}
}

// EscapeESQLString escapes special characters in a string for use in ES|QL queries.
// Currently escapes double quotes which need to be escaped in ES|QL string literals.
func EscapeESQLString(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}
