// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package errfmt

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// FormatQueryError builds a detailed error including response status, body, and pretty query.
// It best-effort indents the provided query JSON; on failure it still includes the raw query.
func FormatQueryError(status string, body []byte, queryJSON []byte) error {
	var prettyQuery bytes.Buffer
	_ = json.Indent(&prettyQuery, queryJSON, "", "  ")
	if prettyQuery.Len() == 0 {
		prettyQuery.Write(queryJSON)
	}
	return fmt.Errorf("search failed: %s\nError: %s\n\nQuery:\n%s", status, string(body), prettyQuery.String())
}


