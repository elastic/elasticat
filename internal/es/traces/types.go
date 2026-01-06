// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package traces

import "time"

// TransactionNameAgg represents aggregated statistics for a transaction name
type TransactionNameAgg struct {
	Name        string    // Transaction name (e.g., "GET /api/users")
	Count       int64     // Number of transactions
	AvgDuration float64   // Average duration in milliseconds
	MinDuration float64   // Minimum duration in milliseconds
	MaxDuration float64   // Maximum duration in milliseconds
	TraceCount  int64     // Number of unique traces
	AvgSpans    float64   // Average number of spans per trace
	ErrorRate   float64   // Percentage of errors (0-100)
	LastSeen    time.Time // Timestamp of the most recent transaction
}

// Note: ESQLResult and ESQLColumn have been moved to es/types.go
// as they are shared across traces, metrics, and perspectives packages.
