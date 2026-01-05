// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package perspectives

import (
	"context"
	"io"

	"github.com/elastic/elasticat/internal/es/traces"
)

// SearchResponse represents a raw search response body
type SearchResponse struct {
	Body       io.ReadCloser
	StatusCode int
	Status     string
	IsError    bool
}

// Executor defines the Elasticsearch operations needed for perspectives
type Executor interface {
	// SearchForPerspectives executes a search query and returns the raw response
	SearchForPerspectives(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error)

	// ExecuteESQLQuery executes an ES|QL query and returns the structured result
	ExecuteESQLQuery(ctx context.Context, query string) (*traces.ESQLResult, error)
}
