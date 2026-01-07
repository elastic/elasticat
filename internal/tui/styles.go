// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#5A5A5A")
	successColor   = lipgloss.Color("#04B575")
	warningColor   = lipgloss.Color("#FFCC00")
	errorColor     = lipgloss.Color("#FF5F56")
	infoColor      = lipgloss.Color("#61AFEF")
	debugColor     = lipgloss.Color("#6C757D")
	traceColor     = lipgloss.Color("#888888")
	fgColor        = lipgloss.Color("#E0E0E0")
	mutedColor     = lipgloss.Color("#6C757D")
)

// Styles
var (
	// App frame
	AppStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1).
			MarginBottom(1)

	// Title header with cat ASCII art
	TitleHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				Padding(0, 1)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(fgColor).
			Background(lipgloss.Color("#333333")).
			Padding(0, 1)

	StatusKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	StatusValueStyle = lipgloss.NewStyle().
				Foreground(fgColor)

	// Log list
	LogListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(0, 1)

	// Log entry styles
	LogEntryStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	SelectedLogStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#4A4A7A")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				PaddingLeft(1)

	// Cell style for selected rows: no padding so column alignment stays fixed
	SelectedCellStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#4A4A7A")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	// Column header row
	HeaderRowStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(secondaryColor).
			PaddingLeft(1).
			MarginBottom(0)

	// Timestamp (width controlled by column layout, not style)
	TimestampStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Service name (width controlled by column layout, not style)
	ServiceStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Bold(true)

	// Resource (OTel resource attributes, width controlled by column layout)
	ResourceStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Log message
	MessageStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	// Search input
	SearchStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1).
			MarginTop(1)

	SearchPromptStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	// Detail panel
	DetailStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1).
			MarginTop(1)

	DetailKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	DetailValueStyle = lipgloss.NewStyle().
				Foreground(fgColor)

	DetailMutedStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	// Compact detail panel (bottom of screen)
	CompactDetailStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(secondaryColor).
				Padding(0, 1)

	// Query overlay (floating window)
	QueryOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(primaryColor).
				Padding(1, 2).
				Background(lipgloss.Color("#1a1a2e"))

	QueryHeaderStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	QueryMethodStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	QueryPathStyle = lipgloss.NewStyle().
			Foreground(infoColor)

	QueryBodyStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	HelpOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(secondaryColor).
				Padding(1, 2)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Loading style
	LoadingStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)
)

// LevelStyle returns the appropriate style for a log level
// Width is controlled by column layout, not by this style
func LevelStyle(level string) lipgloss.Style {
	base := lipgloss.NewStyle()

	switch level {
	case "ERROR", "FATAL", "error", "fatal":
		return base.Foreground(lipgloss.Color("#FFFFFF")).Background(errorColor).Bold(true)
	case "WARN", "WARNING", "warn", "warning":
		return base.Foreground(lipgloss.Color("#000000")).Background(warningColor)
	case "INFO", "info":
		return base.Foreground(lipgloss.Color("#FFFFFF")).Background(successColor)
	case "DEBUG", "debug":
		return base.Foreground(fgColor).Background(debugColor)
	case "TRACE", "trace":
		return base.Foreground(fgColor).Background(traceColor)
	default:
		return base.Foreground(fgColor).Background(secondaryColor)
	}
}

// TruncateWithEllipsis truncates a string and adds ellipsis if needed
// Text formatting functions moved to formatting_text.go:
// - TruncateWithEllipsis()
// - PadOrTruncate()
// - PadLeft()

// HighlightStyle is used for search match highlighting
var HighlightStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#FFD700")).
	Foreground(lipgloss.Color("#000000")).
	Bold(true)

// SparklineStyle is used for metric chart rendering
var SparklineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#61AFEF"))

// ExtractWithHighlight extracts a substring containing the search match and returns
// the extracted text along with the start/end positions of the match within it.
// If no match is found, returns the original text truncated normally.
// The match is positioned within the first ~15 chars of the visible text.
func ExtractWithHighlight(text, search string, maxLen int) (result string, matchStart, matchEnd int) {
	if search == "" || maxLen <= 0 {
		return TruncateWithEllipsis(text, maxLen), -1, -1
	}

	// Case-insensitive search
	textLower := strings.ToLower(text)
	searchLower := strings.ToLower(search)

	matchIdx := strings.Index(textLower, searchLower)
	if matchIdx == -1 {
		return TruncateWithEllipsis(text, maxLen), -1, -1
	}

	searchLen := len(search)
	textLen := len(text)

	// Target: position match to start within first 15 chars of visible area
	targetMatchPos := 12 // Where we want the match to appear in the output
	if targetMatchPos > maxLen/2 {
		targetMatchPos = maxLen / 2
	}

	// Calculate extraction window
	extractStart := matchIdx - targetMatchPos
	if extractStart < 0 {
		extractStart = 0
	}

	// Determine if we need prefix ellipsis
	needPrefixEllipsis := extractStart > 0
	if needPrefixEllipsis {
		extractStart += 3 // Make room for "..."
	}

	// Calculate how much text we can show
	availableLen := maxLen
	if needPrefixEllipsis {
		availableLen -= 3
	}

	extractEnd := extractStart + availableLen
	needSuffixEllipsis := extractEnd < textLen
	if needSuffixEllipsis {
		extractEnd = extractStart + availableLen - 3 // Make room for "..."
	}

	if extractEnd > textLen {
		extractEnd = textLen
		needSuffixEllipsis = false
	}

	// Build the result
	var sb strings.Builder
	if needPrefixEllipsis {
		sb.WriteString("...")
	}

	extracted := text[extractStart:extractEnd]
	sb.WriteString(extracted)

	if needSuffixEllipsis {
		sb.WriteString("...")
	}

	result = sb.String()

	// Calculate match position in result
	matchStart = matchIdx - extractStart
	if needPrefixEllipsis {
		matchStart += 3
	}
	matchEnd = matchStart + searchLen

	// Bounds check
	if matchStart < 0 {
		matchStart = 0
	}
	if matchEnd > len(result) {
		matchEnd = len(result)
	}

	return result, matchStart, matchEnd
}

// RenderWithHighlight renders text with the matched portion highlighted
func RenderWithHighlight(text string, matchStart, matchEnd int, baseStyle lipgloss.Style) string {
	if matchStart < 0 || matchEnd <= matchStart || matchStart >= len(text) {
		return baseStyle.Render(text)
	}

	// Clamp bounds
	if matchEnd > len(text) {
		matchEnd = len(text)
	}

	before := text[:matchStart]
	match := text[matchStart:matchEnd]
	after := text[matchEnd:]

	return baseStyle.Render(before) + HighlightStyle.Render(match) + baseStyle.Render(after)
}
