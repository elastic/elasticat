// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/elasticat/internal/es/errfmt"
	"github.com/elastic/elasticat/internal/es/shared"
)

// LookbackToESQLInterval converts a lookback string (e.g., "now-5m") to ES|QL format (e.g., "5 minutes").
// ES|QL requires full unit names, not abbreviations.
// Deprecated: Use shared.LookbackToESQLInterval instead. This wrapper is kept for backward compatibility.
func LookbackToESQLInterval(lookback string) string {
	return shared.LookbackToESQLInterval(lookback)
}

// GetNames returns aggregated transaction names with statistics using QueryDSL
func GetNames(ctx context.Context, exec Executor, lookback, service, resource string) ([]TransactionNameAgg, error) {
	index := exec.GetIndex()

	// Build base filters (WITHOUT processor.event filter - we'll add that in aggregations)
	var filters []map[string]interface{}

	// Add time range filter if specified
	if lookback != "" {
		filters = append(filters, map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": lookback,
				},
			},
		})
	}

	// Add service filter if specified
	if service != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"service.name": service,
			},
		})
	}

	// Add resource filter if specified
	if resource != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"resource.attributes.deployment.environment": resource,
			},
		})
	}

	// Build the aggregation query (query includes both transactions and spans)
	query := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": filters,
			},
		},
		"aggs": map[string]interface{}{
			// Count total spans globally
			"total_spans": map[string]interface{}{
				"filter": map[string]interface{}{
					"term": map[string]interface{}{
						"attributes.processor.event": "span",
					},
				},
			},
			// Count total unique traces globally
			"total_unique_traces": map[string]interface{}{
				"cardinality": map[string]interface{}{
					"field": "trace.id",
				},
			},
			// Filter to only transactions for the terms aggregation (since "name" field is on transactions)
			"transactions": map[string]interface{}{
				"filter": map[string]interface{}{
					"term": map[string]interface{}{
						"attributes.processor.event": "transaction",
					},
				},
				"aggs": map[string]interface{}{
					"tx_names": map[string]interface{}{
						"terms": map[string]interface{}{
							"field": "name",
							"size":  100,
							"order": map[string]interface{}{
								"_count": "desc",
							},
						},
						"aggs": map[string]interface{}{
							"avg_duration": map[string]interface{}{
								"avg": map[string]interface{}{
									"field": "duration",
								},
							},
							"min_duration": map[string]interface{}{
								"min": map[string]interface{}{
									"field": "duration",
								},
							},
							"max_duration": map[string]interface{}{
								"max": map[string]interface{}{
									"field": "duration",
								},
							},
							"last_seen": map[string]interface{}{
								"max": map[string]interface{}{
									"field": "@timestamp",
								},
							},
							"unique_traces": map[string]interface{}{
								"cardinality": map[string]interface{}{
									"field": "trace.id",
								},
							},
							"errors": map[string]interface{}{
								"filter": map[string]interface{}{
									"bool": map[string]interface{}{
										"should": []map[string]interface{}{
											{"term": map[string]interface{}{"status.code": "Error"}},
											{"term": map[string]interface{}{"status.code": "STATUS_CODE_ERROR"}},
											{"range": map[string]interface{}{"status.code": map[string]interface{}{"gte": 2}}},
										},
										"minimum_should_match": 1,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := exec.SearchForTraces(ctx, index, queryJSON, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer res.Body.Close()

	if res.IsError {
		body, _ := io.ReadAll(res.Body)
		return nil, errfmt.FormatQueryError(res.Status, body, queryJSON)
	}

	// Parse response
	var raw map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	aggs, ok := raw["aggregations"].(map[string]interface{})
	if !ok {
		return []TransactionNameAgg{}, nil
	}

	// Extract global span count and unique traces to calculate avgSpans
	var globalAvgSpans float64
	if totalSpansAgg, ok := aggs["total_spans"].(map[string]interface{}); ok {
		if spanCount, ok := totalSpansAgg["doc_count"].(float64); ok {
			if totalTracesAgg, ok := aggs["total_unique_traces"].(map[string]interface{}); ok {
				if traceCount, ok := totalTracesAgg["value"].(float64); ok && traceCount > 0 {
					globalAvgSpans = spanCount / traceCount
				}
			}
		}
	}

	// Navigate through the "transactions" filter aggregation
	transactions, ok := aggs["transactions"].(map[string]interface{})
	if !ok {
		return []TransactionNameAgg{}, nil
	}

	txNames, ok := transactions["tx_names"].(map[string]interface{})
	if !ok {
		return []TransactionNameAgg{}, nil
	}

	buckets, ok := txNames["buckets"].([]interface{})
	if !ok {
		return []TransactionNameAgg{}, nil
	}

	result := make([]TransactionNameAgg, 0, len(buckets))
	for _, b := range buckets {
		bucket, ok := b.(map[string]interface{})
		if !ok {
			continue
		}

		agg := TransactionNameAgg{}

		if name, ok := bucket["key"].(string); ok {
			agg.Name = name
		}
		if count, ok := bucket["doc_count"].(float64); ok {
			agg.Count = int64(count)
		}

		// Extract average duration (convert from nanoseconds to milliseconds)
		if avgDur, ok := bucket["avg_duration"].(map[string]interface{}); ok {
			if v, ok := avgDur["value"].(float64); ok {
				agg.AvgDuration = v / 1_000_000 // nano to ms
			}
		}

		// Extract min duration (convert from nanoseconds to milliseconds)
		if minDur, ok := bucket["min_duration"].(map[string]interface{}); ok {
			if v, ok := minDur["value"].(float64); ok {
				agg.MinDuration = v / 1_000_000 // nano to ms
			}
		}

		// Extract max duration (convert from nanoseconds to milliseconds)
		if maxDur, ok := bucket["max_duration"].(map[string]interface{}); ok {
			if v, ok := maxDur["value"].(float64); ok {
				agg.MaxDuration = v / 1_000_000 // nano to ms
			}
		}

		// Extract unique trace count
		if uniqueTraces, ok := bucket["unique_traces"].(map[string]interface{}); ok {
			if v, ok := uniqueTraces["value"].(float64); ok {
				agg.TraceCount = int64(v)
			}
		}

		// Extract last seen timestamp
		if lastSeen, ok := bucket["last_seen"].(map[string]interface{}); ok {
			if v, ok := lastSeen["value"].(float64); ok {
				agg.LastSeen = time.UnixMilli(int64(v))
			}
		}

		// Use global average spans per trace
		agg.AvgSpans = globalAvgSpans

		// Calculate error rate
		if errors, ok := bucket["errors"].(map[string]interface{}); ok {
			if errorCount, ok := errors["doc_count"].(float64); ok {
				if agg.Count > 0 {
					agg.ErrorRate = (errorCount / float64(agg.Count)) * 100
				}
			}
		}

		result = append(result, agg)
	}

	return result, nil
}

// GetNamesESSQL retrieves transaction aggregations using ES|QL
// This uses a 3-query approach with client-side correlation to calculate accurate span counts per transaction name
func GetNamesESSQL(ctx context.Context, exec Executor, lookback, service, resource string, negateService, negateResource bool) (*TransactionNamesResult, error) {
	index := exec.GetIndex()

	// Build filter clauses for queries
	// ES|QL requires full unit names (e.g., "5 minutes" not "5m")
	timeFilter := fmt.Sprintf("@timestamp >= NOW() - %s", LookbackToESQLInterval(lookback))

	serviceFilter := ""
	if service != "" {
		op := "=="
		if negateService {
			op = "!="
		}
		serviceFilter = fmt.Sprintf("AND service.name %s \"%s\"", op, service)
	}

	resourceFilter := ""
	if resource != "" {
		op := "=="
		if negateResource {
			op = "!="
		}
		resourceFilter = fmt.Sprintf("AND resource.attributes.deployment.environment %s \"%s\"", op, resource)
	}

	// Query 1: Get transaction stats per transaction name
	q1 := fmt.Sprintf(`FROM %s
| WHERE processor.event == "transaction"
  AND %s
  %s
  %s
| STATS
    tx_count = COUNT(*),
    unique_traces = COUNT_DISTINCT(trace.id),
    min_duration = MIN(transaction.duration.us),
    avg_duration = AVG(transaction.duration.us),
    max_duration = MAX(transaction.duration.us),
    error_count = COUNT(CASE(event.outcome == "failure", 1, null)),
    last_seen = MAX(@timestamp)
  BY transaction.name
| EVAL error_rate = error_count / tx_count * 100
| SORT tx_count DESC
| LIMIT 100`,
		index, timeFilter, serviceFilter, resourceFilter)

	statsResult, err := exec.ExecuteESQLQuery(ctx, q1)
	if err != nil {
		// Treat expected empty-state errors (no matching indices, unsupported field types)
		// as empty results rather than surfacing errors to the UI.
		if shared.IsESQLEmptyStateError(err) {
			return &TransactionNamesResult{Names: []TransactionNameAgg{}, Query: q1}, nil
		}
		return nil, fmt.Errorf("failed to execute transaction stats query: %w", err)
	}

	// Parse transaction stats
	txStats := make([]TransactionNameAgg, 0, len(statsResult.Values))
	txNameToIndex := make(map[string]int) // Map tx name -> index in txStats slice

	for _, row := range statsResult.Values {
		if len(row) < 9 {
			continue // Skip malformed rows
		}

		agg := TransactionNameAgg{}

		// Parse columns (order matches STATS ... BY clause above)
		if v, ok := row[0].(float64); ok {
			agg.Count = int64(v)
		}
		if v, ok := row[1].(float64); ok {
			agg.TraceCount = int64(v)
		}
		if v, ok := row[2].(float64); ok {
			agg.MinDuration = v / 1_000_000 // nano to ms
		}
		if v, ok := row[3].(float64); ok {
			agg.AvgDuration = v / 1_000_000 // nano to ms
		}
		if v, ok := row[4].(float64); ok {
			agg.MaxDuration = v / 1_000_000 // nano to ms
		}
		// Skip row[5] - error_count (we use error_rate instead)
		// row[6] - last_seen timestamp
		if ts, ok := row[6].(string); ok {
			if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				agg.LastSeen = parsed
			}
		}
		if v, ok := row[7].(string); ok {
			agg.Name = v
		}
		if v, ok := row[8].(float64); ok {
			agg.ErrorRate = v
		}

		txNameToIndex[agg.Name] = len(txStats)
		txStats = append(txStats, agg)
	}

	// Query 2: Get trace.id -> transaction.name mapping
	q2 := fmt.Sprintf(`FROM %s
| WHERE processor.event == "transaction"
  AND %s
  %s
  %s
| KEEP transaction.name, trace.id
| LIMIT 100000`,
		index, timeFilter, serviceFilter, resourceFilter)

	mappingResult, err := exec.ExecuteESQLQuery(ctx, q2)
	if err != nil {
		if shared.IsESQLEmptyStateError(err) {
			return &TransactionNamesResult{Names: []TransactionNameAgg{}, Query: q1}, nil
		}
		return nil, fmt.Errorf("failed to execute trace mapping query: %w", err)
	}

	// Build map: trace.id -> transaction.name
	traceToTxName := make(map[string]string, len(mappingResult.Values))
	for _, row := range mappingResult.Values {
		if len(row) < 2 {
			continue
		}

		var txName, traceID string
		if v, ok := row[0].(string); ok {
			txName = v
		}
		if v, ok := row[1].(string); ok {
			traceID = v
		}

		if txName != "" && traceID != "" {
			traceToTxName[traceID] = txName
		}
	}

	// Query 3: Get span counts by trace.id
	q3 := fmt.Sprintf(`FROM %s
| WHERE processor.event == "span"
  AND %s
| STATS span_count = COUNT(*) BY trace.id`,
		index, timeFilter)

	spanResult, err := exec.ExecuteESQLQuery(ctx, q3)
	if err != nil {
		if shared.IsESQLEmptyStateError(err) {
			return &TransactionNamesResult{Names: []TransactionNameAgg{}, Query: q1}, nil
		}
		return nil, fmt.Errorf("failed to execute span counts query: %w", err)
	}

	// Build map: trace.id -> span_count
	traceToSpanCount := make(map[string]int64, len(spanResult.Values))
	for _, row := range spanResult.Values {
		if len(row) < 2 {
			continue
		}

		var spanCount int64
		var traceID string

		if v, ok := row[0].(float64); ok {
			spanCount = int64(v)
		}
		if v, ok := row[1].(string); ok {
			traceID = v
		}

		if traceID != "" {
			traceToSpanCount[traceID] = spanCount
		}
	}

	// Correlate: Sum spans per transaction name
	txNameToTotalSpans := make(map[string]int64)
	for traceID, txName := range traceToTxName {
		spanCount := traceToSpanCount[traceID]
		txNameToTotalSpans[txName] += spanCount
	}

	// Enrich stats with span data
	for i := range txStats {
		totalSpans := txNameToTotalSpans[txStats[i].Name]
		if txStats[i].TraceCount > 0 {
			txStats[i].AvgSpans = float64(totalSpans) / float64(txStats[i].TraceCount)
		}
	}

	return &TransactionNamesResult{Names: txStats, Query: q1}, nil
}
