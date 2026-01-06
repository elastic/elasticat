// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package perspectives

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/elastic/elasticat/internal/es/traces"
)

// mockExecutor implements the Executor interface for testing
type mockExecutor struct {
	esqlResult    *traces.ESQLResult
	esqlErr       error
	lastESQLQuery string
}

func (m *mockExecutor) SearchForPerspectives(ctx context.Context, index string, body []byte, size int) (*SearchResponse, error) {
	return nil, nil // Not used in current tests
}

func (m *mockExecutor) ExecuteESQLQuery(ctx context.Context, query string) (*traces.ESQLResult, error) {
	m.lastESQLQuery = query
	if m.esqlErr != nil {
		return nil, m.esqlErr
	}
	return m.esqlResult, nil
}

func TestGetByField_Success(t *testing.T) {
	mock := &mockExecutor{
		esqlResult: &traces.ESQLResult{
			Columns: []traces.ESQLColumn{
				{Name: "logs", Type: "long"},
				{Name: "traces", Type: "long"},
				{Name: "metrics", Type: "long"},
				{Name: "service.name", Type: "keyword"},
			},
			Values: [][]interface{}{
				{float64(100), float64(50), float64(25), "api-service"},
				{float64(80), float64(40), float64(20), "web-service"},
				{float64(60), float64(30), float64(15), "worker-service"},
			},
		},
	}

	results, err := GetByField(context.Background(), mock, "now-1h", "service.name")
	if err != nil {
		t.Fatalf("GetByField failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Check first result
	if results[0].Name != "api-service" {
		t.Errorf("Name = %q, want %q", results[0].Name, "api-service")
	}
	if results[0].LogCount != 100 {
		t.Errorf("LogCount = %d, want %d", results[0].LogCount, 100)
	}
	if results[0].TraceCount != 50 {
		t.Errorf("TraceCount = %d, want %d", results[0].TraceCount, 50)
	}
	if results[0].MetricCount != 25 {
		t.Errorf("MetricCount = %d, want %d", results[0].MetricCount, 25)
	}
}

func TestGetByField_QueryFormat(t *testing.T) {
	mock := &mockExecutor{
		esqlResult: &traces.ESQLResult{
			Columns: []traces.ESQLColumn{},
			Values:  [][]interface{}{},
		},
	}

	_, err := GetByField(context.Background(), mock, "now-1h", "service.name")
	if err != nil {
		t.Fatalf("GetByField failed: %v", err)
	}

	// Verify query contains expected elements
	query := mock.lastESQLQuery
	if !strings.Contains(query, "service.name") {
		t.Error("Query should contain field name")
	}
	if !strings.Contains(query, "1 hour") {
		t.Error("Query should contain lookback interval")
	}
	if !strings.Contains(query, "STATS") {
		t.Error("Query should contain STATS")
	}
	if !strings.Contains(query, "logs") {
		t.Error("Query should count logs")
	}
	if !strings.Contains(query, "traces") {
		t.Error("Query should count traces")
	}
	if !strings.Contains(query, "metrics") {
		t.Error("Query should count metrics")
	}
}

func TestGetByField_EmptyResponse(t *testing.T) {
	mock := &mockExecutor{
		esqlResult: &traces.ESQLResult{
			Columns: []traces.ESQLColumn{
				{Name: "logs", Type: "long"},
				{Name: "traces", Type: "long"},
				{Name: "metrics", Type: "long"},
				{Name: "service.name", Type: "keyword"},
			},
			Values: [][]interface{}{},
		},
	}

	results, err := GetByField(context.Background(), mock, "now-1h", "service.name")
	if err != nil {
		t.Fatalf("GetByField failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestGetByField_SkipsEmptyNames(t *testing.T) {
	mock := &mockExecutor{
		esqlResult: &traces.ESQLResult{
			Columns: []traces.ESQLColumn{
				{Name: "logs", Type: "long"},
				{Name: "traces", Type: "long"},
				{Name: "metrics", Type: "long"},
				{Name: "service.name", Type: "keyword"},
			},
			Values: [][]interface{}{
				{float64(100), float64(50), float64(25), "valid-service"},
				{float64(80), float64(40), float64(20), ""},  // Empty name - should be skipped
				{float64(60), float64(30), float64(15), nil}, // Nil name - should be skipped
			},
		},
	}

	results, err := GetByField(context.Background(), mock, "now-1h", "service.name")
	if err != nil {
		t.Fatalf("GetByField failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result (skipping empty names), got %d", len(results))
	}

	if results[0].Name != "valid-service" {
		t.Errorf("Name = %q, want %q", results[0].Name, "valid-service")
	}
}

func TestGetByField_Error(t *testing.T) {
	mock := &mockExecutor{
		esqlErr: io.EOF,
	}

	_, err := GetByField(context.Background(), mock, "now-1h", "service.name")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "perspective query failed") {
		t.Errorf("Error should mention perspective query: %v", err)
	}
}

func TestGetServices(t *testing.T) {
	mock := &mockExecutor{
		esqlResult: &traces.ESQLResult{
			Columns: []traces.ESQLColumn{
				{Name: "logs", Type: "long"},
				{Name: "traces", Type: "long"},
				{Name: "metrics", Type: "long"},
				{Name: "service.name", Type: "keyword"},
			},
			Values: [][]interface{}{
				{float64(100), float64(50), float64(25), "my-service"},
			},
		},
	}

	results, err := GetServices(context.Background(), mock, "now-1h")
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}

	// Verify it used service.name field
	if !strings.Contains(mock.lastESQLQuery, "service.name") {
		t.Error("GetServices should query by service.name")
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestGetResources(t *testing.T) {
	mock := &mockExecutor{
		esqlResult: &traces.ESQLResult{
			Columns: []traces.ESQLColumn{
				{Name: "logs", Type: "long"},
				{Name: "traces", Type: "long"},
				{Name: "metrics", Type: "long"},
				{Name: "resource.attributes.deployment.environment", Type: "keyword"},
			},
			Values: [][]interface{}{
				{float64(100), float64(50), float64(25), "production"},
			},
		},
	}

	results, err := GetResources(context.Background(), mock, "now-24h")
	if err != nil {
		t.Fatalf("GetResources failed: %v", err)
	}

	// Verify it used deployment.environment field
	if !strings.Contains(mock.lastESQLQuery, "resource.attributes.deployment.environment") {
		t.Error("GetResources should query by deployment.environment")
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestGetByField_MissingColumns(t *testing.T) {
	// Test robustness when columns are missing or in different order
	mock := &mockExecutor{
		esqlResult: &traces.ESQLResult{
			Columns: []traces.ESQLColumn{
				{Name: "service.name", Type: "keyword"},
				{Name: "logs", Type: "long"},
				// traces and metrics columns missing
			},
			Values: [][]interface{}{
				{"my-service", float64(100)},
			},
		},
	}

	results, err := GetByField(context.Background(), mock, "now-1h", "service.name")
	if err != nil {
		t.Fatalf("GetByField failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Should have logs but 0 for missing traces/metrics
	if results[0].LogCount != 100 {
		t.Errorf("LogCount = %d, want %d", results[0].LogCount, 100)
	}
	if results[0].TraceCount != 0 {
		t.Errorf("TraceCount = %d, want %d (missing column)", results[0].TraceCount, 0)
	}
	if results[0].MetricCount != 0 {
		t.Errorf("MetricCount = %d, want %d (missing column)", results[0].MetricCount, 0)
	}
}

