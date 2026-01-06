// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import "testing"

func TestListNav(t *testing.T) {
	t.Parallel()

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
		// Up/k navigation
		{name: "up clamps at zero", args: args{cursor: 0, listLen: 5, key: "up"}, expected: 0},
		{name: "up decrements mid-list", args: args{cursor: 3, listLen: 5, key: "up"}, expected: 2},
		{name: "k decrements", args: args{cursor: 3, listLen: 5, key: "k"}, expected: 2},

		// Down/j navigation
		{name: "down clamps at end", args: args{cursor: 4, listLen: 5, key: "down"}, expected: 4},
		{name: "down increments mid-list", args: args{cursor: 2, listLen: 5, key: "down"}, expected: 3},
		{name: "j increments", args: args{cursor: 1, listLen: 3, key: "j"}, expected: 2},

		// Home/g navigation
		{name: "home goes zero", args: args{cursor: 2, listLen: 5, key: "home"}, expected: 0},
		{name: "g goes zero", args: args{cursor: 2, listLen: 5, key: "g"}, expected: 0},
		{name: "home from zero stays zero", args: args{cursor: 0, listLen: 5, key: "home"}, expected: 0},

		// End/G navigation
		{name: "end goes last", args: args{cursor: 1, listLen: 4, key: "end"}, expected: 3},
		{name: "G goes last", args: args{cursor: 0, listLen: 2, key: "G"}, expected: 1},
		{name: "end from last stays last", args: args{cursor: 4, listLen: 5, key: "end"}, expected: 4},

		// Page up/down navigation
		{name: "pgup clamps low", args: args{cursor: 3, listLen: 5, key: "pgup"}, expected: 0},
		{name: "pgup moves 10 from middle", args: args{cursor: 15, listLen: 20, key: "pgup"}, expected: 5},
		{name: "pgdown clamps high", args: args{cursor: 0, listLen: 5, key: "pgdown"}, expected: 4},
		{name: "pgdown moves 10 from middle", args: args{cursor: 5, listLen: 20, key: "pgdown"}, expected: 15},

		// Edge cases
		{name: "unknown key returns -1", args: args{cursor: 2, listLen: 5, key: "x"}, expected: -1},
		{name: "empty string key returns -1", args: args{cursor: 2, listLen: 5, key: ""}, expected: -1},
		{name: "empty list end returns 0", args: args{cursor: 0, listLen: 0, key: "end"}, expected: 0},
		{name: "empty list G returns 0", args: args{cursor: 0, listLen: 0, key: "G"}, expected: 0},
		// Note: pgdown on empty list returns cursor+10 (unclamped) - callers must validate
		{name: "empty list pgdown unclamped", args: args{cursor: 0, listLen: 0, key: "pgdown"}, expected: 10},
		{name: "single item list down clamps", args: args{cursor: 0, listLen: 1, key: "down"}, expected: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := listNav(tc.args.cursor, tc.args.listLen, tc.args.key)
			if got != tc.expected {
				t.Errorf("listNav(cursor=%d, listLen=%d, key=%q) = %d, want %d",
					tc.args.cursor, tc.args.listLen, tc.args.key, got, tc.expected)
			}
		})
	}
}

func TestIsNavKey(t *testing.T) {
	t.Parallel()

	t.Run("navigation keys return true", func(t *testing.T) {
		t.Parallel()
		navKeys := []string{"up", "k", "down", "j", "home", "g", "end", "G", "pgup", "pgdown"}
		for _, key := range navKeys {
			if !isNavKey(key) {
				t.Errorf("isNavKey(%q) = false, want true", key)
			}
		}
	})

	t.Run("non-navigation keys return false", func(t *testing.T) {
		t.Parallel()
		nonNav := []string{"", "x", "enter", "tab", "ctrl+c", "space", "a", "1", "esc", "q"}
		for _, key := range nonNav {
			if isNavKey(key) {
				t.Errorf("isNavKey(%q) = true, want false", key)
			}
		}
	})
}

