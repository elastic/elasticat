package tui

import (
	"testing"

	"github.com/elastic/elasticat/internal/es"
)

func TestMoveSelectionClampsAndScrolls(t *testing.T) {
	m := Model{
		logs:          []es.LogEntry{{}, {}},
		selectedIndex: 0,
	}

	if moved := m.moveSelection(-1); moved {
		t.Fatalf("expected no movement when already at top")
	}
	if m.selectedIndex != 0 || m.userHasScrolled {
		t.Fatalf("unexpected state after no-op move: idx=%d scrolled=%v", m.selectedIndex, m.userHasScrolled)
	}

	if moved := m.moveSelection(1); !moved {
		t.Fatalf("expected movement downward")
	}
	if m.selectedIndex != 1 || !m.userHasScrolled {
		t.Fatalf("expected index 1 and scrolled after move, got idx=%d scrolled=%v", m.selectedIndex, m.userHasScrolled)
	}

	if moved := m.moveSelection(10); moved {
		t.Fatalf("expected no movement when already at bottom")
	}
	if m.selectedIndex != 1 {
		t.Fatalf("expected index to remain at bottom, got %d", m.selectedIndex)
	}
}

func TestNeedsSpanFetch(t *testing.T) {
	m := Model{
		lastFetchedTraceID: "trace-1",
		spans:              []es.LogEntry{{TraceID: "trace-1"}},
	}

	if m.needsSpanFetch("trace-1") {
		t.Fatalf("should not fetch when spans already loaded for the same trace")
	}

	m.spans = nil
	m.spansLoading = true
	if m.needsSpanFetch("trace-1") {
		t.Fatalf("should not fetch when a fetch is already in flight")
	}

	m.spansLoading = false
	if !m.needsSpanFetch("trace-2") {
		t.Fatalf("expected fetch for a new trace id")
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := truncateWithEllipsis("helloworld", 5); got != "he..." {
		t.Fatalf("truncateWithEllipsis unexpected: %q", got)
	}

	if got := singleLine("a\nb\r\nc"); got != "a b  c" {
		t.Fatalf("singleLine unexpected: %q", got)
	}

	kv := map[string]interface{}{"b": 2, "a": 1, "c": 3}
	if got := formatKVPreview(kv, 2, 100); got != "a=1, b=2, ..." {
		t.Fatalf("formatKVPreview unexpected: %q", got)
	}
}
