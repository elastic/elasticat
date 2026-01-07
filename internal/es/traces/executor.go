// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"

	"github.com/elastic/elasticat/internal/es/shared"
)

// Executor defines the Elasticsearch operations needed for traces
type Executor interface {
	// Embed ESQLExecutor for common ES|QL query execution
	shared.ESQLExecutor

	// SearchForTraces executes a search query and returns the raw response
	SearchForTraces(ctx context.Context, index string, body []byte, size int) (*shared.SearchResponse, error)

	// GetIndex returns the current index pattern
	GetIndex() string
}
