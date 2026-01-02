package tui

// renderSearchInput renders the search input box
func (m Model) renderSearchInput() string {
	prompt := SearchPromptStyle.Render("Search: ")
	input := m.searchInput.View()
	return SearchStyle.Width(m.width - 4).Render(prompt + input)
}

// renderIndexInput renders the index input box
func (m Model) renderIndexInput() string {
	prompt := SearchPromptStyle.Render("Index: ")
	input := m.indexInput.View()
	return SearchStyle.Width(m.width - 4).Render(prompt + input)
}
