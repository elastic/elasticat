// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"io"
)

// SearchResponse represents a raw search response body
type SearchResponse struct {
	Body       io.ReadCloser
	StatusCode int
	Status     string
	IsError    bool
}

// Executor defines the Elasticsearch operations needed for traces
type Executor interface {
	// SearchForTraces executes a search query and returns the raw response
	SearchForTraces(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error)

	// ExecuteESQLQuery executes an ES|QL query and returns the result
	ExecuteESQLQuery(ctx context.Context, query string) (*ESQLResult, error)

	// GetIndex returns the current index pattern
	GetIndex() string
}
