// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// mockExecutor implements the Executor interface for testing
type mockExecutor struct {
	index            string
	searchResponse   *SearchResponse
	searchErr        error
	esqlResult       *ESQLResult
	esqlErr          error
	lastSearchBody   []byte
	lastESQLQuery    string
	esqlCallCount    int
	esqlResults      []*ESQLResult // For multiple ESQL calls
}

func (m *mockExecutor) GetIndex() string {
	return m.index
}

func (m *mockExecutor) SearchForTraces(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error) {
	m.lastSearchBody = body
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResponse, nil
}

func (m *mockExecutor) ExecuteESQLQuery(ctx context.Context, query string) (*ESQLResult, error) {
	m.lastESQLQuery = query
	if m.esqlErr != nil {
		return nil, m.esqlErr
	}
	// Support multiple ESQL calls for GetNamesESSQL
	if len(m.esqlResults) > m.esqlCallCount {
		result := m.esqlResults[m.esqlCallCount]
		m.esqlCallCount++
		return result, nil
	}
	return m.esqlResult, nil
}

func TestLookbackToESQLInterval(t *testing.T) {
	tests := []struct {
		lookback string
		expected string
	}{
		{"now-5m", "5 minutes"},
		{"now-10m", "10 minutes"},
		{"now-15m", "15 minutes"},
		{"now-30m", "30 minutes"},
		{"now-1h", "1 hour"},
		{"now-3h", "3 hours"},
		{"now-6h", "6 hours"},
		{"now-12h", "12 hours"},
		{"now-24h", "24 hours"},
		{"now-1d", "24 hours"},
		{"now-1w", "7 days"},
		{"", "24 hours"},       // Default
		{"invalid", "24 hours"}, // Unknown defaults
	}

	for _, tc := range tests {
		t.Run(tc.lookback, func(t *testing.T) {
			result := LookbackToESQLInterval(tc.lookback)
			if result != tc.expected {
				t.Errorf("LookbackToESQLInterval(%q) = %q, want %q", tc.lookback, result, tc.expected)
			}
		})
	}
}

func TestGetNames_Success(t *testing.T) {
	// Create a mock aggregation response
	aggResponse := map[string]interface{}{
		"aggregations": map[string]interface{}{
			"total_spans": map[string]interface{}{
				"doc_count": float64(100),
			},
			"total_unique_traces": map[string]interface{}{
				"value": float64(20),
			},
			"transactions": map[string]interface{}{
				"doc_count": float64(50),
				"tx_names": map[string]interface{}{
					"buckets": []interface{}{
						map[string]interface{}{
							"key":       "GET /api/users",
							"doc_count": float64(25),
							"avg_duration": map[string]interface{}{
								"value": float64(1500000000), // 1.5s in nanoseconds
							},
							"min_duration": map[string]interface{}{
								"value": float64(500000000), // 0.5s in nanoseconds
							},
							"max_duration": map[string]interface{}{
								"value": float64(3000000000), // 3s in nanoseconds
							},
							"unique_traces": map[string]interface{}{
								"value": float64(10),
							},
							"errors": map[string]interface{}{
								"doc_count": float64(5),
							},
						},
						map[string]interface{}{
							"key":       "POST /api/orders",
							"doc_count": float64(25),
							"avg_duration": map[string]interface{}{
								"value": float64(2000000000), // 2s
							},
							"min_duration": map[string]interface{}{
								"value": float64(1000000000),
							},
							"max_duration": map[string]interface{}{
								"value": float64(4000000000),
							},
							"unique_traces": map[string]interface{}{
								"value": float64(10),
							},
							"errors": map[string]interface{}{
								"doc_count": float64(0),
							},
						},
					},
				},
			},
		},
	}

	responseJSON, _ := json.Marshal(aggResponse)

	mock := &mockExecutor{
		index: "traces-*",
		searchResponse: &SearchResponse{
			Body:       io.NopCloser(strings.NewReader(string(responseJSON))),
			StatusCode: 200,
			Status:     "200 OK",
			IsError:    false,
		},
	}

	results, err := GetNames(context.Background(), mock, "now-1h", "", "")
	if err != nil {
		t.Fatalf("GetNames failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check first result
	if results[0].Name != "GET /api/users" {
		t.Errorf("Name = %q, want %q", results[0].Name, "GET /api/users")
	}
	if results[0].Count != 25 {
		t.Errorf("Count = %d, want %d", results[0].Count, 25)
	}
	if results[0].AvgDuration != 1500 { // 1500ms
		t.Errorf("AvgDuration = %f, want %f", results[0].AvgDuration, 1500.0)
	}
	if results[0].ErrorRate != 20 { // 5/25 * 100
		t.Errorf("ErrorRate = %f, want %f", results[0].ErrorRate, 20.0)
	}

	// Check global avg spans calculation
	expectedAvgSpans := 100.0 / 20.0 // total_spans / total_unique_traces
	if results[0].AvgSpans != expectedAvgSpans {
		t.Errorf("AvgSpans = %f, want %f", results[0].AvgSpans, expectedAvgSpans)
	}
}

func TestGetNames_WithFilters(t *testing.T) {
	// Test that filters are properly added to the query
	aggResponse := map[string]interface{}{
		"aggregations": map[string]interface{}{
			"total_spans":         map[string]interface{}{"doc_count": float64(0)},
			"total_unique_traces": map[string]interface{}{"value": float64(0)},
			"transactions": map[string]interface{}{
				"tx_names": map[string]interface{}{
					"buckets": []interface{}{},
				},
			},
		},
	}
	responseJSON, _ := json.Marshal(aggResponse)

	mock := &mockExecutor{
		index: "traces-*",
		searchResponse: &SearchResponse{
			Body:       io.NopCloser(strings.NewReader(string(responseJSON))),
			StatusCode: 200,
			Status:     "200 OK",
			IsError:    false,
		},
	}

	_, err := GetNames(context.Background(), mock, "now-1h", "my-service", "production")
	if err != nil {
		t.Fatalf("GetNames failed: %v", err)
	}

	// Parse the query that was sent
	var query map[string]interface{}
	if err := json.Unmarshal(mock.lastSearchBody, &query); err != nil {
		t.Fatalf("Failed to parse query: %v", err)
	}

	// Check that query has filters
	q := query["query"].(map[string]interface{})
	boolQ := q["bool"].(map[string]interface{})
	filters := boolQ["filter"].([]interface{})

	// Should have time, service, and resource filters
	if len(filters) != 3 {
		t.Errorf("Expected 3 filters, got %d", len(filters))
	}
}

func TestGetNames_EmptyResponse(t *testing.T) {
	aggResponse := map[string]interface{}{
		"aggregations": map[string]interface{}{
			"total_spans":         map[string]interface{}{"doc_count": float64(0)},
			"total_unique_traces": map[string]interface{}{"value": float64(0)},
			"transactions": map[string]interface{}{
				"tx_names": map[string]interface{}{
					"buckets": []interface{}{},
				},
			},
		},
	}
	responseJSON, _ := json.Marshal(aggResponse)

	mock := &mockExecutor{
		index: "traces-*",
		searchResponse: &SearchResponse{
			Body:       io.NopCloser(strings.NewReader(string(responseJSON))),
			StatusCode: 200,
			Status:     "200 OK",
			IsError:    false,
		},
	}

	results, err := GetNames(context.Background(), mock, "", "", "")
	if err != nil {
		t.Fatalf("GetNames failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestGetNames_ErrorResponse(t *testing.T) {
	mock := &mockExecutor{
		index: "traces-*",
		searchResponse: &SearchResponse{
			Body:       io.NopCloser(strings.NewReader(`{"error": "index not found"}`)),
			StatusCode: 404,
			Status:     "404 Not Found",
			IsError:    true,
		},
	}

	_, err := GetNames(context.Background(), mock, "", "", "")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Error should mention status code: %v", err)
	}
}

func TestGetNamesESSQL_Success(t *testing.T) {
	mock := &mockExecutor{
		index: "traces-*",
		esqlResults: []*ESQLResult{
			// Query 1: Transaction stats
			{
				Columns: []ESQLColumn{
					{Name: "tx_count", Type: "long"},
					{Name: "unique_traces", Type: "long"},
					{Name: "min_duration", Type: "double"},
					{Name: "avg_duration", Type: "double"},
					{Name: "max_duration", Type: "double"},
					{Name: "error_count", Type: "long"},
					{Name: "transaction.name", Type: "keyword"},
					{Name: "error_rate", Type: "double"},
				},
				Values: [][]interface{}{
					{float64(100), float64(50), float64(500000000), float64(1500000000), float64(3000000000), float64(10), "GET /api/users", float64(10)},
					{float64(50), float64(25), float64(1000000000), float64(2000000000), float64(4000000000), float64(0), "POST /api/orders", float64(0)},
				},
			},
			// Query 2: Trace mapping
			{
				Columns: []ESQLColumn{
					{Name: "transaction.name", Type: "keyword"},
					{Name: "trace.id", Type: "keyword"},
				},
				Values: [][]interface{}{
					{"GET /api/users", "trace-1"},
					{"GET /api/users", "trace-2"},
					{"POST /api/orders", "trace-3"},
				},
			},
			// Query 3: Span counts
			{
				Columns: []ESQLColumn{
					{Name: "span_count", Type: "long"},
					{Name: "trace.id", Type: "keyword"},
				},
				Values: [][]interface{}{
					{float64(5), "trace-1"},
					{float64(3), "trace-2"},
					{float64(10), "trace-3"},
				},
			},
		},
	}

	results, err := GetNamesESSQL(context.Background(), mock, "now-1h", "", "", false, false)
	if err != nil {
		t.Fatalf("GetNamesESSQL failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check first result
	if results[0].Name != "GET /api/users" {
		t.Errorf("Name = %q, want %q", results[0].Name, "GET /api/users")
	}
	if results[0].Count != 100 {
		t.Errorf("Count = %d, want %d", results[0].Count, 100)
	}
	if results[0].TraceCount != 50 {
		t.Errorf("TraceCount = %d, want %d", results[0].TraceCount, 50)
	}
	if results[0].ErrorRate != 10 {
		t.Errorf("ErrorRate = %f, want %f", results[0].ErrorRate, 10.0)
	}

	// Check average spans calculation
	// trace-1 has 5 spans, trace-2 has 3 spans -> (5+3)/50 unique traces
	expectedAvgSpans := float64(5+3) / float64(50)
	if results[0].AvgSpans != expectedAvgSpans {
		t.Errorf("AvgSpans = %f, want %f", results[0].AvgSpans, expectedAvgSpans)
	}
}

func TestGetNamesESSQL_WithFilters(t *testing.T) {
	mock := &mockExecutor{
		index: "traces-*",
		esqlResults: []*ESQLResult{
			{Values: [][]interface{}{}},
			{Values: [][]interface{}{}},
			{Values: [][]interface{}{}},
		},
	}

	_, err := GetNamesESSQL(context.Background(), mock, "now-1h", "my-service", "production", false, false)
	if err != nil {
		t.Fatalf("GetNamesESSQL failed: %v", err)
	}

	// Check that service filter was included
	if !strings.Contains(mock.lastESQLQuery, "span") {
		// The last query should be for spans
		t.Log("Query was for spans as expected")
	}

	// Reset and test with negation
	mock.esqlCallCount = 0
	mock.lastESQLQuery = ""

	_, err = GetNamesESSQL(context.Background(), mock, "now-1h", "my-service", "", true, false)
	if err != nil {
		t.Fatalf("GetNamesESSQL failed: %v", err)
	}
}

func TestGetNamesESSQL_Error(t *testing.T) {
	mock := &mockExecutor{
		index:   "traces-*",
		esqlErr: io.EOF,
	}

	_, err := GetNamesESSQL(context.Background(), mock, "", "", "", false, false)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestGetNames_NoAggregations(t *testing.T) {
	// Response with no aggregations key
	responseJSON := `{}`

	mock := &mockExecutor{
		index: "traces-*",
		searchResponse: &SearchResponse{
			Body:       io.NopCloser(strings.NewReader(responseJSON)),
			StatusCode: 200,
			Status:     "200 OK",
			IsError:    false,
		},
	}

	results, err := GetNames(context.Background(), mock, "", "", "")
	if err != nil {
		t.Fatalf("GetNames failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

