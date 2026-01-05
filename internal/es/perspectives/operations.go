// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package perspectives

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/elastic/elasticat/internal/index"
)

// GetByField aggregates counts of logs, traces, and metrics for a given field
func GetByField(ctx context.Context, exec Executor, lookback string, field string) ([]PerspectiveAgg, error) {
	// Query across all indices to get logs, traces, and metrics counts
	indexPattern := index.All

	// Build time range filter
	timeFilter := map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"gte": lookback, // lookback already includes "now-" prefix from ESRange()
			},
		},
	}

	// Build aggregation
	agg := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []interface{}{timeFilter},
			},
		},
		"aggs": map[string]interface{}{
			"items": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": field,
					"size":  100, // Top 100 services/resources
				},
				"aggs": map[string]interface{}{
					// Count logs (excluding transactions and spans)
					"logs": map[string]interface{}{
						"filter": map[string]interface{}{
							"bool": map[string]interface{}{
								"must_not": []interface{}{
									map[string]interface{}{
										"term": map[string]interface{}{
											"processor.event": "transaction",
										},
									},
									map[string]interface{}{
										"term": map[string]interface{}{
											"processor.event": "span",
										},
									},
								},
							},
						},
					},
					// Count traces (transactions only)
					"traces": map[string]interface{}{
						"filter": map[string]interface{}{
							"term": map[string]interface{}{
								"processor.event": "transaction",
							},
						},
					},
					// Count metrics (documents with metrics fields)
					"metrics": map[string]interface{}{
						"filter": map[string]interface{}{
							"exists": map[string]interface{}{
								"field": "metrics",
							},
						},
					},
				},
			},
		},
	}

	// Execute search
	queryJSON, err := json.Marshal(agg)
	if err != nil {
		return nil, fmt.Errorf("encode query: %w", err)
	}

	res, err := exec.SearchForPerspectives(ctx, indexPattern, queryJSON, 0)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError {
		body, _ := io.ReadAll(res.Body)
		// Pretty-print the query for error messages
		var prettyQuery bytes.Buffer
		err = json.Indent(&prettyQuery, queryJSON, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("indent query: %w", err)
		}
		return nil, fmt.Errorf("search error: %s\nError: %s\n\nQuery:\n%s", res.Status, string(body), prettyQuery.String())
	}

	// Parse response
	var result struct {
		Aggregations struct {
			Items struct {
				Buckets []map[string]interface{} `json:"buckets"`
			} `json:"items"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract perspective data
	perspectiveList := []PerspectiveAgg{}
	for _, bucket := range result.Aggregations.Items.Buckets {
		name, ok := bucket["key"].(string)
		if !ok || name == "" {
			continue
		}

		agg := PerspectiveAgg{Name: name}

		// Extract log count
		if logs, ok := bucket["logs"].(map[string]interface{}); ok {
			if count, ok := logs["doc_count"].(float64); ok {
				agg.LogCount = int64(count)
			}
		}

		// Extract trace count
		if traces, ok := bucket["traces"].(map[string]interface{}); ok {
			if count, ok := traces["doc_count"].(float64); ok {
				agg.TraceCount = int64(count)
			}
		}

		// Extract metric count
		if metrics, ok := bucket["metrics"].(map[string]interface{}); ok {
			if count, ok := metrics["doc_count"].(float64); ok {
				agg.MetricCount = int64(count)
			}
		}

		perspectiveList = append(perspectiveList, agg)
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
