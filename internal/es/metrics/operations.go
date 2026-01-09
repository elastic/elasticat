// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/elastic/elasticat/internal/es/errfmt"
	"github.com/elastic/elasticat/internal/es/shared"
)

// GetFieldNames discovers metric field names from field_caps API
// Returns only aggregatable numeric fields under the "metrics.*" namespace
func GetFieldNames(ctx context.Context, exec Executor, index string) ([]MetricFieldInfo, error) {
	return getFieldNamesFiltered(ctx, exec, index, false)
}

// GetFieldNamesForESQL discovers metric field names compatible with ES|QL aggregations.
// ES|QL cannot aggregate counter types or histogram fields.
func GetFieldNamesForESQL(ctx context.Context, exec Executor, index string) ([]MetricFieldInfo, error) {
	return getFieldNamesFiltered(ctx, exec, index, true)
}

func getFieldNamesFiltered(ctx context.Context, exec Executor, index string, esqlCompatible bool) ([]MetricFieldInfo, error) {
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

			// For ES|QL compatibility, skip types that can't be aggregated
			if esqlCompatible {
				// ES|QL doesn't support histogram types
				if info.Type == "histogram" {
					continue
				}
				// ES|QL doesn't support MIN/MAX/AVG on counter types
				if info.TimeSeriesMetric == "counter" {
					continue
				}
			}

			// Filter to supported numeric types
			switch info.Type {
			case "long", "double", "float", "half_float", "scaled_float", "aggregate_metric_double":
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
			case "histogram":
				// Only include histogram for non-ES|QL (Query DSL) path
				if !esqlCompatible {
					shortName := name
					if strings.HasPrefix(name, "metrics.") {
						shortName = name[8:]
					}
					metricFields = append(metricFields, MetricFieldInfo{
						Name:           name,
						ShortName:      shortName,
						Type:           info.Type,
						TimeSeriesType: info.TimeSeriesMetric,
					})
				}
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
// NOTE: This path intentionally remains Query DSL-based. The ES|QL surface
// cannot yet express the dynamic field discovery plus extended_stats/date
// histogram combination we build here without multiple client-side joins,
// so DSL is kept for correctness.
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

		// Build sub-aggregations based on field type
		subAggs := map[string]interface{}{
			// Always add last_seen aggregation (works for all types)
			"last_seen": map[string]interface{}{
				"max": map[string]interface{}{
					"field": "@timestamp",
				},
			},
			"latest": map[string]interface{}{
				"top_hits": map[string]interface{}{
					"size": 1,
					"sort": []map[string]interface{}{
						{"@timestamp": "desc"},
					},
					"_source": []string{mf.Name, "@timestamp"},
				},
			},
		}

		if mf.Type == "histogram" || mf.Type == "aggregate_metric_double" {
			// For histogram and aggregate_metric_double fields, use individual min/max/avg aggregations
			// (extended_stats doesn't work with these pre-aggregated field types and causes shard failures)
			subAggs["hist_min"] = map[string]interface{}{
				"min": map[string]interface{}{
					"field": mf.Name,
				},
			}
			subAggs["hist_max"] = map[string]interface{}{
				"max": map[string]interface{}{
					"field": mf.Name,
				},
			}
			subAggs["hist_avg"] = map[string]interface{}{
				"avg": map[string]interface{}{
					"field": mf.Name,
				},
			}
			subAggs["over_time"] = map[string]interface{}{
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
			}
		} else {
			// For regular numeric fields, use extended_stats and avg
			subAggs["stats"] = map[string]interface{}{
				"extended_stats": map[string]interface{}{
					"field": mf.Name,
				},
			}
			subAggs["over_time"] = map[string]interface{}{
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
			}
		}

		aggs[aggName] = map[string]interface{}{
			"filter": map[string]interface{}{
				"exists": map[string]interface{}{
					"field": mf.Name,
				},
			},
			"aggs": subAggs,
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
		return nil, errfmt.FormatQueryError(res.Status, body, queryJSON)
	}

	result, err := parseAggResponse(res.Body, metricFields, opts.BucketSize)
	if err != nil {
		return nil, err
	}

	// Generate an ES|QL query for Kibana integration (simpler than the full DSL query)
	result.Query = generateKibanaESQLQuery(index, opts)

	return result, nil
}

// generateKibanaESQLQuery creates an ES|QL query for Kibana integration.
// Groups metrics by metricset.name to show which metric types are present and their volume.
func generateKibanaESQLQuery(index string, opts AggregateMetricsOptions) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("FROM %s\n", index))

	// Build WHERE clause
	whereParts := []string{}
	if opts.Lookback != "" {
		whereParts = append(whereParts, fmt.Sprintf("@timestamp >= NOW() - %s", shared.LookbackToESQLInterval(opts.Lookback)))
	}
	if opts.Service != "" {
		op := "=="
		if opts.NegateService {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("service.name %s \"%s\"", op, opts.Service))
	}
	if opts.Resource != "" {
		op := "=="
		if opts.NegateResource {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("resource.attributes.deployment.environment %s \"%s\"", op, opts.Resource))
	}

	if len(whereParts) > 0 {
		sb.WriteString("| WHERE ")
		sb.WriteString(strings.Join(whereParts, " AND "))
		sb.WriteString("\n")
	}

	// Group by metricset.name to show breakdown of metric types
	sb.WriteString("| STATS doc_count = COUNT(*) BY metricset.name\n")
	sb.WriteString("| SORT doc_count DESC, metricset.name")

	return sb.String()
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

		// Skip metrics with no data (doc_count == 0 means no documents have this field)
		docCount, _ := metricAgg["doc_count"].(float64)
		if docCount == 0 {
			continue
		}

		am := AggregatedMetric{
			Name:      mf.Name,
			ShortName: mf.ShortName,
			Type:      mf.TimeSeriesType,
		}

		// Mark if this is a histogram type for display purposes
		isHistogram := mf.Type == "histogram"
		isPreAggregated := mf.Type == "histogram" || mf.Type == "aggregate_metric_double"
		if isHistogram {
			am.Type = "histogram"
		}

		// Extract stats - handle both extended_stats and pre-aggregated field types
		if isPreAggregated {
			// For histogram and aggregate_metric_double, we use separate min/max/avg aggregations
			if histMin, ok := metricAgg["hist_min"].(map[string]interface{}); ok {
				if v, ok := histMin["value"].(float64); ok {
					am.Min = v
				}
			}
			if histMax, ok := metricAgg["hist_max"].(map[string]interface{}); ok {
				if v, ok := histMax["value"].(float64); ok {
					am.Max = v
				}
			}
			if histAvg, ok := metricAgg["hist_avg"].(map[string]interface{}); ok {
				if v, ok := histAvg["value"].(float64); ok {
					am.Avg = v
				}
			}
		} else if stats, ok := metricAgg["stats"].(map[string]interface{}); ok {
			// extended_stats response for regular numeric fields
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
						// All metrics now use avg for over_time buckets
						if v, ok := value["value"].(float64); ok {
							mb.Value = v
						}
					}
					am.Buckets = append(am.Buckets, mb)
				}
			}
		}

		// Extract last_seen from max aggregation (works for all types)
		if lastSeen, ok := metricAgg["last_seen"].(map[string]interface{}); ok {
			if v, ok := lastSeen["value"].(float64); ok {
				am.LastSeen = time.UnixMilli(int64(v))
			}
		}

		// Extract latest value from top_hits (may not work for histogram types)
		if latest, ok := metricAgg["latest"].(map[string]interface{}); ok {
			if hits, ok := latest["hits"].(map[string]interface{}); ok {
				if hitsList, ok := hits["hits"].([]interface{}); ok && len(hitsList) > 0 {
					if hit, ok := hitsList[0].(map[string]interface{}); ok {
						if source, ok := hit["_source"].(map[string]interface{}); ok {
							// For histograms, latest value doesn't make sense (it's a distribution)
							// We'll use the p50 (median) from the most recent percentiles instead
							if !isHistogram {
								am.Latest = shared.GetNestedFloat(source, mf.Name)
							}
						}
					}
				}
			}
		}

		// For histograms, use the median (Avg which is p50) as "Latest" display value
		if isHistogram && am.Latest == 0 {
			am.Latest = am.Avg
		}

		result.Metrics = append(result.Metrics, am)
	}

	return result, nil
}

// Note: extractNestedFloat has been replaced by shared.GetNestedFloat

// AggregateESQL retrieves aggregated statistics for metrics using ES|QL.
// This version uses ES|QL for stats computation, making the query available for Kibana.
// Note: Sparkline buckets are not available in ES|QL mode (date_histogram not supported).
func AggregateESQL(ctx context.Context, exec Executor, opts AggregateMetricsOptions) (*MetricsAggResult, error) {
	index := exec.GetIndex()

	// Discover metrics using field_caps, filtering for ES|QL-compatible types
	// (excludes histogram and counter types which ES|QL can't aggregate)
	metricFields, err := GetFieldNamesForESQL(ctx, exec, index)
	if err != nil {
		return nil, err
	}

	if len(metricFields) == 0 {
		return &MetricsAggResult{Metrics: []AggregatedMetric{}, BucketSize: opts.BucketSize}, nil
	}

	// Limit to 20 metrics to avoid huge ES|QL queries
	maxMetrics := 20
	if len(metricFields) > maxMetrics {
		metricFields = metricFields[:maxMetrics]
	}

	// Build ES|QL query for stats
	query := buildMetricsESQLQuery(index, metricFields, opts)

	// Execute stats query
	statsResult, err := exec.ExecuteESQLQuery(ctx, query)
	if err != nil {
		// Treat expected empty-state errors (no matching indices, unsupported field types)
		// as empty results rather than surfacing errors to the UI.
		if shared.IsESQLEmptyStateError(err) {
			return &MetricsAggResult{Metrics: []AggregatedMetric{}, BucketSize: opts.BucketSize, Query: query}, nil
		}
		return nil, fmt.Errorf("ES|QL metrics stats query failed: %w", err)
	}

	// Parse stats results
	result := parseESQLStatsResult(statsResult, metricFields, opts.BucketSize)
	result.Query = query

	// Get latest values with a separate query
	latestQuery := buildLatestValueQuery(index, metricFields, opts)
	latestResult, err := exec.ExecuteESQLQuery(ctx, latestQuery)
	if err == nil {
		enrichWithLatestValues(result, latestResult, metricFields)
	}

	return result, nil
}

// buildMetricsESQLQuery constructs an ES|QL query for metric statistics
func buildMetricsESQLQuery(index string, fields []MetricFieldInfo, opts AggregateMetricsOptions) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("FROM %s\n", index))

	// Build WHERE clause
	whereParts := []string{}
	if opts.Lookback != "" {
		whereParts = append(whereParts, fmt.Sprintf("@timestamp >= NOW() - %s", shared.LookbackToESQLInterval(opts.Lookback)))
	}
	if opts.Service != "" {
		op := "=="
		if opts.NegateService {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("service.name %s \"%s\"", op, opts.Service))
	}
	if opts.Resource != "" {
		op := "=="
		if opts.NegateResource {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("resource.attributes.deployment.environment %s \"%s\"", op, opts.Resource))
	}

	if len(whereParts) > 0 {
		sb.WriteString("| WHERE ")
		sb.WriteString(strings.Join(whereParts, " AND "))
		sb.WriteString("\n")
	}

	// Build STATS clause for each metric
	sb.WriteString("| STATS\n")
	statsParts := []string{}
	for i, mf := range fields {
		// Use backticks for field names with dots
		fieldRef := fmt.Sprintf("`%s`", mf.Name)
		statsParts = append(statsParts,
			fmt.Sprintf("    m%d_min = MIN(%s), m%d_max = MAX(%s), m%d_avg = AVG(%s)", i, fieldRef, i, fieldRef, i, fieldRef))
	}
	sb.WriteString(strings.Join(statsParts, ",\n"))

	return sb.String()
}

// buildLatestValueQuery constructs an ES|QL query to get the latest value for each metric
func buildLatestValueQuery(index string, fields []MetricFieldInfo, opts AggregateMetricsOptions) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("FROM %s\n", index))

	// Build WHERE clause (same as stats query)
	whereParts := []string{}
	if opts.Lookback != "" {
		whereParts = append(whereParts, fmt.Sprintf("@timestamp >= NOW() - %s", shared.LookbackToESQLInterval(opts.Lookback)))
	}
	if opts.Service != "" {
		op := "=="
		if opts.NegateService {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("service.name %s \"%s\"", op, opts.Service))
	}
	if opts.Resource != "" {
		op := "=="
		if opts.NegateResource {
			op = "!="
		}
		whereParts = append(whereParts, fmt.Sprintf("resource.attributes.deployment.environment %s \"%s\"", op, opts.Resource))
	}

	if len(whereParts) > 0 {
		sb.WriteString("| WHERE ")
		sb.WriteString(strings.Join(whereParts, " AND "))
		sb.WriteString("\n")
	}

	// Sort by timestamp descending and get first row
	sb.WriteString("| SORT @timestamp DESC\n")
	sb.WriteString("| LIMIT 1\n")

	// Keep only the metric fields
	keepFields := []string{"@timestamp"}
	for _, mf := range fields {
		keepFields = append(keepFields, fmt.Sprintf("`%s`", mf.Name))
	}
	sb.WriteString("| KEEP ")
	sb.WriteString(strings.Join(keepFields, ", "))

	return sb.String()
}

// parseESQLStatsResult parses the ES|QL stats result into MetricsAggResult
func parseESQLStatsResult(result *shared.ESQLResult, fields []MetricFieldInfo, bucketSize string) *MetricsAggResult {
	aggResult := &MetricsAggResult{
		Metrics:    make([]AggregatedMetric, 0, len(fields)),
		BucketSize: bucketSize,
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range result.Columns {
		colIndex[col.Name] = i
	}

	// Parse the single result row (STATS without BY returns one row)
	if len(result.Values) == 0 {
		return aggResult
	}

	row := result.Values[0]

	for i, mf := range fields {
		minCol := fmt.Sprintf("m%d_min", i)
		maxCol := fmt.Sprintf("m%d_max", i)
		avgCol := fmt.Sprintf("m%d_avg", i)

		var minVal, maxVal, avgVal float64
		var hasData bool

		if idx, ok := colIndex[minCol]; ok && idx < len(row) {
			if v, ok := row[idx].(float64); ok {
				minVal = v
				hasData = true
			}
		}
		if idx, ok := colIndex[maxCol]; ok && idx < len(row) {
			if v, ok := row[idx].(float64); ok {
				maxVal = v
				hasData = true
			}
		}
		if idx, ok := colIndex[avgCol]; ok && idx < len(row) {
			if v, ok := row[idx].(float64); ok {
				avgVal = v
				hasData = true
			}
		}

		// Skip metrics with no data (all stats are null)
		if !hasData {
			continue
		}

		aggResult.Metrics = append(aggResult.Metrics, AggregatedMetric{
			Name:      mf.Name,
			ShortName: mf.ShortName,
			Type:      mf.TimeSeriesType,
			Min:       minVal,
			Max:       maxVal,
			Avg:       avgVal,
		})
	}

	return aggResult
}

// enrichWithLatestValues adds latest values from the latest query result
func enrichWithLatestValues(result *MetricsAggResult, latestResult *shared.ESQLResult, fields []MetricFieldInfo) {
	if len(latestResult.Values) == 0 {
		return
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range latestResult.Columns {
		colIndex[col.Name] = i
	}

	row := latestResult.Values[0]

	// Extract @timestamp for LastSeen (same for all metrics since it's one row)
	var lastSeen time.Time
	if idx, ok := colIndex["@timestamp"]; ok && idx < len(row) {
		if ts, ok := row[idx].(string); ok {
			if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				lastSeen = parsed
			}
		}
	}

	// Match by metric name since indices may not align after filtering
	for i := range result.Metrics {
		metricName := result.Metrics[i].Name
		if idx, ok := colIndex[metricName]; ok && idx < len(row) {
			if v, ok := row[idx].(float64); ok {
				result.Metrics[i].Latest = v
			}
		}
		// Set LastSeen for all metrics from the latest document
		if !lastSeen.IsZero() {
			result.Metrics[i].LastSeen = lastSeen
		}
	}
}
