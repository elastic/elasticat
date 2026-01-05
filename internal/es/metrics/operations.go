// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// GetFieldNames discovers metric field names from field_caps API
// Returns only aggregatable numeric fields under the "metrics.*" namespace
func GetFieldNames(ctx context.Context, exec Executor, index string) ([]MetricFieldInfo, error) {
	res, err := exec.FieldCaps(ctx, index, "metrics.*")
	if err != nil {
		return nil, fmt.Errorf("failed to get metric field caps: %w", err)
	}

	var metricFields []MetricFieldInfo
	for name, typeMap := range res.Fields {
		for _, info := range typeMap {
			// Skip object types (not actual metric values)
			if info.Type == "object" {
				continue
			}
			// Only include aggregatable numeric types
			if !info.Aggregatable {
				continue
			}
			// Filter to long, double, float, histogram, aggregate_metric_double types
			switch info.Type {
			case "long", "double", "float", "half_float", "scaled_float", "histogram", "aggregate_metric_double":
				shortName := name
				if strings.HasPrefix(name, "metrics.") {
					shortName = name[8:] // Remove "metrics." prefix
				}
				metricFields = append(metricFields, MetricFieldInfo{
					Name:           name,
					ShortName:      shortName,
					Type:           info.Type,
					TimeSeriesType: info.TimeSeriesMetric,
				})
			}
			break // Only process first type
		}
	}

	// Sort by name for consistent display
	SortFields(metricFields)

	return metricFields, nil
}

// SortFields sorts metric fields by short name
func SortFields(fields []MetricFieldInfo) {
	for i := 0; i < len(fields)-1; i++ {
		for j := i + 1; j < len(fields); j++ {
			if fields[i].ShortName > fields[j].ShortName {
				fields[i], fields[j] = fields[j], fields[i]
			}
		}
	}
}

// Aggregate retrieves aggregated statistics for all discovered metrics
func Aggregate(ctx context.Context, exec Executor, opts AggregateMetricsOptions) (*MetricsAggResult, error) {
	index := exec.GetIndex()

	// Discover metrics
	metricFields, err := GetFieldNames(ctx, exec, index)
	if err != nil {
		return nil, err
	}

	if len(metricFields) == 0 {
		return &MetricsAggResult{Metrics: []AggregatedMetric{}, BucketSize: opts.BucketSize}, nil
	}

	// Limit to 50 metrics to avoid huge queries
	maxMetrics := 50
	if len(metricFields) > maxMetrics {
		metricFields = metricFields[:maxMetrics]
	}

	// Build aggregation query
	aggs := make(map[string]interface{})
	for i, mf := range metricFields {
		aggName := fmt.Sprintf("m%d", i)
		aggs[aggName] = map[string]interface{}{
			"filter": map[string]interface{}{
				"exists": map[string]interface{}{
					"field": mf.Name,
				},
			},
			"aggs": map[string]interface{}{
				"stats": map[string]interface{}{
					"extended_stats": map[string]interface{}{
						"field": mf.Name,
					},
				},
				"over_time": map[string]interface{}{
					"date_histogram": map[string]interface{}{
						"field":          "@timestamp",
						"fixed_interval": opts.BucketSize,
					},
					"aggs": map[string]interface{}{
						"value": map[string]interface{}{
							"avg": map[string]interface{}{
								"field": mf.Name,
							},
						},
					},
				},
				"latest": map[string]interface{}{
					"top_hits": map[string]interface{}{
						"size": 1,
						"sort": []map[string]interface{}{
							{"@timestamp": "desc"},
						},
						"_source": []string{mf.Name},
					},
				},
			},
		}
	}

	query := map[string]interface{}{
		"size": 0,
		"aggs": aggs,
	}

	// Build filters array
	var filters []interface{}
	var mustNot []interface{}

	// Add time range filter if specified
	if opts.Lookback != "" {
		filters = append(filters, map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": opts.Lookback,
				},
			},
		})
	}

	// Add service filter if specified
	if opts.Service != "" {
		serviceClause := map[string]interface{}{
			"term": map[string]interface{}{
				"service.name": opts.Service,
			},
		}
		if opts.NegateService {
			mustNot = append(mustNot, serviceClause)
		} else {
			filters = append(filters, serviceClause)
		}
	}

	// Add resource filter if specified
	if opts.Resource != "" {
		resourceClause := map[string]interface{}{
			"term": map[string]interface{}{
				"resource.attributes.deployment.environment": opts.Resource,
			},
		}
		if opts.NegateResource {
			mustNot = append(mustNot, resourceClause)
		} else {
			filters = append(filters, resourceClause)
		}
	}

	// Add filters to query if any exist
	if len(filters) > 0 || len(mustNot) > 0 {
		boolQuery := map[string]interface{}{}
		if len(filters) > 0 {
			boolQuery["filter"] = filters
		}
		if len(mustNot) > 0 {
			boolQuery["must_not"] = mustNot
		}
		query["query"] = map[string]interface{}{
			"bool": boolQuery,
		}
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal aggregation query: %w", err)
	}

	res, err := exec.SearchForMetrics(ctx, index, queryJSON, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer res.Body.Close()

	if res.IsError {
		body, _ := io.ReadAll(res.Body)
		// Pretty-print the query for error messages
		var prettyQuery bytes.Buffer
		_ = json.Indent(&prettyQuery, queryJSON, "", "  ")
		return nil, fmt.Errorf("aggregation failed: %s\nError: %s\n\nQuery:\n%s", res.Status, string(body), prettyQuery.String())
	}

	return parseAggResponse(res.Body, metricFields, opts.BucketSize)
}

func parseAggResponse(body io.Reader, fields []MetricFieldInfo, bucketSize string) (*MetricsAggResult, error) {
	// Parse the complex nested aggregation response
	var raw map[string]interface{}
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	aggs, ok := raw["aggregations"].(map[string]interface{})
	if !ok {
		return &MetricsAggResult{Metrics: []AggregatedMetric{}, BucketSize: bucketSize}, nil
	}

	result := &MetricsAggResult{
		Metrics:    make([]AggregatedMetric, 0, len(fields)),
		BucketSize: bucketSize,
	}

	for i, mf := range fields {
		aggName := fmt.Sprintf("m%d", i)
		metricAgg, ok := aggs[aggName].(map[string]interface{})
		if !ok {
			continue
		}

		am := AggregatedMetric{
			Name:      mf.Name,
			ShortName: mf.ShortName,
			Type:      mf.TimeSeriesType,
		}

		// Extract stats
		if stats, ok := metricAgg["stats"].(map[string]interface{}); ok {
			if min, ok := stats["min"].(float64); ok {
				am.Min = min
			}
			if max, ok := stats["max"].(float64); ok {
				am.Max = max
			}
			if avg, ok := stats["avg"].(float64); ok {
				am.Avg = avg
			}
		}

		// Extract time series buckets
		if overTime, ok := metricAgg["over_time"].(map[string]interface{}); ok {
			if buckets, ok := overTime["buckets"].([]interface{}); ok {
				am.Buckets = make([]MetricBucket, 0, len(buckets))
				for _, b := range buckets {
					bucket, ok := b.(map[string]interface{})
					if !ok {
						continue
					}
					mb := MetricBucket{}
					if keyMs, ok := bucket["key"].(float64); ok {
						mb.Timestamp = time.UnixMilli(int64(keyMs))
					}
					if count, ok := bucket["doc_count"].(float64); ok {
						mb.Count = int64(count)
					}
					if value, ok := bucket["value"].(map[string]interface{}); ok {
						if v, ok := value["value"].(float64); ok {
							mb.Value = v
						}
					}
					am.Buckets = append(am.Buckets, mb)
				}
			}
		}

		// Extract latest value from top_hits
		if latest, ok := metricAgg["latest"].(map[string]interface{}); ok {
			if hits, ok := latest["hits"].(map[string]interface{}); ok {
				if hitsList, ok := hits["hits"].([]interface{}); ok && len(hitsList) > 0 {
					if hit, ok := hitsList[0].(map[string]interface{}); ok {
						if source, ok := hit["_source"].(map[string]interface{}); ok {
							am.Latest = extractNestedFloat(source, mf.Name)
						}
					}
				}
			}
		}

		result.Metrics = append(result.Metrics, am)
	}

	return result, nil
}

// extractNestedFloat extracts a float64 from a nested map using dot notation
func extractNestedFloat(data map[string]interface{}, path string) float64 {
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return 0
		}
	}

	if f, ok := current.(float64); ok {
		return f
	}
	return 0
}
