// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package perspectives

import (
	"context"

	"github.com/elastic/elasticat/internal/es/shared"
)

// Executor defines the Elasticsearch operations needed for perspectives
type Executor interface {
	// Embed ESQLExecutor for common ES|QL query execution
	shared.ESQLExecutor

	// SearchForPerspectives executes a search query and returns the raw response
	SearchForPerspectives(ctx context.Context, index string, body []byte, size int) (*shared.SearchResponse, error)
}
