// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package perspectives

import (
	"context"
	"fmt"

	"github.com/elastic/elasticat/internal/es/traces"
	"github.com/elastic/elasticat/internal/index"
)

// GetByField aggregates counts of logs, traces, and metrics for a given field
func GetByField(ctx context.Context, exec Executor, lookback string, field string) ([]PerspectiveAgg, error) {
	lookbackInterval := traces.LookbackToESQLInterval(lookback)

	query := fmt.Sprintf(`FROM %s
| WHERE @timestamp >= NOW() - %s
| STATS
    logs = COUNT(CASE(processor.event != "transaction" AND processor.event != "span", 1, null)),
    traces = COUNT(CASE(processor.event == "transaction", 1, null)),
    metrics = COUNT(CASE(metrics IS NOT NULL, 1, null))
  BY %s
| SORT logs DESC
| LIMIT 100`, index.All, lookbackInterval, field)

	res, err := exec.ExecuteESQLQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ES|QL perspective query failed: %w", err)
	}

	colIndex := map[string]int{}
	for i, col := range res.Columns {
		colIndex[col.Name] = i
	}

	// Expected columns: logs, traces, metrics, <field>
	getFloat := func(row []interface{}, name string) int64 {
		idx, ok := colIndex[name]
		if !ok || idx >= len(row) {
			return 0
		}
		if v, ok := row[idx].(float64); ok {
			return int64(v)
		}
		return 0
	}

	getString := func(row []interface{}, name string) string {
		idx, ok := colIndex[name]
		if !ok || idx >= len(row) {
			return ""
		}
		if v, ok := row[idx].(string); ok {
			return v
		}
		return ""
	}

	perspectiveList := []PerspectiveAgg{}
	for _, row := range res.Values {
		name := getString(row, field)
		if name == "" {
			continue
		}
		perspectiveList = append(perspectiveList, PerspectiveAgg{
			Name:        name,
			LogCount:    getFloat(row, "logs"),
			TraceCount:  getFloat(row, "traces"),
			MetricCount: getFloat(row, "metrics"),
		})
	}

	return perspectiveList, nil
}

// GetServices returns aggregated counts per service
func GetServices(ctx context.Context, exec Executor, lookback string) ([]PerspectiveAgg, error) {
	return GetByField(ctx, exec, lookback, "service.name")
}

// GetResources returns aggregated counts per resource environment
func GetResources(ctx context.Context, exec Executor, lookback string) ([]PerspectiveAgg, error) {
	return GetByField(ctx, exec, lookback, "resource.attributes.deployment.environment")
}
