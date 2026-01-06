// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"errors"
	"testing"
)

func TestIsESQLEmptyStateError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some random error"),
			expected: false,
		},
		{
			name: "unknown index error",
			err: &ESQLUnknownIndexError{
				Index:  "traces-*",
				Status: "400 Bad Request",
			},
			expected: true,
		},
		{
			name: "unsupported field type error",
			err: &ESQLUnsupportedFieldTypeError{
				Field:  "metrics.duration.histogram",
				Type:   "histogram",
				Status: "400 Bad Request",
			},
			expected: true,
		},
		{
			name:     "wrapped unknown index error",
			err:      wrapError(&ESQLUnknownIndexError{Index: "logs-*"}),
			expected: true,
		},
		{
			name:     "wrapped unsupported field type error",
			err:      wrapError(&ESQLUnsupportedFieldTypeError{Field: "foo", Type: "bar"}),
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsESQLEmptyStateError(tc.err)
			if result != tc.expected {
				t.Errorf("IsESQLEmptyStateError(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestEmptyESQLResult(t *testing.T) {
	result := EmptyESQLResult()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Columns) != 0 {
		t.Errorf("expected empty Columns, got %d", len(result.Columns))
	}
	if len(result.Values) != 0 {
		t.Errorf("expected empty Values, got %d", len(result.Values))
	}
}

// wrapError wraps an error to test that errors.As works through wrapping
func wrapError(err error) error {
	return wrappedError{underlying: err}
}

type wrappedError struct {
	underlying error
}

func (w wrappedError) Error() string {
	return "wrapped: " + w.underlying.Error()
}

func (w wrappedError) Unwrap() error {
	return w.underlying
}
