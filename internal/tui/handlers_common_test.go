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

func TestPushView(t *testing.T) {
	t.Parallel()

	t.Run("pushes view onto stack and changes mode", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}

		m.pushView(viewDetail)

		if m.mode != viewDetail {
			t.Errorf("mode = %v, want %v", m.mode, viewDetail)
		}
		if len(m.viewStack) != 1 {
			t.Errorf("viewStack len = %d, want 1", len(m.viewStack))
		}
		if m.viewStack[0].Mode != viewLogs {
			t.Errorf("viewStack[0].Mode = %v, want %v", m.viewStack[0].Mode, viewLogs)
		}
	})

	t.Run("multiple pushes grow stack correctly", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}

		m.pushView(viewDetail)
		m.pushView(viewDetailJSON)
		m.pushView(viewHelp)

		if m.mode != viewHelp {
			t.Errorf("mode = %v, want %v", m.mode, viewHelp)
		}
		if len(m.viewStack) != 3 {
			t.Errorf("viewStack len = %d, want 3", len(m.viewStack))
		}
		// Stack should be: [viewLogs, viewDetail, viewDetailJSON]
		expected := []viewMode{viewLogs, viewDetail, viewDetailJSON}
		for i, exp := range expected {
			if m.viewStack[i].Mode != exp {
				t.Errorf("viewStack[%d].Mode = %v, want %v", i, m.viewStack[i].Mode, exp)
			}
		}
	})
}

func TestPopView(t *testing.T) {
	t.Parallel()

	t.Run("pops view from stack and restores mode", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}
		m.pushView(viewDetail)

		ok := m.popView()

		if !ok {
			t.Error("popView returned false, want true")
		}
		if m.mode != viewLogs {
			t.Errorf("mode = %v, want %v", m.mode, viewLogs)
		}
		if len(m.viewStack) != 0 {
			t.Errorf("viewStack len = %d, want 0", len(m.viewStack))
		}
	})

	t.Run("returns false on empty stack", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}

		ok := m.popView()

		if ok {
			t.Error("popView returned true on empty stack, want false")
		}
		if m.mode != viewLogs {
			t.Errorf("mode changed unexpectedly: got %v, want %v", m.mode, viewLogs)
		}
	})

	t.Run("multiple pops restore in correct order", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}
		m.pushView(viewDetail)
		m.pushView(viewDetailJSON)
		m.pushView(viewHelp)

		// Pop viewHelp -> viewDetailJSON
		m.popView()
		if m.mode != viewDetailJSON {
			t.Errorf("after first pop: mode = %v, want %v", m.mode, viewDetailJSON)
		}

		// Pop viewDetailJSON -> viewDetail
		m.popView()
		if m.mode != viewDetail {
			t.Errorf("after second pop: mode = %v, want %v", m.mode, viewDetail)
		}

		// Pop viewDetail -> viewLogs
		m.popView()
		if m.mode != viewLogs {
			t.Errorf("after third pop: mode = %v, want %v", m.mode, viewLogs)
		}

		// Fourth pop should fail (empty)
		ok := m.popView()
		if ok {
			t.Error("fourth popView returned true on empty stack")
		}
	})
}

func TestPeekViewStack(t *testing.T) {
	t.Parallel()

	t.Run("returns top of stack without modifying it", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}
		m.pushView(viewDetail)
		m.pushView(viewHelp)

		peeked := m.peekViewStack()

		if peeked != viewDetail {
			t.Errorf("peekViewStack = %v, want %v", peeked, viewDetail)
		}
		// Stack should be unchanged
		if len(m.viewStack) != 2 {
			t.Errorf("viewStack len = %d, want 2 (should not be modified)", len(m.viewStack))
		}
		if m.mode != viewHelp {
			t.Errorf("mode = %v, want %v (should not be modified)", m.mode, viewHelp)
		}
	})

	t.Run("returns current mode on empty stack", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewMetricsDashboard}

		peeked := m.peekViewStack()

		if peeked != viewMetricsDashboard {
			t.Errorf("peekViewStack on empty stack = %v, want current mode %v", peeked, viewMetricsDashboard)
		}
	})
}

func TestClearViewStack(t *testing.T) {
	t.Parallel()

	t.Run("clears non-empty stack", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}
		m.pushView(viewDetail)
		m.pushView(viewHelp)

		m.clearViewStack()

		if len(m.viewStack) != 0 {
			t.Errorf("viewStack len = %d, want 0", len(m.viewStack))
		}
		// Mode should not be changed by clear
		if m.mode != viewHelp {
			t.Errorf("mode = %v, want %v (should not be modified)", m.mode, viewHelp)
		}
	})

	t.Run("no-op on empty stack", func(t *testing.T) {
		t.Parallel()
		m := Model{mode: viewLogs}

		m.clearViewStack()

		if len(m.viewStack) != 0 {
			t.Errorf("viewStack len = %d, want 0", len(m.viewStack))
		}
	})
}
