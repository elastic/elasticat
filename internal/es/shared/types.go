// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

// Package shared contains types used across the es package and its subpackages.
// This package breaks import cycles by providing a common dependency.
package shared

import (
	"context"
	"io"
)

// SearchResponse represents a raw search response body from Elasticsearch.
// This is shared across traces, metrics, and perspectives packages.
type SearchResponse struct {
	Body       io.ReadCloser
	StatusCode int
	Status     string
	IsError    bool
}

// ESQLResult represents the response from an ES|QL query.
// This is shared across all packages that execute ES|QL queries.
type ESQLResult struct {
	Columns   []ESQLColumn    `json:"columns"`
	Values    [][]interface{} `json:"values"`
	Took      int             `json:"took"`
	IsPartial bool            `json:"is_partial"`
}

// ESQLColumn describes a column in an ES|QL result
type ESQLColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FieldCapsResponse represents the relevant parts of a field_caps API response
type FieldCapsResponse struct {
	Fields map[string]map[string]FieldCapsInfo
}

// FieldCapsInfo contains field capability information
type FieldCapsInfo struct {
	Type             string
	Aggregatable     bool
	TimeSeriesMetric string
}

// ESQLExecutor is the base interface for executing ES|QL queries.
// This is embedded by the more specific Executor interfaces in traces, metrics, and perspectives.
type ESQLExecutor interface {
	ExecuteESQLQuery(ctx context.Context, query string) (*ESQLResult, error)
}

// === Nested Path Extraction Helpers ===

// GetNestedPath traverses a map by dot-separated path and returns the value.
// Returns (value, true) if found, (nil, false) otherwise.
func GetNestedPath(data map[string]interface{}, path string) (interface{}, bool) {
	if data == nil || path == "" {
		return nil, false
	}
	return getNestedParts(data, splitPath(path))
}

// GetNestedParts traverses a map by path parts and returns the value.
// Returns (value, true) if found, (nil, false) otherwise.
func GetNestedParts(data map[string]interface{}, parts ...string) (interface{}, bool) {
	return getNestedParts(data, parts)
}

func getNestedParts(data map[string]interface{}, parts []string) (interface{}, bool) {
	if data == nil || len(parts) == 0 {
		return nil, false
	}

	current := interface{}(data)
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// GetNestedString retrieves a string value at the given dot-separated path.
// Returns empty string if not found or not a string.
func GetNestedString(data map[string]interface{}, path string) string {
	val, ok := GetNestedPath(data, path)
	if !ok {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// GetNestedFloat retrieves a float64 value at the given dot-separated path.
// Returns 0 if not found or not a number.
func GetNestedFloat(data map[string]interface{}, path string) float64 {
	val, ok := GetNestedPath(data, path)
	if !ok {
		return 0
	}
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	// Simple dot split - for paths like "metrics.system.cpu.usage"
	parts := []string{}
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}
