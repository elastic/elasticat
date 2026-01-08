// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

// ViewKeymap returns the full keymap for the current view/mode.
func (m Model) ViewKeymap() []KeyBinding {
	mode := m.mode
	if mode == viewHelp {
		mode = m.peekViewStack()
	}
	switch mode {
	case viewLogs:
		return m.keymapLogs()
	case viewDetail:
		return m.keymapDetail()
	case viewDetailJSON:
		return m.keymapDetailJSON()
	case viewFields:
		return m.keymapFields()
	case viewMetricsDashboard:
		return m.keymapMetricsDashboard()
	case viewMetricDetail:
		return m.keymapMetricDetail()
	case viewTraceNames:
		return m.keymapTraceNames()
	case viewPerspectiveList:
		return m.keymapPerspectiveList()
	case viewErrorModal:
		return m.keymapErrorModal()
	case viewChat:
		return m.keymapChat()
	default:
		// Modes with active text input or small sets fall back to minimal quick bindings.
		return nil
	}
}

func (m Model) keymapLogs() []KeyBinding {
	quick := []KeyBinding{
		ScrollBinding(KeyKindQuick),
		ActionBindingWithLabel(ActionSelect, "details", KeyKindQuick, "View"),
		ActionBinding(ActionSearch, KeyKindQuick, "Filter"),
		ActionBinding(ActionCycleLookback, KeyKindQuick, "Filter"),
		ActionBinding(ActionPerspective, KeyKindQuick, "View"),
		ActionBinding(ActionKibana, KeyKindQuick, "View"),
		ActionBinding(ActionChat, KeyKindQuick, "AI"),
	}

	// Full list excludes items already in quick to avoid duplicates in help overlay
	full := []KeyBinding{
		ActionBinding(ActionCycleSignal, KeyKindFull, "View"),
		ActionBinding(ActionSort, KeyKindFull, "View"),
		ActionBinding(ActionFields, KeyKindFull, "View"),
		ActionBinding(ActionQuery, KeyKindFull, "View"),
		ActionBinding(ActionRefresh, KeyKindFull, "View"),
		ActionBinding(ActionAutoRefresh, KeyKindFull, "View"),
		ActionBinding(ActionSendToChat, KeyKindFull, "AI"),
		ActionBinding(ActionCreds, KeyKindFull, "System"),
		ActionBinding(ActionOtelConfig, KeyKindFull, "System"),
		CombinedBinding([]string{"0-4"}, "level filters", KeyKindFull, "Filter"),
		ActionBinding(ActionQuit, KeyKindFull, "System"),
	}

	if m.signalType == signalMetrics && m.metricsViewMode == metricsViewDocuments {
		full = append([]KeyBinding{CombinedBinding([]string{"d"}, "dashboard", KeyKindFull, "View")}, full...)
	}
	if m.signalType == signalTraces && (m.traceViewLevel == traceViewTransactions || m.traceViewLevel == traceViewSpans) {
		full = append([]KeyBinding{ActionBinding(ActionBack, KeyKindFull, "Navigation")}, full...)
	}

	return append(quick, full...)
}

func (m Model) keymapDetail() []KeyBinding {
	quick := []KeyBinding{
		ScrollBinding(KeyKindQuick),
		PrevNextBinding("prev/next", KeyKindQuick),
		ActionBinding(ActionJSON, KeyKindQuick, "View"),
		ActionBinding(ActionCopy, KeyKindQuick, "Clipboard"),
		ActionBinding(ActionKibana, KeyKindQuick, "View"),
		ActionBindingWithLabel(ActionBack, "close", KeyKindQuick, "Navigation"),
	}
	full := DetailGlobalBindings()
	if m.signalType == signalTraces {
		full = append(full, ActionBinding(ActionSpans, KeyKindFull, "View"))
	}
	return append(quick, full...)
}

func (m Model) keymapDetailJSON() []KeyBinding {
	quick := []KeyBinding{
		ScrollBinding(KeyKindQuick),
		PrevNextBinding("prev/next", KeyKindQuick),
		ActionBindingWithLabel(ActionJSON, "details", KeyKindQuick, "View"),
		ActionBinding(ActionCopy, KeyKindQuick, "Clipboard"),
		ActionBindingWithLabel(ActionBack, "close", KeyKindQuick, "Navigation"),
	}
	return append(quick, DetailGlobalBindings()...)
}

func (m Model) keymapMetricsDashboard() []KeyBinding {
	quick := []KeyBinding{
		ScrollBinding(KeyKindQuick),
		ActionBindingWithLabel(ActionSelect, "detail", KeyKindQuick, "View"),
		ActionBinding(ActionCycleLookback, KeyKindQuick, "Filter"),
		ActionBinding(ActionPerspective, KeyKindQuick, "View"),
		ActionBinding(ActionKibana, KeyKindQuick, "View"),
		ActionBinding(ActionChat, KeyKindQuick, "AI"),
	}
	// Full list only adds items not in quick
	full := []KeyBinding{
		ActionBinding(ActionCycleSignal, KeyKindFull, "View"),
		ActionBinding(ActionRefresh, KeyKindFull, "View"),
		ActionBinding(ActionSendToChat, KeyKindFull, "AI"),
		ActionBinding(ActionCreds, KeyKindFull, "System"),
		ActionBinding(ActionOtelConfig, KeyKindFull, "System"),
		CombinedBinding([]string{"d"}, "documents", KeyKindFull, "View"),
		ActionBinding(ActionSearch, KeyKindFull, "Filter"),
		ActionBinding(ActionQuit, KeyKindFull, "System"),
	}
	return append(quick, full...)
}

func (m Model) keymapMetricDetail() []KeyBinding {
	quick := []KeyBinding{
		PrevNextBinding("prev/next metric", KeyKindQuick),
		CombinedBinding(ScrollDisplayKeys, "prev/next doc", KeyKindQuick, "Navigation"),
		ActionBinding(ActionCycleLookback, KeyKindQuick, "Filter"),
		ActionBinding(ActionJSON, KeyKindQuick, "View"),
		ActionBinding(ActionCopy, KeyKindQuick, "Clipboard"),
		ActionBinding(ActionKibana, KeyKindQuick, "View"),
		ActionBinding(ActionBack, KeyKindQuick, "Navigation"),
	}
	full := append([]KeyBinding{ActionBinding(ActionRefresh, KeyKindFull, "View")}, GlobalBindings()...)
	return append(quick, full...)
}

func (m Model) keymapTraceNames() []KeyBinding {
	quick := []KeyBinding{
		ScrollBinding(KeyKindQuick),
		ActionBindingWithLabel(ActionSelect, "select", KeyKindQuick, "View"),
		ActionBinding(ActionCycleLookback, KeyKindQuick, "Filter"),
		ActionBinding(ActionPerspective, KeyKindQuick, "View"),
		ActionBinding(ActionChat, KeyKindQuick, "AI"),
	}
	full := []KeyBinding{
		ActionBinding(ActionCycleSignal, KeyKindFull, "View"),
		ActionBinding(ActionQuery, KeyKindFull, "View"),
		ActionBinding(ActionSearch, KeyKindFull, "Filter"),
		ActionBinding(ActionRefresh, KeyKindFull, "View"),
		ActionBinding(ActionSendToChat, KeyKindFull, "AI"),
		ActionBinding(ActionCreds, KeyKindFull, "System"),
		ActionBinding(ActionOtelConfig, KeyKindFull, "System"),
		ActionBinding(ActionQuit, KeyKindFull, "System"),
	}
	return append(quick, full...)
}

func (m Model) keymapPerspectiveList() []KeyBinding {
	quick := []KeyBinding{
		ScrollBinding(KeyKindQuick),
		ActionBindingWithLabel(ActionSelect, "include/exclude", KeyKindQuick, "Filter"),
		ActionBindingWithLabel(ActionPerspective, "cycle", KeyKindQuick, "View"),
		ActionBinding(ActionCycleLookback, KeyKindQuick, "Filter"),
		ActionBinding(ActionBack, KeyKindQuick, "Navigation"),
	}
	full := []KeyBinding{
		ActionBinding(ActionSearch, KeyKindFull, "Filter"),
		ActionBinding(ActionRefresh, KeyKindFull, "View"),
	}
	full = append(full, GlobalBindingsWithQuit()...)
	return append(quick, full...)
}

func (m Model) keymapFields() []KeyBinding {
	quick := []KeyBinding{
		ScrollBinding(KeyKindQuick),
		CombinedBinding([]string{"space", "enter"}, "toggle", KeyKindQuick, "View"),
		ActionBinding(ActionSearch, KeyKindQuick, "Filter"),
		ActionBinding(ActionReset, KeyKindQuick, "View"),
		ActionBindingWithLabel(ActionBack, "close", KeyKindQuick, "Navigation"),
	}
	return append(quick, GlobalBindings()...)
}

func (m Model) keymapErrorModal() []KeyBinding {
	// Small set; help disabled; quick only.
	return []KeyBinding{
		ScrollBinding(KeyKindQuick),
		CombinedBinding([]string{"pgup", "pgdown"}, "page", KeyKindQuick, "Navigation"),
		CombinedBinding([]string{"g", "G"}, "top/bottom", KeyKindQuick, "Navigation"),
		ActionBinding(ActionCopy, KeyKindQuick, "Clipboard"),
		ActionBindingWithLabel(ActionBack, "close", KeyKindQuick, "Navigation"),
	}
}

func (m Model) keymapChat() []KeyBinding {
	if m.chatInsertMode {
		quick := []KeyBinding{
			CombinedBinding([]string{"enter"}, "send", KeyKindQuick, "Input"),
			CombinedBinding([]string{"esc"}, "normal", KeyKindQuick, "Input"),
		}
		return append(quick, SystemBindings()...)
	}

	quick := []KeyBinding{
		CombinedBinding([]string{"j", "k", "↑", "↓"}, "scroll", KeyKindQuick, "Navigation"),
		CombinedBinding([]string{"i", "enter"}, "insert", KeyKindQuick, "Input"),
		ActionBinding(ActionCycleSignal, KeyKindQuick, "View"),
		ActionBindingWithLabel(ActionBack, "close", KeyKindQuick, "Navigation"),
	}
	full := []KeyBinding{
		CombinedBinding([]string{"pgup", "pgdn"}, "page", KeyKindFull, "Navigation"),
		CombinedBinding([]string{"g", "G"}, "top/bottom", KeyKindFull, "Navigation"),
	}
	full = append(full, SystemBindings()...)
	return append(quick, full...)
}
