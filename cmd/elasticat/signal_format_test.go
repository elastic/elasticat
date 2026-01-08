// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

func TestColumnsForKind(t *testing.T) {
	t.Parallel()

	t.Run("with field override", func(t *testing.T) {
		t.Parallel()

		fields := []string{"field1", "field2", "field3"}
		cols := columnsForKind(signalKindLogs, fields)

		if len(cols) != 3 {
			t.Fatalf("expected 3 columns, got %d", len(cols))
		}

		for i, col := range cols {
			if col.Field != fields[i] {
				t.Errorf("column %d: expected Field %q, got %q", i, fields[i], col.Field)
			}
			if col.Label != fields[i] {
				t.Errorf("column %d: expected Label %q, got %q", i, fields[i], col.Label)
			}
			if col.Width != 0 {
				t.Errorf("column %d: expected Width 0 (flex), got %d", i, col.Width)
			}
		}
	})

	t.Run("skips empty field names in override", func(t *testing.T) {
		t.Parallel()

		fields := []string{"field1", "", "field2", ""}
		cols := columnsForKind(signalKindLogs, fields)

		if len(cols) != 2 {
			t.Fatalf("expected 2 columns (empty strings skipped), got %d", len(cols))
		}
		if cols[0].Field != "field1" {
			t.Errorf("expected first field 'field1', got %q", cols[0].Field)
		}
		if cols[1].Field != "field2" {
			t.Errorf("expected second field 'field2', got %q", cols[1].Field)
		}
	})

	t.Run("uses default fields for logs when no override", func(t *testing.T) {
		t.Parallel()

		cols := columnsForKind(signalKindLogs, nil)

		if len(cols) == 0 {
			t.Fatal("expected default columns for logs")
		}
		// Should have timestamp field
		hasTimestamp := false
		for _, col := range cols {
			if col.Field == "@timestamp" {
				hasTimestamp = true
				break
			}
		}
		if !hasTimestamp {
			t.Error("expected @timestamp in default columns")
		}
	})

	t.Run("uses default fields for traces when no override", func(t *testing.T) {
		t.Parallel()

		cols := columnsForKind(signalKindTraces, nil)

		if len(cols) == 0 {
			t.Fatal("expected default columns for traces")
		}
	})

	t.Run("uses default fields for metrics when no override", func(t *testing.T) {
		t.Parallel()

		cols := columnsForKind(signalKindMetrics, nil)

		if len(cols) == 0 {
			t.Fatal("expected default columns for metrics")
		}
	})

	t.Run("empty override uses defaults", func(t *testing.T) {
		t.Parallel()

		cols := columnsForKind(signalKindLogs, []string{})

		if len(cols) == 0 {
			t.Fatal("expected default columns for empty override")
		}
	})
}

func TestComputeColumnWidths(t *testing.T) {
	t.Parallel()

	t.Run("all fixed width columns", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "a", Width: 10},
			{Field: "b", Width: 20},
			{Field: "c", Width: 15},
		}

		widths := computeColumnWidths(cols, 100)

		if widths[0] != 10 {
			t.Errorf("expected width[0]=10, got %d", widths[0])
		}
		if widths[1] != 20 {
			t.Errorf("expected width[1]=20, got %d", widths[1])
		}
		if widths[2] != 15 {
			t.Errorf("expected width[2]=15, got %d", widths[2])
		}
	})

	t.Run("one flex column gets remaining space", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "a", Width: 10},
			{Field: "b", Width: 0}, // flex
			{Field: "c", Width: 10},
		}

		widths := computeColumnWidths(cols, 100)

		// Total = 100, fixed = 20, separators = 2, available = 78
		if widths[0] != 10 {
			t.Errorf("expected width[0]=10, got %d", widths[0])
		}
		if widths[1] != 78 {
			t.Errorf("expected width[1]=78 (flex), got %d", widths[1])
		}
		if widths[2] != 10 {
			t.Errorf("expected width[2]=10, got %d", widths[2])
		}
	})

	t.Run("first flex column gets space, others get default", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "a", Width: 0}, // flex - gets available space
			{Field: "b", Width: 0}, // second flex - gets default 10
		}

		widths := computeColumnWidths(cols, 100)

		// First flex column gets the available space
		// Total = 100, fixed = 0, separators = 1, available = 99
		if widths[0] != 99 {
			t.Errorf("expected width[0]=99 (first flex), got %d", widths[0])
		}
		if widths[1] != 10 {
			t.Errorf("expected width[1]=10 (second flex gets default), got %d", widths[1])
		}
	})

	t.Run("handles zero total width", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "a", Width: 10},
			{Field: "b", Width: 0},
		}

		widths := computeColumnWidths(cols, 0)

		// Should default to 80 width
		// Total = 80, fixed = 10, separators = 1, available = 69
		if widths[0] != 10 {
			t.Errorf("expected width[0]=10, got %d", widths[0])
		}
		if widths[1] != 69 {
			t.Errorf("expected width[1]=69, got %d", widths[1])
		}
	})

	t.Run("handles negative total width", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "a", Width: 0},
		}

		widths := computeColumnWidths(cols, -10)

		// Should default to 80 width
		if widths[0] != 80 {
			t.Errorf("expected width[0]=80, got %d", widths[0])
		}
	})

	t.Run("minimum available space is 10", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "a", Width: 100}, // More than total
			{Field: "b", Width: 0},   // flex
		}

		widths := computeColumnWidths(cols, 50)

		// Fixed exceeds total, but available should be at least 10
		if widths[1] < 10 {
			t.Errorf("expected width[1]>=10, got %d", widths[1])
		}
	})

	t.Run("empty columns", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{}
		widths := computeColumnWidths(cols, 100)

		if len(widths) != 0 {
			t.Errorf("expected 0 widths for empty columns, got %d", len(widths))
		}
	})

	t.Run("single column", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "only", Width: 0},
		}

		widths := computeColumnWidths(cols, 100)

		// No separators for single column, all available space
		if widths[0] != 100 {
			t.Errorf("expected width[0]=100, got %d", widths[0])
		}
	})
}

func TestPadOrTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		width    int
		expected string
	}{
		{"pad short string", "hi", 5, "hi   "},
		{"exact length", "hello", 5, "hello"},
		{"truncate long string", "hello world", 5, "hello"},
		{"empty string pads", "", 5, "     "},
		{"zero width returns original", "test", 0, "test"},
		{"negative width returns original", "test", -5, "test"},
		{"single char pad", "a", 3, "a  "},
		{"single char truncate", "abc", 1, "a"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := padOrTruncate(tc.value, tc.width)
			if result != tc.expected {
				t.Errorf("padOrTruncate(%q, %d) = %q, want %q", tc.value, tc.width, result, tc.expected)
			}
		})
	}
}

func TestDisplayColumn(t *testing.T) {
	t.Parallel()

	t.Run("struct initialization", func(t *testing.T) {
		t.Parallel()

		col := displayColumn{
			Field: "service.name",
			Label: "SERVICE",
			Width: 20,
		}

		if col.Field != "service.name" {
			t.Errorf("expected Field 'service.name', got %q", col.Field)
		}
		if col.Label != "SERVICE" {
			t.Errorf("expected Label 'SERVICE', got %q", col.Label)
		}
		if col.Width != 20 {
			t.Errorf("expected Width 20, got %d", col.Width)
		}
	})

	t.Run("zero values", func(t *testing.T) {
		t.Parallel()

		col := displayColumn{}

		if col.Field != "" {
			t.Errorf("expected empty Field, got %q", col.Field)
		}
		if col.Width != 0 {
			t.Errorf("expected Width 0, got %d", col.Width)
		}
	})
}

func TestTableRenderer(t *testing.T) {
	t.Parallel()

	t.Run("struct initialization", func(t *testing.T) {
		t.Parallel()

		cols := []displayColumn{
			{Field: "a", Label: "A", Width: 10},
		}
		renderer := &tableRenderer{
			columns: cols,
			widths:  []int{10},
			sep:     " | ",
		}

		if len(renderer.columns) != 1 {
			t.Errorf("expected 1 column, got %d", len(renderer.columns))
		}
		if renderer.sep != " | " {
			t.Errorf("expected separator ' | ', got %q", renderer.sep)
		}
	})
}

func TestSignalKindSignalType(t *testing.T) {
	t.Parallel()

	t.Run("logs kind", func(t *testing.T) {
		t.Parallel()

		kind := signalKindLogs
		if kind.signalType().String() != "Logs" {
			t.Errorf("expected 'Logs', got %q", kind.signalType().String())
		}
	})

	t.Run("traces kind", func(t *testing.T) {
		t.Parallel()

		kind := signalKindTraces
		if kind.signalType().String() != "Traces" {
			t.Errorf("expected 'Traces', got %q", kind.signalType().String())
		}
	})

	t.Run("metrics kind", func(t *testing.T) {
		t.Parallel()

		kind := signalKindMetrics
		if kind.signalType().String() != "Metrics" {
			t.Errorf("expected 'Metrics', got %q", kind.signalType().String())
		}
	})
}

func TestSignalKindDefaultIndex(t *testing.T) {
	t.Parallel()

	t.Run("logs default index", func(t *testing.T) {
		t.Parallel()

		kind := signalKindLogs
		if kind.defaultIndex() != "logs-*" {
			t.Errorf("expected 'logs-*', got %q", kind.defaultIndex())
		}
	})

	t.Run("traces default index", func(t *testing.T) {
		t.Parallel()

		kind := signalKindTraces
		if kind.defaultIndex() != "traces-*" {
			t.Errorf("expected 'traces-*', got %q", kind.defaultIndex())
		}
	})

	t.Run("metrics default index", func(t *testing.T) {
		t.Parallel()

		kind := signalKindMetrics
		if kind.defaultIndex() != "metrics-*" {
			t.Errorf("expected 'metrics-*', got %q", kind.defaultIndex())
		}
	})
}

func TestSignalKindProcessorEvent(t *testing.T) {
	t.Parallel()

	t.Run("traces returns transaction", func(t *testing.T) {
		t.Parallel()

		kind := signalKindTraces
		if kind.processorEvent() != "transaction" {
			t.Errorf("expected 'transaction', got %q", kind.processorEvent())
		}
	})

	t.Run("logs returns empty", func(t *testing.T) {
		t.Parallel()

		kind := signalKindLogs
		if kind.processorEvent() != "" {
			t.Errorf("expected empty string, got %q", kind.processorEvent())
		}
	})

	t.Run("metrics returns empty", func(t *testing.T) {
		t.Parallel()

		kind := signalKindMetrics
		if kind.processorEvent() != "" {
			t.Errorf("expected empty string, got %q", kind.processorEvent())
		}
	})
}
