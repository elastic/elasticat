// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

// getContentHeight returns the available height for main content
// accounting for title, status bar, help bar, and optional compact detail
func (m Model) getContentHeight(includeCompactDetail bool) int {
	const titleHeaderHeight = 1
	const newlines = 4 // Newlines between sections

	fixedHeight := titleHeaderHeight + statusBarHeight + helpBarHeight + layoutPadding + newlines
	if includeCompactDetail {
		fixedHeight += compactDetailHeight
	}

	contentHeight := m.height - fixedHeight
	if contentHeight < 3 {
		contentHeight = 3
	}
	return contentHeight
}

// getFullScreenHeight returns height for full-screen views (detail, fields, etc.)
// These views don't have compact detail but need extra space for their own headers
func (m Model) getFullScreenHeight() int {
	const titleHeaderHeight = 1
	const extraPadding = 2 // Extra padding for full-screen views

	fixedHeight := titleHeaderHeight + statusBarHeight + helpBarHeight + layoutPadding + extraPadding

	contentHeight := m.height - fixedHeight
	if contentHeight < 3 {
		contentHeight = 3
	}
	return contentHeight
}

// calcVisibleRange calculates the start and end indices for a centered scrolling list.
// It accounts for header rows and border space (subtracts 4 from viewHeight).
// Returns startIdx and endIdx for slicing the list.
func calcVisibleRange(cursor, listLen, viewHeight int) (startIdx, endIdx int) {
	contentHeight := viewHeight - 4 // Account for borders and header
	if contentHeight < 3 {
		contentHeight = 3
	}

	startIdx = cursor - contentHeight/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx = startIdx + contentHeight
	if endIdx > listLen {
		endIdx = listLen
		startIdx = endIdx - contentHeight
		if startIdx < 0 {
			startIdx = 0
		}
	}
	return startIdx, endIdx
}
