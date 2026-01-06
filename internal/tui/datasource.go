// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"

	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/es/metrics"
	"github.com/elastic/elasticat/internal/es/perspectives"
	"github.com/elastic/elasticat/internal/es/traces"
)

// DataSource defines the data operations required by the TUI.
// This interface decouples the TUI from the concrete es.Client,
// enabling easier testing with mock implementations.
type DataSource interface {
	// Tail retrieves the most recent documents using Query DSL.
	// Used for histogram metrics where ES|QL doesn't support the field type.
	Tail(ctx context.Context, opts es.TailOptions) (*es.SearchResult, error)

	// TailESQL retrieves the most recent documents via ES|QL.
	// Returns the search result plus the rendered ES|QL query string for display.
	TailESQL(ctx context.Context, opts es.TailOptions) (*es.SearchResult, string, error)

	// SearchESQL performs a text search using ES|QL.
	// Returns the search result plus the ES|QL query string for display.
	SearchESQL(ctx context.Context, queryStr string, opts es.SearchOptions) (*es.SearchResult, string, error)

	// CountESQL returns only the document count for the provided TailOptions.
	// Used by auto lookback detection to avoid full fetches.
	CountESQL(ctx context.Context, opts es.TailOptions) (int64, string, error)

	// AggregateMetrics retrieves aggregated statistics for all discovered metrics.
	AggregateMetrics(ctx context.Context, opts metrics.AggregateMetricsOptions) (*metrics.MetricsAggResult, error)

	// GetTransactionNamesESQL retrieves transaction aggregations using ES|QL.
	GetTransactionNamesESQL(ctx context.Context, lookback, service, resource string, negateService, negateResource bool) ([]traces.TransactionNameAgg, error)

	// GetServices returns aggregated counts per service.
	GetServices(ctx context.Context, lookback string) ([]perspectives.PerspectiveAgg, error)

	// GetResources returns aggregated counts per resource environment.
	GetResources(ctx context.Context, lookback string) ([]perspectives.PerspectiveAgg, error)

	// GetFieldCaps retrieves available fields from the index.
	GetFieldCaps(ctx context.Context) ([]es.FieldInfo, error)

	// Ping checks if Elasticsearch is reachable.
	Ping(ctx context.Context) error

	// GetIndex returns the current index pattern.
	GetIndex() string

	// SetIndex changes the index pattern.
	SetIndex(index string)
}

// Compile-time check that es.Client implements DataSource.
var _ DataSource = (*es.Client)(nil)
