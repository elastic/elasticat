package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"fmt"
)

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.autoDetectLookback(),
		m.tickCmd(),
		func() tea.Msg { return tea.EnableMouseCellMotion() },
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		return m, nil

	case logsMsg:
		m.loading = false
		m.lastRefresh = time.Now()
		if msg.err != nil {
			m.err = msg.err
			m.showErrorModal()
		} else {
			m.logs = msg.logs
			m.total = msg.total
			m.err = nil
			m.lastQueryJSON = msg.queryJSON
			m.lastQueryIndex = msg.index

			// Fetch spans for the selected trace when logs first load
			if m.signalType == signalTraces && len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
				traceID := m.logs[m.selectedIndex].TraceID
				if traceID != "" {
					m.spansLoading = true
					return m, m.fetchSpans(traceID)
				}
			}
		}
		return m, nil

	case tickMsg:
		if m.autoRefresh && m.mode == viewLogs {
			cmds = append(cmds, m.fetchLogs())
		}
		cmds = append(cmds, m.tickCmd())
		return m, tea.Batch(cmds...)

	case fieldCapsMsg:
		m.fieldsLoading = false
		if msg.err != nil {
			m.err = msg.err
			m.showErrorModal()
		} else {
			m.availableFields = msg.fields
		}
		return m, nil

	case autoDetectMsg:
		if msg.err != nil {
			// Auto-detect failed, just use current lookback and fetch
			m.loading = true
			// Signal-specific fetch on error
			switch m.signalType {
			case signalMetrics:
				if m.metricsViewMode == metricsViewAggregated {
					m.metricsLoading = true
					return m, m.fetchAggregatedMetrics()
				}
			case signalTraces:
				if m.traceViewLevel == traceViewNames {
					m.tracesLoading = true
					return m, m.fetchTransactionNames()
				}
			}
			return m, m.fetchLogs()
		}
		// Set the detected lookback and fetch
		m.lookback = msg.lookback
		m.statusMessage = fmt.Sprintf("Found %d entries in %s", msg.total, msg.lookback.String())
		m.statusTime = time.Now()
		// Signal-specific fetch
		switch m.signalType {
		case signalMetrics:
			if m.metricsViewMode == metricsViewAggregated {
				m.metricsLoading = true
				return m, m.fetchAggregatedMetrics()
			}
		case signalTraces:
			if m.traceViewLevel == traceViewNames {
				m.tracesLoading = true
				return m, m.fetchTransactionNames()
			}
		}
		m.loading = true
		return m, m.fetchLogs()

	case metricsAggMsg:
		m.metricsLoading = false
		if msg.err != nil {
			m.err = msg.err
			m.showErrorModal()
		} else {
			m.aggregatedMetrics = msg.result
			m.err = nil
		}
		return m, nil

	case transactionNamesMsg:
		m.tracesLoading = false
		if msg.err != nil {
			m.err = msg.err
			m.showErrorModal()
		} else {
			m.transactionNames = msg.names
			m.err = nil
		}
		return m, nil

	case spansMsg:
		m.spansLoading = false
		if msg.err != nil {
			m.err = msg.err
			m.showErrorModal()
		} else {
			m.spans = msg.spans
			m.err = nil
		}
		return m, nil

	case perspectiveDataMsg:
		m.perspectiveLoading = false
		if msg.err != nil {
			m.err = msg.err
			m.showErrorModal()
		} else {
			m.perspectiveItems = msg.items
			m.err = nil
			m.statusMessage = fmt.Sprintf("Loaded %d %s", len(msg.items), m.currentPerspective.String())
			m.statusTime = time.Now()
		}
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		m.showErrorModal()
		return m, nil
	}

	// Update components based on mode
	switch m.mode {
	case viewSearch:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewIndex:
		var cmd tea.Cmd
		m.indexInput, cmd = m.indexInput.Update(msg)
		cmds = append(cmds, cmd)
	case viewDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case viewErrorModal:
		var cmd tea.Cmd
		m.errorViewport, cmd = m.errorViewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// showErrorModal configures the error viewport and switches to error modal view
func (m *Model) showErrorModal() {
	m.previousMode = m.mode
	m.mode = viewErrorModal
	// Set up error viewport dimensions
	modalWidth := min(m.width-8, 80)
	m.errorViewport.Width = modalWidth - 8 // Account for border + padding + margin
	m.errorViewport.Height = min(m.height-15, 20) // Leave room for title/actions
	if m.err != nil {
		m.errorViewport.SetContent(m.err.Error())
	}
	m.errorViewport.GotoTop()
}

