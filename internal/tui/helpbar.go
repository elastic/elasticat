package tui

import "strings"

// renderHelpBar renders context-sensitive help bar showing available keyboard shortcuts
func (m Model) renderHelpBar() string {
	var keys []string

	switch m.mode {
	case viewLogs:
		keys = []string{
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" signal"),
			HelpKeyStyle.Render("p") + HelpDescStyle.Render(" perspective"),
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" lookback"),
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" search"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" details"),
			HelpKeyStyle.Render("s") + HelpDescStyle.Render(" sort"),
			HelpKeyStyle.Render("f") + HelpDescStyle.Render(" fields"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" clear"),
			HelpKeyStyle.Render("Q") + HelpDescStyle.Render(" query"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
		// Add 'd' for dashboard when viewing metrics documents
		if m.signalType == signalMetrics && m.metricsViewMode == metricsViewDocuments {
			keys = append([]string{HelpKeyStyle.Render("d") + HelpDescStyle.Render(" dashboard")}, keys...)
		}
		// Add 'esc' for trace navigation
		if m.signalType == signalTraces && (m.traceViewLevel == traceViewTransactions || m.traceViewLevel == traceViewSpans) {
			keys = append([]string{HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back")}, keys...)
		}
	case viewSearch:
		keys = []string{
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" search"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" cancel"),
		}
	case viewIndex:
		keys = []string{
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" apply"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" cancel"),
		}
	case viewQuery:
		keys = []string{
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("k") + HelpDescStyle.Render(" Kibana"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" curl"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	case viewFields:
		keys = []string{
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("space") + HelpDescStyle.Render(" toggle"),
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" search"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" reset"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	case viewDetail:
		keys = []string{
			HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" prev/next"),
			HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("j") + HelpDescStyle.Render(" JSON"),
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
		// Add 's' for viewing spans when in traces
		if m.signalType == signalTraces {
			keys = append([]string{HelpKeyStyle.Render("s") + HelpDescStyle.Render(" spans")}, keys...)
		}
	case viewDetailJSON:
		keys = []string{
			HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" prev/next"),
			HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" details"),
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" copy"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" close"),
		}
	case viewMetricsDashboard:
		keys = []string{
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" detail"),
			HelpKeyStyle.Render("p") + HelpDescStyle.Render(" perspective"),
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" lookback"),
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" documents"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" signal"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	case viewMetricDetail:
		keys = []string{
			HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" prev/next metric"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back to dashboard"),
		}
	case viewTraceNames:
		keys = []string{
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" select"),
			HelpKeyStyle.Render("p") + HelpDescStyle.Render(" perspective"),
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" lookback"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" signal"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	case viewPerspectiveList:
		keys = []string{
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" toggle filter"),
			HelpKeyStyle.Render("p") + HelpDescStyle.Render(" cycle"),
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" lookback"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" clear all"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	}

	return HelpStyle.Render(strings.Join(keys, "  "))
}
