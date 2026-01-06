// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/elastic/elasticat/internal/es/shared"
)

// mockExecutor implements the Executor interface for testing
type mockExecutor struct {
	index          string
	fieldCapsResp  *shared.FieldCapsResponse
	fieldCapsErr   error
	searchResponse *shared.SearchResponse
	searchErr      error
	lastSearchBody []byte
	esqlResult     *shared.ESQLResult
}

func (m *mockExecutor) GetIndex() string {
	return m.index
}

func (m *mockExecutor) FieldCaps(ctx context.Context, index, fields string) (*shared.FieldCapsResponse, error) {
	if m.fieldCapsErr != nil {
		return nil, m.fieldCapsErr
	}
	return m.fieldCapsResp, nil
}

func (m *mockExecutor) SearchForMetrics(ctx context.Context, index string, body []byte, size int) (*shared.SearchResponse, error) {
	m.lastSearchBody = body
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResponse, nil
}

// ExecuteESQLQuery is unused in these tests but required by the interface.
func (m *mockExecutor) ExecuteESQLQuery(ctx context.Context, query string) (*shared.ESQLResult, error) {
	return m.esqlResult, nil
}

func TestSortFields(t *testing.T) {
	fields := []MetricFieldInfo{
		{Name: "metrics.z", ShortName: "z"},
		{Name: "metrics.a", ShortName: "a"},
		{Name: "metrics.m", ShortName: "m"},
	}

	SortFields(fields)

	expected := []string{"a", "m", "z"}
	for i, f := range fields {
		if f.ShortName != expected[i] {
			t.Errorf("fields[%d].ShortName = %q, want %q", i, f.ShortName, expected[i])
		}
	}
}

func TestSortFields_Empty(t *testing.T) {
	fields := []MetricFieldInfo{}
	SortFields(fields) // Should not panic
	if len(fields) != 0 {
		t.Error("Expected empty slice to remain empty")
	}
}

func TestSortFields_SingleElement(t *testing.T) {
	fields := []MetricFieldInfo{{Name: "metrics.single", ShortName: "single"}}
	SortFields(fields)
	if fields[0].ShortName != "single" {
		t.Errorf("ShortName = %q, want %q", fields[0].ShortName, "single")
	}
}

func TestGetFieldNames_Success(t *testing.T) {
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{
				"metrics.cpu.usage": {
					"double": {Type: "double", Aggregatable: true, TimeSeriesMetric: "gauge"},
				},
				"metrics.memory.used": {
					"long": {Type: "long", Aggregatable: true, TimeSeriesMetric: "gauge"},
				},
				"metrics.requests.count": {
					"histogram": {Type: "histogram", Aggregatable: true, TimeSeriesMetric: "counter"},
				},
				// Should be filtered out - not aggregatable
				"metrics.tags": {
					"keyword": {Type: "keyword", Aggregatable: false},
				},
				// Should be filtered out - object type
				"metrics.nested": {
					"object": {Type: "object", Aggregatable: false},
				},
			},
		},
	}

	fields, err := GetFieldNames(context.Background(), mock, "metrics-*")
	if err != nil {
		t.Fatalf("GetFieldNames failed: %v", err)
	}

	// Should have 3 valid metric fields
	if len(fields) != 3 {
		t.Fatalf("Expected 3 fields, got %d", len(fields))
	}

	// Check that short names are extracted correctly
	shortNames := make(map[string]bool)
	for _, f := range fields {
		shortNames[f.ShortName] = true
		// Short name should not have "metrics." prefix
		if strings.HasPrefix(f.ShortName, "metrics.") {
			t.Errorf("ShortName %q should not have metrics. prefix", f.ShortName)
		}
	}

	if !shortNames["cpu.usage"] {
		t.Error("Expected cpu.usage in fields")
	}
	if !shortNames["memory.used"] {
		t.Error("Expected memory.used in fields")
	}
}

func TestGetFieldNames_FiltersByType(t *testing.T) {
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{
				"metrics.valid_double": {
					"double": {Type: "double", Aggregatable: true},
				},
				"metrics.valid_long": {
					"long": {Type: "long", Aggregatable: true},
				},
				"metrics.valid_float": {
					"float": {Type: "float", Aggregatable: true},
				},
				"metrics.valid_half_float": {
					"half_float": {Type: "half_float", Aggregatable: true},
				},
				"metrics.valid_scaled_float": {
					"scaled_float": {Type: "scaled_float", Aggregatable: true},
				},
				"metrics.valid_histogram": {
					"histogram": {Type: "histogram", Aggregatable: true},
				},
				"metrics.valid_aggregate_metric": {
					"aggregate_metric_double": {Type: "aggregate_metric_double", Aggregatable: true},
				},
				// Invalid types
				"metrics.invalid_text": {
					"text": {Type: "text", Aggregatable: false},
				},
				"metrics.invalid_keyword": {
					"keyword": {Type: "keyword", Aggregatable: true},
				},
			},
		},
	}

	fields, err := GetFieldNames(context.Background(), mock, "metrics-*")
	if err != nil {
		t.Fatalf("GetFieldNames failed: %v", err)
	}

	// Should only include numeric types
	if len(fields) != 7 {
		t.Errorf("Expected 7 valid numeric fields, got %d", len(fields))
	}
}

func TestGetFieldNames_Error(t *testing.T) {
	mock := &mockExecutor{
		index:        "metrics-*",
		fieldCapsErr: io.EOF,
	}

	_, err := GetFieldNames(context.Background(), mock, "metrics-*")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestAggregate_Success(t *testing.T) {
	// Create mock field caps response
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{
				"metrics.cpu.usage": {
					"double": {Type: "double", Aggregatable: true, TimeSeriesMetric: "gauge"},
				},
			},
		},
	}

	// Create mock aggregation response
	now := time.Now()
	aggResponse := map[string]interface{}{
		"aggregations": map[string]interface{}{
			"m0": map[string]interface{}{
				"doc_count": float64(100),
				"stats": map[string]interface{}{
					"min": float64(10.5),
					"max": float64(95.2),
					"avg": float64(45.8),
				},
				"over_time": map[string]interface{}{
					"buckets": []interface{}{
						map[string]interface{}{
							"key":       float64(now.Add(-5 * time.Minute).UnixMilli()),
							"doc_count": float64(10),
							"value": map[string]interface{}{
								"value": float64(42.0),
							},
						},
						map[string]interface{}{
							"key":       float64(now.UnixMilli()),
							"doc_count": float64(10),
							"value": map[string]interface{}{
								"value": float64(48.0),
							},
						},
					},
				},
				"latest": map[string]interface{}{
					"hits": map[string]interface{}{
						"hits": []interface{}{
							map[string]interface{}{
								"_source": map[string]interface{}{
									"metrics": map[string]interface{}{
										"cpu": map[string]interface{}{
											"usage": float64(50.0),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	responseJSON, _ := json.Marshal(aggResponse)

	mock.searchResponse = &shared.SearchResponse{
		Body:       io.NopCloser(strings.NewReader(string(responseJSON))),
		StatusCode: 200,
		Status:     "200 OK",
		IsError:    false,
	}

	opts := AggregateMetricsOptions{
		Lookback:   "now-1h",
		BucketSize: "10s",
	}

	result, err := Aggregate(context.Background(), mock, opts)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	if result.BucketSize != "10s" {
		t.Errorf("BucketSize = %q, want %q", result.BucketSize, "10s")
	}

	if len(result.Metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(result.Metrics))
	}

	metric := result.Metrics[0]
	if metric.Name != "metrics.cpu.usage" {
		t.Errorf("Name = %q, want %q", metric.Name, "metrics.cpu.usage")
	}
	if metric.ShortName != "cpu.usage" {
		t.Errorf("ShortName = %q, want %q", metric.ShortName, "cpu.usage")
	}
	if metric.Min != 10.5 {
		t.Errorf("Min = %f, want %f", metric.Min, 10.5)
	}
	if metric.Max != 95.2 {
		t.Errorf("Max = %f, want %f", metric.Max, 95.2)
	}
	if metric.Avg != 45.8 {
		t.Errorf("Avg = %f, want %f", metric.Avg, 45.8)
	}
	if metric.Latest != 50.0 {
		t.Errorf("Latest = %f, want %f", metric.Latest, 50.0)
	}
	if len(metric.Buckets) != 2 {
		t.Errorf("Expected 2 buckets, got %d", len(metric.Buckets))
	}
}

func TestAggregate_Histogram(t *testing.T) {
	// Create mock field caps response with histogram field
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{
				"metrics.transaction.duration.histogram": {
					"histogram": {Type: "histogram", Aggregatable: true},
				},
			},
		},
	}

	// Create mock aggregation response with percentiles format
	now := time.Now()
	aggResponse := map[string]interface{}{
		"aggregations": map[string]interface{}{
			"m0": map[string]interface{}{
				"doc_count": float64(142),
				"stats": map[string]interface{}{
					"values": map[string]interface{}{
						"0.0":   float64(0),
						"50.0":  float64(179.08),
						"100.0": float64(125632.78),
					},
				},
				"over_time": map[string]interface{}{
					"buckets": []interface{}{
						map[string]interface{}{
							"key":       float64(now.Add(-10 * time.Minute).UnixMilli()),
							"doc_count": float64(70),
							"value": map[string]interface{}{
								"values": map[string]interface{}{
									"50.0": float64(150.5),
								},
							},
						},
						map[string]interface{}{
							"key":       float64(now.UnixMilli()),
							"doc_count": float64(72),
							"value": map[string]interface{}{
								"values": map[string]interface{}{
									"50.0": float64(200.3),
								},
							},
						},
					},
				},
				"last_seen": map[string]interface{}{
					"value": float64(now.UnixMilli()),
				},
				"latest": map[string]interface{}{
					"hits": map[string]interface{}{
						"hits": []interface{}{},
					},
				},
			},
		},
	}

	responseJSON, _ := json.Marshal(aggResponse)
	mock.searchResponse = &shared.SearchResponse{
		Body:       io.NopCloser(strings.NewReader(string(responseJSON))),
		StatusCode: 200,
		Status:     "200 OK",
		IsError:    false,
	}

	opts := AggregateMetricsOptions{
		Lookback:   "now-24h",
		BucketSize: "10m",
	}

	result, err := Aggregate(context.Background(), mock, opts)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	if len(result.Metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(result.Metrics))
	}

	metric := result.Metrics[0]

	// Verify histogram type is detected
	if metric.Type != "histogram" {
		t.Errorf("Type = %q, want %q", metric.Type, "histogram")
	}

	// Verify percentile values are parsed correctly
	if metric.Min != 0 {
		t.Errorf("Min = %f, want %f", metric.Min, 0.0)
	}
	if metric.Avg != 179.08 {
		t.Errorf("Avg (p50) = %f, want %f", metric.Avg, 179.08)
	}
	if metric.Max != 125632.78 {
		t.Errorf("Max = %f, want %f", metric.Max, 125632.78)
	}

	// Verify Latest is set to p50 for histograms
	if metric.Latest != 179.08 {
		t.Errorf("Latest = %f, want %f", metric.Latest, 179.08)
	}

	// Verify buckets use percentile values
	if len(metric.Buckets) != 2 {
		t.Fatalf("Expected 2 buckets, got %d", len(metric.Buckets))
	}
	if metric.Buckets[0].Value != 150.5 {
		t.Errorf("Bucket[0].Value = %f, want %f", metric.Buckets[0].Value, 150.5)
	}
	if metric.Buckets[1].Value != 200.3 {
		t.Errorf("Bucket[1].Value = %f, want %f", metric.Buckets[1].Value, 200.3)
	}

	// Verify LastSeen is set
	if metric.LastSeen.IsZero() {
		t.Error("LastSeen should be set")
	}
}

func TestAggregate_WithFilters(t *testing.T) {
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{
				"metrics.test": {
					"double": {Type: "double", Aggregatable: true},
				},
			},
		},
		searchResponse: &shared.SearchResponse{
			Body:       io.NopCloser(strings.NewReader(`{"aggregations": {}}`)),
			StatusCode: 200,
			Status:     "200 OK",
			IsError:    false,
		},
	}

	opts := AggregateMetricsOptions{
		Lookback:   "now-1h",
		BucketSize: "1m",
		Service:    "my-service",
		Resource:   "production",
	}

	_, err := Aggregate(context.Background(), mock, opts)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	// Verify filters were included in query
	var query map[string]interface{}
	if err := json.Unmarshal(mock.lastSearchBody, &query); err != nil {
		t.Fatalf("Failed to parse query: %v", err)
	}

	q, ok := query["query"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected query key")
	}

	boolQ, ok := q["bool"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected bool query")
	}

	filters, ok := boolQ["filter"].([]interface{})
	if !ok {
		t.Fatal("Expected filter array")
	}

	// Should have lookback, service, and resource filters
	if len(filters) != 3 {
		t.Errorf("Expected 3 filters, got %d", len(filters))
	}
}

func TestAggregate_NegatedFilters(t *testing.T) {
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{
				"metrics.test": {
					"double": {Type: "double", Aggregatable: true},
				},
			},
		},
		searchResponse: &shared.SearchResponse{
			Body:       io.NopCloser(strings.NewReader(`{"aggregations": {}}`)),
			StatusCode: 200,
			Status:     "200 OK",
			IsError:    false,
		},
	}

	opts := AggregateMetricsOptions{
		BucketSize:     "1m",
		Service:        "excluded-service",
		NegateService:  true,
		Resource:       "excluded-env",
		NegateResource: true,
	}

	_, err := Aggregate(context.Background(), mock, opts)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	// Verify must_not was used
	var query map[string]interface{}
	if err := json.Unmarshal(mock.lastSearchBody, &query); err != nil {
		t.Fatalf("Failed to parse query: %v", err)
	}

	q := query["query"].(map[string]interface{})
	boolQ := q["bool"].(map[string]interface{})

	mustNot, ok := boolQ["must_not"].([]interface{})
	if !ok {
		t.Fatal("Expected must_not array for negated filters")
	}

	if len(mustNot) != 2 {
		t.Errorf("Expected 2 must_not clauses, got %d", len(mustNot))
	}
}

func TestAggregate_NoMetrics(t *testing.T) {
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{},
		},
	}

	opts := AggregateMetricsOptions{BucketSize: "1m"}

	result, err := Aggregate(context.Background(), mock, opts)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	if len(result.Metrics) != 0 {
		t.Errorf("Expected 0 metrics, got %d", len(result.Metrics))
	}
}

func TestAggregate_ErrorResponse(t *testing.T) {
	mock := &mockExecutor{
		index: "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{
			Fields: map[string]map[string]shared.FieldCapsInfo{
				"metrics.test": {
					"double": {Type: "double", Aggregatable: true},
				},
			},
		},
		searchResponse: &shared.SearchResponse{
			Body:       io.NopCloser(strings.NewReader(`{"error": "index not found"}`)),
			StatusCode: 404,
			Status:     "404 Not Found",
			IsError:    true,
		},
	}

	opts := AggregateMetricsOptions{BucketSize: "1m"}

	_, err := Aggregate(context.Background(), mock, opts)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Error should mention status code: %v", err)
	}
}

func TestAggregate_LimitsMetrics(t *testing.T) {
	// Create more than 50 fields to test the limit
	fields := make(map[string]map[string]shared.FieldCapsInfo)
	for i := 0; i < 60; i++ {
		name := "metrics.field" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		fields[name] = map[string]shared.FieldCapsInfo{
			"double": {Type: "double", Aggregatable: true},
		}
	}

	mock := &mockExecutor{
		index:         "metrics-*",
		fieldCapsResp: &shared.FieldCapsResponse{Fields: fields},
		searchResponse: &shared.SearchResponse{
			Body:       io.NopCloser(strings.NewReader(`{"aggregations": {}}`)),
			StatusCode: 200,
			Status:     "200 OK",
			IsError:    false,
		},
	}

	opts := AggregateMetricsOptions{BucketSize: "1m"}

	_, err := Aggregate(context.Background(), mock, opts)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	// The query should only include 50 metrics
	var query map[string]interface{}
	if err := json.Unmarshal(mock.lastSearchBody, &query); err != nil {
		t.Fatalf("Failed to parse query: %v", err)
	}

	aggs := query["aggs"].(map[string]interface{})
	if len(aggs) != 50 {
		t.Errorf("Expected 50 aggregations, got %d", len(aggs))
	}
}

func TestGetNestedFloat(t *testing.T) {
	data := map[string]interface{}{
		"metrics": map[string]interface{}{
			"cpu": map[string]interface{}{
				"usage": float64(45.5),
			},
		},
		"simple": float64(10.0),
	}

	tests := []struct {
		path     string
		expected float64
	}{
		{"metrics.cpu.usage", 45.5},
		{"simple", 10.0},
		{"nonexistent", 0},
		{"metrics.nonexistent", 0},
		{"", 0},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			result := shared.GetNestedFloat(data, tc.path)
			if result != tc.expected {
				t.Errorf("GetNestedFloat(%q) = %f, want %f", tc.path, result, tc.expected)
			}
		})
	}
}
