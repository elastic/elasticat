// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

// renderSearchInput renders the search input box
func (m Model) renderSearchInput() string {
	prompt := SearchPromptStyle.Render("Search: ")
	input := m.Components.SearchInput.View()
	return SearchStyle.Width(m.UI.Width - 4).Render(prompt + input)
}

// renderIndexInput renders the index input box
func (m Model) renderIndexInput() string {
	prompt := SearchPromptStyle.Render("Index: ")
	input := m.Components.IndexInput.View()
	return SearchStyle.Width(m.UI.Width - 4).Render(prompt + input)
}
