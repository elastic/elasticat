// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package perspectives

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/elasticat/internal/es/shared"
	"github.com/elastic/elasticat/internal/es/traces"
	"github.com/elastic/elasticat/internal/index"
)

// GetByField aggregates counts of logs, traces, and metrics for a given field
func GetByField(ctx context.Context, exec Executor, lookback string, field string) ([]PerspectiveAgg, error) {
	lookbackInterval := traces.LookbackToESQLInterval(lookback)

	// Use data_stream.type to distinguish signal types:
	// - "logs" for log documents
	// - "traces" for trace documents (transactions and spans)
	// - "metrics" for metric documents
	//
	// ES|QL returns a 400 verification_exception when any FROM pattern matches no indices.
	// For mixed-signal queries (logs-*,traces-*,metrics-*), drop missing patterns and retry;
	// if none remain, treat as empty state.
	from := index.All
	buildQuery := func(fromPattern string) string {
		return fmt.Sprintf(`FROM %s
| WHERE @timestamp >= NOW() - %s
| STATS
    logs = COUNT(CASE(data_stream.type == "logs", 1, null)),
    traces = COUNT(CASE(data_stream.type == "traces", 1, null)),
    metrics = COUNT(CASE(data_stream.type == "metrics", 1, null))
  BY %s
| SORT logs DESC
| LIMIT 100`, fromPattern, lookbackInterval, field)
	}

	query := buildQuery(from)
	var res *shared.ESQLResult
	for {
		var err error
		res, err = exec.ExecuteESQLQuery(ctx, query)
		if err != nil {
			// Unknown index: remove that pattern and retry with remaining indices
			if missing, ok := shared.IsESQLUnknownIndex(err); ok {
				from = removeIndexPattern(from, missing)
				if from == "" {
					return []PerspectiveAgg{}, nil
				}
				query = buildQuery(from)
				continue
			}
			// Other empty-state errors (e.g., unsupported field types): return empty
			if shared.IsESQLEmptyStateError(err) {
				return []PerspectiveAgg{}, nil
			}
			return nil, fmt.Errorf("ES|QL perspective query failed: %w", err)
		}
		break
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

func removeIndexPattern(from string, missing string) string {
	parts := strings.Split(from, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == missing {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ",")
}

// GetServices returns aggregated counts per service
func GetServices(ctx context.Context, exec Executor, lookback string) ([]PerspectiveAgg, error) {
	return GetByField(ctx, exec, lookback, "service.name")
}

// GetResources returns aggregated counts per resource environment
func GetResources(ctx context.Context, exec Executor, lookback string) ([]PerspectiveAgg, error) {
	return GetByField(ctx, exec, lookback, "resource.attributes.deployment.environment")
}
