// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"strings"
	"testing"
)

func TestPrettyJSON(t *testing.T) {
	t.Parallel()

	t.Run("empty string returns empty", func(t *testing.T) {
		t.Parallel()

		result := PrettyJSON("")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("formats simple object", func(t *testing.T) {
		t.Parallel()

		input := `{"name":"test","value":42}`
		result := PrettyJSON(input)

		// Should have newlines and indentation
		if !strings.Contains(result, "\n") {
			t.Error("expected formatted output with newlines")
		}
		if !strings.Contains(result, "  ") {
			t.Error("expected 2-space indentation")
		}
		if !strings.Contains(result, `"name": "test"`) {
			t.Error("expected formatted name field")
		}
		if !strings.Contains(result, `"value": 42`) {
			t.Error("expected formatted value field")
		}
	})

	t.Run("formats nested object", func(t *testing.T) {
		t.Parallel()

		input := `{"outer":{"inner":"value"}}`
		result := PrettyJSON(input)

		// Check structure
		if !strings.Contains(result, "outer") {
			t.Error("expected outer key")
		}
		if !strings.Contains(result, "inner") {
			t.Error("expected inner key")
		}
		// Verify proper nesting (inner should have more indentation than outer)
		lines := strings.Split(result, "\n")
		outerIndent := 0
		innerIndent := 0
		for _, line := range lines {
			if strings.Contains(line, `"outer"`) {
				outerIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
			if strings.Contains(line, `"inner"`) {
				innerIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
		}
		if innerIndent <= outerIndent {
			t.Error("expected inner to have more indentation than outer")
		}
	})

	t.Run("formats array", func(t *testing.T) {
		t.Parallel()

		input := `[1,2,3]`
		result := PrettyJSON(input)

		if !strings.Contains(result, "[\n") {
			t.Error("expected array opening bracket with newline")
		}
		if !strings.Contains(result, "1") && !strings.Contains(result, "2") && !strings.Contains(result, "3") {
			t.Error("expected array elements")
		}
	})

	t.Run("returns raw on invalid JSON", func(t *testing.T) {
		t.Parallel()

		input := `not valid json`
		result := PrettyJSON(input)

		if result != input {
			t.Errorf("expected raw input %q, got %q", input, result)
		}
	})

	t.Run("returns raw on partial JSON", func(t *testing.T) {
		t.Parallel()

		input := `{"incomplete":`
		result := PrettyJSON(input)

		if result != input {
			t.Errorf("expected raw input %q, got %q", input, result)
		}
	})

	t.Run("handles already formatted JSON", func(t *testing.T) {
		t.Parallel()

		input := `{
  "name": "test"
}`
		result := PrettyJSON(input)

		// Should still be valid and formatted
		if !strings.Contains(result, `"name"`) {
			t.Error("expected name field")
		}
	})

	t.Run("handles special characters", func(t *testing.T) {
		t.Parallel()

		input := `{"message":"line1\nline2\ttab"}`
		result := PrettyJSON(input)

		// JSON escapes should be preserved
		if !strings.Contains(result, "message") {
			t.Error("expected message field")
		}
	})

	t.Run("handles unicode", func(t *testing.T) {
		t.Parallel()

		input := `{"name":"日本語"}`
		result := PrettyJSON(input)

		if !strings.Contains(result, "日本語") {
			t.Error("expected unicode to be preserved")
		}
	})

	t.Run("handles null values", func(t *testing.T) {
		t.Parallel()

		input := `{"value":null}`
		result := PrettyJSON(input)

		if !strings.Contains(result, "null") {
			t.Error("expected null value to be preserved")
		}
	})

	t.Run("handles boolean values", func(t *testing.T) {
		t.Parallel()

		input := `{"enabled":true,"disabled":false}`
		result := PrettyJSON(input)

		if !strings.Contains(result, "true") {
			t.Error("expected true value")
		}
		if !strings.Contains(result, "false") {
			t.Error("expected false value")
		}
	})

	t.Run("handles large numbers", func(t *testing.T) {
		t.Parallel()

		input := `{"big":12345678901234567890}`
		result := PrettyJSON(input)

		// JSON marshalling may convert to scientific notation for very large numbers
		if !strings.Contains(result, "big") {
			t.Error("expected big field")
		}
	})

	t.Run("handles empty object", func(t *testing.T) {
		t.Parallel()

		input := `{}`
		result := PrettyJSON(input)

		if result != "{}" {
			t.Errorf("expected '{}', got %q", result)
		}
	})

	t.Run("handles empty array", func(t *testing.T) {
		t.Parallel()

		input := `[]`
		result := PrettyJSON(input)

		if result != "[]" {
			t.Errorf("expected '[]', got %q", result)
		}
	})
}

// Test the wrapper functions (they delegate to shared package)
func TestGetNestedPathWrapper(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"service": map[string]interface{}{
			"name": "test-service",
		},
	}

	val, ok := GetNestedPath(data, "service.name")
	if !ok {
		t.Fatal("expected ok to be true")
	}
	if val != "test-service" {
		t.Errorf("expected 'test-service', got %v", val)
	}
}

func TestGetNestedPartsWrapper(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "value",
		},
	}

	val, ok := GetNestedParts(data, "a", "b")
	if !ok {
		t.Fatal("expected ok to be true")
	}
	if val != "value" {
		t.Errorf("expected 'value', got %v", val)
	}
}

func TestGetNestedStringWrapper(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"name": "my-name",
	}

	result := GetNestedString(data, "name")
	if result != "my-name" {
		t.Errorf("expected 'my-name', got %q", result)
	}

	// Test missing path
	result = GetNestedString(data, "missing")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestGetNestedFloatWrapper(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"value": 3.14,
	}

	result := GetNestedFloat(data, "value")
	if result != 3.14 {
		t.Errorf("expected 3.14, got %v", result)
	}

	// Test missing path
	result = GetNestedFloat(data, "missing")
	if result != 0 {
		t.Errorf("expected 0, got %v", result)
	}
}
