// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import "testing"

func TestListNav(t *testing.T) {
	type args struct {
		cursor  int
		listLen int
		key     string
	}
	tests := []struct {
		name     string
		args     args
		expected int
	}{
		{name: "up clamps at zero", args: args{cursor: 0, listLen: 5, key: "up"}, expected: 0},
		{name: "k decrements", args: args{cursor: 3, listLen: 5, key: "k"}, expected: 2},
		{name: "down clamps at end", args: args{cursor: 4, listLen: 5, key: "down"}, expected: 4},
		{name: "j increments", args: args{cursor: 1, listLen: 3, key: "j"}, expected: 2},
		{name: "home goes zero", args: args{cursor: 2, listLen: 5, key: "home"}, expected: 0},
		{name: "g goes zero", args: args{cursor: 2, listLen: 5, key: "g"}, expected: 0},
		{name: "end goes last", args: args{cursor: 1, listLen: 4, key: "end"}, expected: 3},
		{name: "G goes last", args: args{cursor: 0, listLen: 2, key: "G"}, expected: 1},
		{name: "pgup clamps low", args: args{cursor: 3, listLen: 5, key: "pgup"}, expected: 0},
		{name: "pgdown clamps high", args: args{cursor: 0, listLen: 5, key: "pgdown"}, expected: 4},
		{name: "unknown key returns -1", args: args{cursor: 2, listLen: 5, key: "x"}, expected: -1},
		{name: "empty list returns 0 on end", args: args{cursor: 0, listLen: 0, key: "end"}, expected: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := listNav(tc.args.cursor, tc.args.listLen, tc.args.key)
			if got != tc.expected {
				t.Fatalf("listNav(%v) = %d, want %d", tc.args, got, tc.expected)
			}
		})
	}
}

func TestIsNavKey(t *testing.T) {
	navKeys := []string{"up", "k", "down", "j", "home", "g", "end", "G", "pgup", "pgdown"}
	for _, key := range navKeys {
		if !isNavKey(key) {
			t.Fatalf("isNavKey(%q) = false, want true", key)
		}
	}

	nonNav := []string{"", "x", "enter", "tab", "ctrl+c"}
	for _, key := range nonNav {
		if isNavKey(key) {
			t.Fatalf("isNavKey(%q) = true, want false", key)
		}
	}
}

