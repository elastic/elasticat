package tui

import "strings"

// PadLeft pads a string to the left to reach the specified width
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// TruncateWithEllipsis truncates a string to maxLen, adding "..." if truncated
func TruncateWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// PadOrTruncate ensures a string is exactly the given width, padding with spaces or truncating
func PadOrTruncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	if len(s) > width {
		if width <= 3 {
			return s[:width]
		}
		return s[:width-3] + "..."
	}
	// Pad with spaces
	return s + strings.Repeat(" ", width-len(s))
}
