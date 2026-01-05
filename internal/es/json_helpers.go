// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package es

import "encoding/json"

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
