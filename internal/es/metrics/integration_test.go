// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/elastic/elasticat/internal/es/shared"
	"github.com/elastic/go-elasticsearch/v8"
)

type realExecutor struct {
	es    *elasticsearch.Client
	index string
}

func (e *realExecutor) GetIndex() string {
	return e.index
}

func (e *realExecutor) FieldCaps(ctx context.Context, index, fields string) (*shared.FieldCapsResponse, error) {
	res, err := e.es.FieldCaps(
		e.es.FieldCaps.WithContext(ctx),
		e.es.FieldCaps.WithIndex(index),
		e.es.FieldCaps.WithFields(fields),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var response struct {
		Fields map[string]map[string]struct {
			Type             string `json:"type"`
			Aggregatable     bool   `json:"aggregatable"`
			TimeSeriesMetric string `json:"time_series_metric,omitempty"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	result := &shared.FieldCapsResponse{
		Fields: make(map[string]map[string]shared.FieldCapsInfo),
	}
	for name, typeMap := range response.Fields {
		result.Fields[name] = make(map[string]shared.FieldCapsInfo)
		for typeName, info := range typeMap {
			result.Fields[name][typeName] = shared.FieldCapsInfo{
				Type:             info.Type,
				Aggregatable:     info.Aggregatable,
				TimeSeriesMetric: info.TimeSeriesMetric,
			}
		}
	}
	return result, nil
}

func (e *realExecutor) SearchForMetrics(ctx context.Context, index string, body []byte, size int) (*shared.SearchResponse, error) {
	res, err := e.es.Search(
		e.es.Search.WithContext(ctx),
		e.es.Search.WithIndex(index),
		e.es.Search.WithBody(io.NopCloser(io.Reader(bytesReader(body)))),
	)
	if err != nil {
		return nil, err
	}
	return &shared.SearchResponse{
		Body:       res.Body,
		StatusCode: res.StatusCode,
		Status:     res.Status(),
		IsError:    res.IsError(),
	}, nil
}

type bytesReader []byte

func (b bytesReader) Read(p []byte) (n int, err error) {
	return copy(p, b), io.EOF
}

// Run with: go test -tags=integration -v -run TestIntegrationHistogram ./internal/es/metrics/...
func TestIntegrationHistogram(t *testing.T) {
	esURL := os.Getenv("ES_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	cfg := elasticsearch.Config{
		Addresses: []string{esURL},
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create ES client: %v", err)
	}

	exec := &realExecutor{es: es, index: "metrics-*"}

	fields, err := GetFieldNames(context.Background(), exec, "metrics-*")
	if err != nil {
		t.Fatalf("GetFieldNames failed: %v", err)
	}

	fmt.Println("=== All discovered fields ===")
	for _, f := range fields {
		fmt.Printf("  %s: Type=%q, TimeSeriesType=%q\n", f.Name, f.Type, f.TimeSeriesType)
	}

	opts := AggregateMetricsOptions{
		Lookback:   "now-24h",
		BucketSize: "10m",
	}
	result, err := Aggregate(context.Background(), exec, opts)
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	fmt.Println("\n=== Aggregation results ===")
	for _, m := range result.Metrics {
		fmt.Printf("  %s: Type=%q, Min=%.2f, Avg=%.2f, Max=%.2f, Latest=%.2f, LastSeen=%v, Buckets=%d\n",
			m.ShortName, m.Type, m.Min, m.Avg, m.Max, m.Latest, m.LastSeen.Format(time.RFC3339), len(m.Buckets))
	}
}
