// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"testing"
)

func TestLookbackToESQLInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lookback string
		expected string
	}{
		// Minute intervals
		{"5 minutes", "now-5m", "5 minutes"},
		{"10 minutes", "now-10m", "10 minutes"},
		{"15 minutes", "now-15m", "15 minutes"},
		{"30 minutes", "now-30m", "30 minutes"},

		// Hour intervals
		{"1 hour", "now-1h", "1 hour"},
		{"3 hours", "now-3h", "3 hours"},
		{"6 hours", "now-6h", "6 hours"},
		{"12 hours", "now-12h", "12 hours"},
		{"24 hours", "now-24h", "24 hours"},

		// Day intervals
		{"1 day equals 24 hours", "now-1d", "24 hours"},
		{"1 week", "now-1w", "7 days"},

		// Default cases
		{"unknown returns default", "now-2h", "24 hours"},
		{"empty string returns default", "", "24 hours"},
		{"invalid format returns default", "invalid", "24 hours"},
		{"partial match returns default", "now-", "24 hours"},
		{"wrong prefix returns default", "yesterday-1h", "24 hours"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := LookbackToESQLInterval(tc.lookback)
			if result != tc.expected {
				t.Errorf("LookbackToESQLInterval(%q) = %q, want %q", tc.lookback, result, tc.expected)
			}
		})
	}
}

func TestEscapeESQLString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"no special chars", "hello world", "hello world"},
		{"single quote", "it's fine", "it's fine"},
		{"double quote", `say "hello"`, `say \"hello\"`},
		{"multiple double quotes", `"a" and "b"`, `\"a\" and \"b\"`},
		{"mixed content", `user: "john" level: "info"`, `user: \"john\" level: \"info\"`},
		{"only double quote", `"`, `\"`},
		{"consecutive double quotes", `""`, `\"\"`},
		{"newlines preserved", "line1\nline2", "line1\nline2"},
		{"tabs preserved", "col1\tcol2", "col1\tcol2"},
		{"unicode preserved", "日本語", "日本語"},
		{"special chars without quotes", "a@b#c$d%e", "a@b#c$d%e"},
		{"backslash preserved", `path\to\file`, `path\to\file`},
		{"asterisk preserved", "logs-*", "logs-*"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := EscapeESQLString(tc.input)
			if result != tc.expected {
				t.Errorf("EscapeESQLString(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGetNestedPath(t *testing.T) {
	t.Parallel()

	t.Run("single level path", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"name": "test",
		}

		val, ok := GetNestedPath(data, "name")
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if val != "test" {
			t.Errorf("expected 'test', got %v", val)
		}
	})

	t.Run("nested path", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"service": map[string]interface{}{
				"name": "my-service",
			},
		}

		val, ok := GetNestedPath(data, "service.name")
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if val != "my-service" {
			t.Errorf("expected 'my-service', got %v", val)
		}
	})

	t.Run("deeply nested path", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"resource": map[string]interface{}{
				"attributes": map[string]interface{}{
					"service": map[string]interface{}{
						"name": "deep-service",
					},
				},
			},
		}

		val, ok := GetNestedPath(data, "resource.attributes.service.name")
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if val != "deep-service" {
			t.Errorf("expected 'deep-service', got %v", val)
		}
	})

	t.Run("nil data returns false", func(t *testing.T) {
		t.Parallel()

		_, ok := GetNestedPath(nil, "some.path")
		if ok {
			t.Error("expected ok to be false for nil data")
		}
	})

	t.Run("empty path returns false", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{"key": "value"}
		_, ok := GetNestedPath(data, "")
		if ok {
			t.Error("expected ok to be false for empty path")
		}
	})

	t.Run("missing key returns false", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"existing": "value",
		}

		_, ok := GetNestedPath(data, "missing")
		if ok {
			t.Error("expected ok to be false for missing key")
		}
	})

	t.Run("path through non-map value returns false", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"service": "not-a-map",
		}

		_, ok := GetNestedPath(data, "service.name")
		if ok {
			t.Error("expected ok to be false when path goes through non-map")
		}
	})

	t.Run("returns non-string values", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"count":   42.0,
			"enabled": true,
			"tags":    []string{"a", "b"},
		}

		val, ok := GetNestedPath(data, "count")
		if !ok || val != 42.0 {
			t.Errorf("expected 42.0, got %v", val)
		}

		val, ok = GetNestedPath(data, "enabled")
		if !ok || val != true {
			t.Errorf("expected true, got %v", val)
		}
	})
}

func TestGetNestedParts(t *testing.T) {
	t.Parallel()

	t.Run("retrieves value by parts", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": "value",
				},
			},
		}

		val, ok := GetNestedParts(data, "a", "b", "c")
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if val != "value" {
			t.Errorf("expected 'value', got %v", val)
		}
	})

	t.Run("nil data returns false", func(t *testing.T) {
		t.Parallel()

		_, ok := GetNestedParts(nil, "a", "b")
		if ok {
			t.Error("expected ok to be false")
		}
	})

	t.Run("empty parts returns false", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{"key": "value"}
		_, ok := GetNestedParts(data)
		if ok {
			t.Error("expected ok to be false for empty parts")
		}
	})

	t.Run("single part", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{"key": "value"}
		val, ok := GetNestedParts(data, "key")
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if val != "value" {
			t.Errorf("expected 'value', got %v", val)
		}
	})
}

func TestGetNestedString(t *testing.T) {
	t.Parallel()

	t.Run("returns string value", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"service": map[string]interface{}{
				"name": "my-service",
			},
		}

		result := GetNestedString(data, "service.name")
		if result != "my-service" {
			t.Errorf("expected 'my-service', got %q", result)
		}
	})

	t.Run("returns empty for missing path", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{}
		result := GetNestedString(data, "missing.path")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("returns empty for non-string value", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"count": 42.0,
		}

		result := GetNestedString(data, "count")
		if result != "" {
			t.Errorf("expected empty string for non-string, got %q", result)
		}
	})

	t.Run("returns empty for nil data", func(t *testing.T) {
		t.Parallel()

		result := GetNestedString(nil, "any.path")
		if result != "" {
			t.Errorf("expected empty string for nil data, got %q", result)
		}
	})
}

func TestGetNestedFloat(t *testing.T) {
	t.Parallel()

	t.Run("returns float value", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"metrics": map[string]interface{}{
				"cpu": 0.85,
			},
		}

		result := GetNestedFloat(data, "metrics.cpu")
		if result != 0.85 {
			t.Errorf("expected 0.85, got %v", result)
		}
	})

	t.Run("returns 0 for missing path", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{}
		result := GetNestedFloat(data, "missing.path")
		if result != 0 {
			t.Errorf("expected 0, got %v", result)
		}
	})

	t.Run("returns 0 for non-float value", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"name": "not-a-number",
		}

		result := GetNestedFloat(data, "name")
		if result != 0 {
			t.Errorf("expected 0 for non-float, got %v", result)
		}
	})

	t.Run("returns 0 for nil data", func(t *testing.T) {
		t.Parallel()

		result := GetNestedFloat(nil, "any.path")
		if result != 0 {
			t.Errorf("expected 0 for nil data, got %v", result)
		}
	})
}

func TestSplitPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{"empty path", "", nil},
		{"single part", "name", []string{"name"}},
		{"two parts", "service.name", []string{"service", "name"}},
		{"multiple parts", "a.b.c.d", []string{"a", "b", "c", "d"}},
		{"trailing dot", "service.", []string{"service"}},
		{"leading dot", ".name", []string{"name"}},
		{"consecutive dots", "a..b", []string{"a", "b"}},
		{"only dots", "...", nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := splitPath(tc.path)

			if tc.expected == nil {
				if result != nil && len(result) > 0 {
					t.Errorf("splitPath(%q) = %v, want nil or empty", tc.path, result)
				}
				return
			}

			if len(result) != len(tc.expected) {
				t.Errorf("splitPath(%q) = %v (len %d), want %v (len %d)",
					tc.path, result, len(result), tc.expected, len(tc.expected))
				return
			}

			for i, part := range result {
				if part != tc.expected[i] {
					t.Errorf("splitPath(%q)[%d] = %q, want %q", tc.path, i, part, tc.expected[i])
				}
			}
		})
	}
}
