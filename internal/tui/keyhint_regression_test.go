// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"strings"
	"testing"
)

// This test prevents reintroducing hardcoded key-hint strings like "[j/k] Scroll" or "Press 'd'".
// The intent is: UI key hints must be derived from the centralized action/keyhint helpers.
func TestNoHardcodedKeyHintsInRenderers(t *testing.T) {
	t.Parallel()

	files := []string{
		"render_overlay.go",
		"render_metrics.go",
	}

	badSubstrings := []string{
		`Render("[`,
		`Press '`,
		`Press "`,
		`[j/k]`,
		`(a/d:`,
		`esc/q`,
	}

	for _, f := range files {
		path := f
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s := string(b)
		for _, bad := range badSubstrings {
			if strings.Contains(s, bad) {
				t.Fatalf("%s contains hardcoded key hint substring %q; use keyHint()/actionHint()/keysHint() helpers instead", path, bad)
			}
		}
	}
}
