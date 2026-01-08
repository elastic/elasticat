// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

// KeyKind indicates whether a binding is part of the quick (always shown) or full (overlay) set.
type KeyKind int

const (
	KeyKindQuick KeyKind = iota
	KeyKindFull
)

// KeyBinding represents a key (or set of keys) and its label/group.
type KeyBinding struct {
	Keys  []string
	Label string
	Kind  KeyKind
	Group string
}

// HelpEnabled returns true if this view should show the help overlay.
// For compact views (e.g., small hotkey sets), this returns false.
func (m Model) HelpEnabled() bool {
	switch m.UI.Mode {
	case viewDetailJSON, viewErrorModal:
		return false
	}
	return true
}

// QuickBindings returns the bindings to show in the help bar (max quickLimit, prepend help when enabled).
func (m Model) QuickBindings() []KeyBinding {
	const quickLimit = 7
	bindings := filterByKind(m.ViewKeymap(), KeyKindQuick)

	if m.HelpEnabled() {
		bindings = append([]KeyBinding{ActionBinding(ActionHelp, KeyKindQuick, "Help")}, bindings...)
	}

	if len(bindings) > quickLimit {
		return bindings[:quickLimit]
	}
	return bindings
}

// FullBindings returns the full set of bindings for the view.
func (m Model) FullBindings() []KeyBinding {
	return m.ViewKeymap()
}

func filterByKind(bindings []KeyBinding, kind KeyKind) []KeyBinding {
	var out []KeyBinding
	for _, b := range bindings {
		if b.Kind == kind {
			out = append(out, b)
		}
	}
	return out
}

// === Common Binding Sets ===
// These helpers reduce duplication across view keymaps.

// GlobalBindings returns the standard global action bindings (Chat, SendToChat, Creds, OtelConfig).
// Use this for views where these actions work but aren't in the quick bar.
func GlobalBindings() []KeyBinding {
	return []KeyBinding{
		ActionBinding(ActionChat, KeyKindFull, "AI"),
		ActionBinding(ActionSendToChat, KeyKindFull, "AI"),
		ActionBinding(ActionCreds, KeyKindFull, "System"),
		ActionBinding(ActionOtelConfig, KeyKindFull, "System"),
	}
}

// GlobalBindingsWithQuit returns global bindings plus Quit.
// Use this for main views that should show the quit option.
func GlobalBindingsWithQuit() []KeyBinding {
	return []KeyBinding{
		ActionBinding(ActionChat, KeyKindFull, "AI"),
		ActionBinding(ActionSendToChat, KeyKindFull, "AI"),
		ActionBinding(ActionCreds, KeyKindFull, "System"),
		ActionBinding(ActionOtelConfig, KeyKindFull, "System"),
		ActionBinding(ActionQuit, KeyKindFull, "System"),
	}
}

// SystemBindings returns just OtelConfig and Quit bindings.
// Use this when Chat/Creds are already in quick bindings or not applicable.
func SystemBindings() []KeyBinding {
	return []KeyBinding{
		ActionBinding(ActionCreds, KeyKindFull, "System"),
		ActionBinding(ActionOtelConfig, KeyKindFull, "System"),
		ActionBinding(ActionQuit, KeyKindFull, "System"),
	}
}

// DetailGlobalBindings returns globals plus CopyOriginal for log detail views.
func DetailGlobalBindings() []KeyBinding {
	return []KeyBinding{
		ActionBinding(ActionCopyOriginal, KeyKindFull, "Clipboard"),
		ActionBinding(ActionChat, KeyKindFull, "AI"),
		ActionBinding(ActionSendToChat, KeyKindFull, "AI"),
		ActionBinding(ActionCreds, KeyKindFull, "System"),
		ActionBinding(ActionOtelConfig, KeyKindFull, "System"),
	}
}
