// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"time"
)

// formatClockTime returns zero-padded HH:MM:SS
func formatClockTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
}

// formatRelativeTime formats a time.Time as a human-readable relative time string
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 0 {
		diff = -diff
		// Future time
		if diff < time.Minute {
			return fmt.Sprintf("+%ds", int(diff.Seconds()))
		}
		if diff < time.Hour {
			return fmt.Sprintf("+%dm", int(diff.Minutes()))
		}
		return fmt.Sprintf("+%dh", int(diff.Hours()))
	}

	switch {
	case diff < time.Second:
		return "now"
	case diff < time.Minute:
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(diff.Hours()/(24*7)))
	case diff < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(diff.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(diff.Hours()/(24*365)))
	}
}

// formatFullTime returns full date and time in local time with seconds
func formatFullTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
