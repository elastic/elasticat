// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/tui"
	"golang.org/x/term"
)

type displayColumn struct {
	Field string
	Label string
	Width int
}

type tableRenderer struct {
	columns []displayColumn
	widths  []int
	sep     string
}

func newTableRenderer(kind signalKind, fieldsOverride []string) *tableRenderer {
	totalWidth := detectTerminalWidth()
	cols := columnsForKind(kind, fieldsOverride)
	return &tableRenderer{
		columns: cols,
		widths:  computeColumnWidths(cols, totalWidth),
		sep:     " ",
	}
}

func columnsForKind(kind signalKind, fieldsOverride []string) []displayColumn {
	// If user provided an explicit field list, build columns directly from it.
	if len(fieldsOverride) > 0 {
		cols := make([]displayColumn, 0, len(fieldsOverride))
		for _, f := range fieldsOverride {
			if f == "" {
				continue
			}
			cols = append(cols, displayColumn{
				Field: f,
				Label: f,
				Width: 0, // flex
			})
		}
		return cols
	}
	// Otherwise, use TUI defaults for the signal kind.
	cols := []displayColumn{}
	for _, field := range tui.DefaultFields(kind.signalType()) {
		if !field.Selected {
			continue
		}
		cols = append(cols, displayColumn{
			Field: field.Name,
			Label: field.Label,
			Width: field.Width,
		})
	}
	return cols
}

func computeColumnWidths(columns []displayColumn, totalWidth int) []int {
	if totalWidth <= 0 {
		totalWidth = 80
	}
	widths := make([]int, len(columns))
	separators := len(columns) - 1
	if separators < 0 {
		separators = 0
	}
	fixed := 0
	flexIdx := -1
	for i, col := range columns {
		if col.Width > 0 {
			fixed += col.Width
		} else if flexIdx < 0 {
			flexIdx = i
		}
	}
	available := totalWidth - fixed - separators
	if available < 10 {
		available = 10
	}
	for i, col := range columns {
		if col.Width > 0 {
			widths[i] = col.Width
		} else if i == flexIdx {
			widths[i] = available
		} else {
			widths[i] = 10
		}
	}
	return widths
}

func detectTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	if env := os.Getenv("COLUMNS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil && val > 0 {
			return val
		}
	}
	return 80
}

func (t *tableRenderer) RenderHeader() {
	if len(t.columns) == 0 {
		return
	}
	parts := make([]string, len(t.columns))
	for i, col := range t.columns {
		parts[i] = padOrTruncate(col.Label, t.widths[i])
	}
	fmt.Println(strings.Join(parts, t.sep))
}

func (t *tableRenderer) RenderRows(entries []es.LogEntry) {
	if len(t.columns) == 0 {
		return
	}
	for _, entry := range entries {
		parts := make([]string, len(t.columns))
		for i, col := range t.columns {
			value := entry.GetFieldValue(col.Field)
			value = strings.ReplaceAll(value, "\n", " ")
			value = strings.ReplaceAll(value, "\r", " ")
			parts[i] = padOrTruncate(value, t.widths[i])
		}
		fmt.Println(strings.Join(parts, t.sep))
	}
}

func padOrTruncate(value string, width int) string {
	if width <= len(value) {
		if width <= 0 {
			return value
		}
		return value[:width]
	}
	return value + strings.Repeat(" ", width-len(value))
}
