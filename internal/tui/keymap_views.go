package tui

// ViewKeymap returns the full keymap for the current view/mode.
func (m Model) ViewKeymap() []KeyBinding {
	mode := m.mode
	if mode == viewHelp {
		mode = m.previousMode
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
	default:
		// Modes with active text input or small sets fall back to minimal quick bindings.
		return nil
	}
}

func (m Model) keymapLogs() []KeyBinding {
	quick := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "details", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"/"}, Label: "search", Kind: KeyKindQuick, Group: "Filter"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindQuick, Group: "Filter"},
		{Keys: []string{"m"}, Label: "signal", Kind: KeyKindQuick, Group: "View"},
	}

	full := []KeyBinding{
		{Keys: []string{"m"}, Label: "signal", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"p"}, Label: "perspective", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"/"}, Label: "search", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"enter"}, Label: "details", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"s"}, Label: "sort", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"f"}, Label: "fields", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"Q"}, Label: "query", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"r"}, Label: "refresh", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"a"}, Label: "auto refresh", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"0", "1", "2", "3", "4"}, Label: "level filters", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"q"}, Label: "quit", Kind: KeyKindFull, Group: "System"},
	}

	if m.signalType == signalMetrics && m.metricsViewMode == metricsViewDocuments {
		full = append([]KeyBinding{{Keys: []string{"d"}, Label: "dashboard", Kind: KeyKindFull, Group: "View"}}, full...)
	}
	if m.signalType == signalTraces && (m.traceViewLevel == traceViewTransactions || m.traceViewLevel == traceViewSpans) {
		full = append([]KeyBinding{{Keys: []string{"esc"}, Label: "back", Kind: KeyKindFull, Group: "Navigation"}}, full...)
	}

	return append(quick, full...)
}

func (m Model) keymapDetail() []KeyBinding {
	quick := []KeyBinding{
		{Keys: []string{"↑", "↓"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"←", "→"}, Label: "prev/next", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"j"}, Label: "JSON", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"y"}, Label: "copy", Kind: KeyKindQuick, Group: "Clipboard"},
		{Keys: []string{"esc"}, Label: "close", Kind: KeyKindQuick, Group: "Navigation"},
	}
	full := []KeyBinding{
		{Keys: []string{"←", "→"}, Label: "prev/next", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"↑", "↓"}, Label: "scroll", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"j"}, Label: "JSON", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"y"}, Label: "copy", Kind: KeyKindFull, Group: "Clipboard"},
		{Keys: []string{"esc"}, Label: "close", Kind: KeyKindFull, Group: "Navigation"},
	}
	if m.signalType == signalTraces {
		full = append(full, KeyBinding{Keys: []string{"s"}, Label: "spans", Kind: KeyKindFull, Group: "View"})
	}
	return append(quick, full...)
}

func (m Model) keymapDetailJSON() []KeyBinding {
	// Small view: no help overlay, quick only.
	return []KeyBinding{
		{Keys: []string{"↑", "↓"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"←", "→"}, Label: "prev/next", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"j"}, Label: "details", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"y"}, Label: "copy", Kind: KeyKindQuick, Group: "Clipboard"},
		{Keys: []string{"esc"}, Label: "close", Kind: KeyKindQuick, Group: "Navigation"},
	}
}

func (m Model) keymapMetricsDashboard() []KeyBinding {
	quick := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "detail", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindQuick, Group: "Filter"},
		{Keys: []string{"p"}, Label: "perspective", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"m"}, Label: "signal", Kind: KeyKindQuick, Group: "View"},
	}
	full := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "detail", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"p"}, Label: "perspective", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"m"}, Label: "signal", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"r"}, Label: "refresh", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"d"}, Label: "documents", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"/"}, Label: "search", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"q"}, Label: "quit", Kind: KeyKindFull, Group: "System"},
	}
	return append(quick, full...)
}

func (m Model) keymapMetricDetail() []KeyBinding {
	quick := []KeyBinding{
		{Keys: []string{"←", "→"}, Label: "prev/next", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"r"}, Label: "refresh", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"esc"}, Label: "back", Kind: KeyKindQuick, Group: "Navigation"},
	}
	full := []KeyBinding{
		{Keys: []string{"←", "→"}, Label: "prev/next", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"r"}, Label: "refresh", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"esc"}, Label: "back", Kind: KeyKindFull, Group: "Navigation"},
	}
	return append(quick, full...)
}

func (m Model) keymapTraceNames() []KeyBinding {
	quick := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "select", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindQuick, Group: "Filter"},
		{Keys: []string{"p"}, Label: "perspective", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"m"}, Label: "signal", Kind: KeyKindQuick, Group: "View"},
	}
	full := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "select", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"p"}, Label: "perspective", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"m"}, Label: "signal", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"/"}, Label: "search", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"r"}, Label: "refresh", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"q"}, Label: "quit", Kind: KeyKindFull, Group: "System"},
	}
	return append(quick, full...)
}

func (m Model) keymapPerspectiveList() []KeyBinding {
	quick := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "include/exclude", Kind: KeyKindQuick, Group: "Filter"},
		{Keys: []string{"p"}, Label: "cycle", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindQuick, Group: "Filter"},
		{Keys: []string{"esc"}, Label: "back", Kind: KeyKindQuick, Group: "Navigation"},
	}
	full := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "include/exclude", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"p"}, Label: "cycle", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"l"}, Label: "lookback", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"/"}, Label: "search", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"r"}, Label: "refresh", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"q"}, Label: "quit", Kind: KeyKindFull, Group: "System"},
	}
	return append(quick, full...)
}

func (m Model) keymapFields() []KeyBinding {
	quick := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"space", "enter"}, Label: "toggle", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"/"}, Label: "search", Kind: KeyKindQuick, Group: "Filter"},
		{Keys: []string{"r"}, Label: "reset", Kind: KeyKindQuick, Group: "View"},
		{Keys: []string{"esc"}, Label: "close", Kind: KeyKindQuick, Group: "Navigation"},
	}
	full := []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindFull, Group: "Navigation"},
		{Keys: []string{"space", "enter"}, Label: "toggle", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"/"}, Label: "search", Kind: KeyKindFull, Group: "Filter"},
		{Keys: []string{"r"}, Label: "reset", Kind: KeyKindFull, Group: "View"},
		{Keys: []string{"esc"}, Label: "close", Kind: KeyKindFull, Group: "Navigation"},
	}
	return append(quick, full...)
}

func (m Model) keymapErrorModal() []KeyBinding {
	// Small set; help disabled; quick only.
	return []KeyBinding{
		{Keys: []string{"j", "k"}, Label: "scroll", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"pgup", "pgdown"}, Label: "page", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"g", "G"}, Label: "top/bottom", Kind: KeyKindQuick, Group: "Navigation"},
		{Keys: []string{"y"}, Label: "copy", Kind: KeyKindQuick, Group: "Clipboard"},
		{Keys: []string{"esc"}, Label: "close", Kind: KeyKindQuick, Group: "Navigation"},
	}
}
