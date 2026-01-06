// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import "testing"

func TestQuickBindings_PrependsHelpKey(t *testing.T) {
	t.Parallel()

	m := Model{mode: viewLogs}
	b := m.QuickBindings()
	if len(b) == 0 {
		t.Fatalf("QuickBindings() returned empty")
	}
	if len(b[0].Keys) != 1 || b[0].Keys[0] != "?" {
		t.Fatalf("expected first quick binding to be help key ?, got %+v", b[0])
	}
	if b[0].Label != "help" {
		t.Fatalf("expected help binding label 'help', got %q", b[0].Label)
	}
}
