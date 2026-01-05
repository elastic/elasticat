// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"io"
)

// FieldCapsResponse represents the relevant parts of a field_caps API response
type FieldCapsResponse struct {
	Fields map[string]map[string]FieldCapsInfo
}

// FieldCapsInfo contains field capability information
type FieldCapsInfo struct {
	Type             string
	Aggregatable     bool
	TimeSeriesMetric string
}

// SearchResponse represents a raw search response body
type SearchResponse struct {
	Body       io.ReadCloser
	StatusCode int
	Status     string
	IsError    bool
}

// Executor defines the Elasticsearch operations needed for metrics
type Executor interface {
	// FieldCaps returns field capabilities for the given index pattern and fields filter
	FieldCaps(ctx context.Context, index, fields string) (*FieldCapsResponse, error)

	// SearchForMetrics executes a search query and returns the raw response
	SearchForMetrics(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error)

	// GetIndex returns the current index pattern
	GetIndex() string
}

