// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"github.com/elastic/elasticat/internal/es/shared"
)

// Executor defines the Elasticsearch operations needed for metrics
type Executor interface {
	// Embed ESQLExecutor for common ES|QL query execution
	shared.ESQLExecutor

	// FieldCaps returns field capabilities for the given index pattern and fields filter
	FieldCaps(ctx context.Context, index, fields string) (*shared.FieldCapsResponse, error)

	// SearchForMetrics executes a search query and returns the raw response
	SearchForMetrics(ctx context.Context, index string, body []byte, size int) (*shared.SearchResponse, error)

	// GetIndex returns the current index pattern
	GetIndex() string
}
