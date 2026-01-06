// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package shared

import "errors"

// ESQLUnknownIndexError indicates an ES|QL query failed because the FROM index pattern
// matched no existing indices/data streams (ES returns a 400 verification_exception).
//
// We treat this as an "empty state" condition in higher layers.
type ESQLUnknownIndexError struct {
	Index  string // the unknown index pattern, e.g. "traces-*"
	Status string // HTTP status string, e.g. "400 Bad Request"
	Body   string // raw ES error body (best-effort)
}

func (e *ESQLUnknownIndexError) Error() string {
	if e == nil {
		return "ES|QL unknown index"
	}
	if e.Status != "" {
		return "ES|QL unknown index: " + e.Index + " (" + e.Status + ")"
	}
	return "ES|QL unknown index: " + e.Index
}

// IsESQLUnknownIndex returns (indexPattern, true) if err represents an ES|QL
// "Unknown index [pattern]" verification error.
func IsESQLUnknownIndex(err error) (string, bool) {
	var u *ESQLUnknownIndexError
	if errors.As(err, &u) && u != nil {
		return u.Index, true
	}
	return "", false
}

// ESQLUnsupportedFieldTypeError indicates an ES|QL query failed because it referenced
// a field with a type not supported by ES|QL in that context (e.g., histogram).
//
// This commonly happens when the UI tries to apply `field IS NOT NULL` filters over
// histogram metric fields. Higher layers should treat this as an empty state.
type ESQLUnsupportedFieldTypeError struct {
	Field  string // full field name, e.g. "metrics.transaction.duration.histogram"
	Type   string // unsupported ES type, e.g. "histogram"
	Status string // HTTP status string, e.g. "400 Bad Request"
	Body   string // raw ES error body (best-effort)
}

func (e *ESQLUnsupportedFieldTypeError) Error() string {
	if e == nil {
		return "ES|QL unsupported field type"
	}
	if e.Type != "" && e.Field != "" {
		return "ES|QL unsupported field type: " + e.Field + " (" + e.Type + ")"
	}
	if e.Field != "" {
		return "ES|QL unsupported field type: " + e.Field
	}
	return "ES|QL unsupported field type"
}

// IsESQLUnsupportedFieldType returns (field, type, true) if err represents an ES|QL
// verification error about using an unsupported field type (e.g. histogram).
func IsESQLUnsupportedFieldType(err error) (string, string, bool) {
	var u *ESQLUnsupportedFieldTypeError
	if errors.As(err, &u) && u != nil {
		return u.Field, u.Type, true
	}
	return "", "", false
}

// IsESQLEmptyStateError returns true if the error represents an expected
// empty-state condition (no data yet, unsupported field type, etc.)
// rather than an actual failure. Callers can use this to return empty
// results instead of surfacing errors to the UI.
func IsESQLEmptyStateError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := IsESQLUnknownIndex(err); ok {
		return true
	}
	if _, _, ok := IsESQLUnsupportedFieldType(err); ok {
		return true
	}
	return false
}

// EmptyESQLResult returns an empty result suitable for returning when
// an empty-state error is detected.
func EmptyESQLResult() *ESQLResult {
	return &ESQLResult{Columns: []ESQLColumn{}, Values: [][]interface{}{}}
}
