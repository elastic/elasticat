// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import "strings"

// renderHelpBar renders the quick help bar using the keymap.
func (m Model) renderHelpBar() string {
	var parts []string
	for _, kb := range m.QuickBindings() {
		parts = append(parts, HelpKeyStyle.Render(strings.Join(kb.Keys, "/"))+HelpDescStyle.Render(" "+kb.Label))
	}
	return HelpStyle.Render(strings.Join(parts, "  "))
}
