// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"encoding/json"

	"github.com/elastic/elasticat/internal/es/shared"
)

// PrettyJSON best-effort pretty prints a raw JSON string.
// If indenting fails, it returns the raw input.
func PrettyJSON(raw string) string {
	if raw == "" {
		return raw
	}
	var tmp interface{}
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return raw
	}
	out, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		return raw
	}
	return string(out)
}

// === Nested Path Extraction Helpers ===
// These delegate to es/shared to avoid import cycles with subpackages.

// GetNestedPath traverses a map by dot-separated path and returns the value.
func GetNestedPath(data map[string]interface{}, path string) (interface{}, bool) {
	return shared.GetNestedPath(data, path)
}

// GetNestedParts traverses a map by path parts and returns the value.
func GetNestedParts(data map[string]interface{}, parts ...string) (interface{}, bool) {
	return shared.GetNestedParts(data, parts...)
}

// GetNestedString retrieves a string value at the given dot-separated path.
func GetNestedString(data map[string]interface{}, path string) string {
	return shared.GetNestedString(data, path)
}

// GetNestedFloat retrieves a float64 value at the given dot-separated path.
func GetNestedFloat(data map[string]interface{}, path string) float64 {
	return shared.GetNestedFloat(data, path)
}
