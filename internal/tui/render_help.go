// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"sort"
	"strings"
)

// renderHelpOverlay renders the full hotkeys overlay (scrollable).
func (m *Model) renderHelpOverlay() string {
	var b strings.Builder
	currentGroup := ""
	bindings := m.FullBindings()
	sort.SliceStable(bindings, func(i, j int) bool {
		if bindings[i].Group == bindings[j].Group {
			return bindings[i].Label < bindings[j].Label
		}
		return bindings[i].Group < bindings[j].Group
	})

	for _, kb := range bindings {
		if kb.Kind != KeyKindFull && kb.Kind != KeyKindQuick {
			continue
		}
		if kb.Group != "" && kb.Group != currentGroup {
			if currentGroup != "" {
				b.WriteString("\n")
			}
			b.WriteString(DetailKeyStyle.Render(kb.Group))
			b.WriteString("\n")
			currentGroup = kb.Group
		}
		b.WriteString("  ")
		b.WriteString(HelpKeyStyle.Render(strings.Join(kb.Keys, "/")))
		b.WriteString(" ")
		b.WriteString(HelpDescStyle.Render(kb.Label))
		b.WriteString("\n")
	}

	content := b.String()
	if content == "" {
		content = "No help available"
	}

	// For now, render without scrolling to debug
	return HelpOverlayStyle.Render(content)
}
