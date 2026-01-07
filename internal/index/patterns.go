// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

// Package index provides constants for Elasticsearch index patterns
package index

// Index patterns for OTel data streams
const (
	Logs    = "logs-*"
	Traces  = "traces-*"
	Metrics = "metrics-*"

	// All is the combined pattern for querying all signal types
	All = Logs + "," + Traces + "," + Metrics
)
