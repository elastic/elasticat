// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
	"github.com/elastic/elasticat/internal/es/metrics"
	"github.com/elastic/elasticat/internal/es/perspectives"
)

type requestKind int

const (
	requestLogs requestKind = iota
	requestMetricsAgg
	requestTransactionNames
	requestSpans
	requestPerspective
	requestFieldCaps
	requestAutoDetect
)

type requestState struct {
	cancel context.CancelFunc
	id     int64
}

func (m *Model) fetchLogs() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestLogs, 10*time.Second)
		defer done()

		var result *es.SearchResult
		var err error
		var queryJSON string
		index := m.client.GetIndex()

		lookbackRange := m.lookback.ESRange()

		// For traces, determine processor event filter based on view level
		processorEvent := ""
		transactionName := ""
		traceID := ""
		if m.signalType == signalTraces {
			switch m.traceViewLevel {
			case traceViewTransactions:
				processorEvent = "transaction"
				transactionName = m.selectedTxName
			case traceViewSpans:
				// When viewing spans, show all events for the trace (no processor filter)
				traceID = m.selectedTraceID
			default:
				processorEvent = "transaction"
			}
		}

		if m.searchQuery != "" {
			opts := es.SearchOptions{
				Size:            100,
				Service:         m.filterService,
				NegateService:   m.negateService,
				Resource:        m.filterResource,
				NegateResource:  m.negateResource,
				Level:           m.levelFilter,
				SortAsc:         m.sortAscending,
				SearchFields:    CollectSearchFields(m.displayFields),
				Lookback:        lookbackRange,
				ProcessorEvent:  processorEvent,
				TransactionName: transactionName,
				TraceID:         traceID,
			}
			result, err = m.client.Search(ctx, m.searchQuery, opts)
			queryJSON, _ = m.client.GetSearchQueryJSON(m.searchQuery, opts)
		} else {
			opts := es.TailOptions{
				Size:            100,
				Service:         m.filterService,
				NegateService:   m.negateService,
				Resource:        m.filterResource,
				NegateResource:  m.negateResource,
				Level:           m.levelFilter,
				SortAsc:         m.sortAscending,
				Lookback:        lookbackRange,
				ProcessorEvent:  processorEvent,
				TransactionName: transactionName,
				TraceID:         traceID,
			}
			result, err = m.client.Tail(ctx, opts)
			queryJSON, _ = m.client.GetTailQueryJSON(opts)
		}

		if err != nil {
			return logsMsg{err: err}
		}

		return logsMsg{logs: result.Logs, total: result.Total, queryJSON: queryJSON, index: index}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchAggregatedMetrics() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestMetricsAgg, 30*time.Second)
		defer done()

		lookbackRange := m.lookback.ESRange()
		bucketInterval := es.LookbackToBucketInterval(lookbackRange)

		opts := metrics.AggregateMetricsOptions{
			Lookback:       lookbackRange,
			BucketSize:     bucketInterval,
			Service:        m.filterService,
			NegateService:  m.negateService,
			Resource:       m.filterResource,
			NegateResource: m.negateResource,
		}

		result, err := m.client.AggregateMetrics(ctx, opts)
		if err != nil {
			return metricsAggMsg{err: err}
		}

		return metricsAggMsg{result: result}
	}
}

func (m *Model) fetchTransactionNames() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestTransactionNames, 30*time.Second)
		defer done()

		lookbackRange := m.lookback.ESRange()

		names, err := m.client.GetTransactionNamesESQL(ctx, lookbackRange, m.filterService, m.filterResource, m.negateService, m.negateResource)
		if err != nil {
			return transactionNamesMsg{err: err}
		}

		return transactionNamesMsg{names: names}
	}
}

// fetchSpans fetches all child spans for a given trace ID
func (m Model) fetchSpans(traceID string) tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestSpans, 10*time.Second)
		defer done()

		result, err := m.client.GetSpansByTraceID(ctx, traceID)
		if err != nil {
			return spansMsg{err: err}
		}

		return spansMsg{spans: result.Logs}
	}
}

func (m *Model) fetchPerspectiveData() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestPerspective, 10*time.Second)
		defer done()

		var aggs []perspectives.PerspectiveAgg
		var err error

		switch m.currentPerspective {
		case PerspectiveServices:
			aggs, err = m.client.GetServices(ctx, m.lookback.ESRange())
		case PerspectiveResources:
			aggs, err = m.client.GetResources(ctx, m.lookback.ESRange())
		}

		if err != nil {
			return perspectiveDataMsg{err: err}
		}

		// Convert to PerspectiveItem
		items := make([]PerspectiveItem, len(aggs))
		for i, agg := range aggs {
			items[i] = PerspectiveItem{
				Name:        agg.Name,
				LogCount:    agg.LogCount,
				TraceCount:  agg.TraceCount,
				MetricCount: agg.MetricCount,
			}
		}

		return perspectiveDataMsg{items: items}
	}
}

func (m *Model) autoDetectLookback() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestAutoDetect, 30*time.Second)
		defer done()

		// For traces, filter to only count transactions
		processorEvent := ""
		if m.signalType == signalTraces {
			processorEvent = "transaction"
		}

		// Try progressively larger time windows until we find enough data
		// Stop at first one with >= 10,000 entries (or use the one with most data)
		targetCount := int64(10000)
		bestLookback := lookback5m
		bestTotal := int64(0)

		for _, lb := range lookbackDurations {
			opts := es.TailOptions{
				Size:           1, // We only need count, not actual results
				Lookback:       lb.ESRange(),
				ProcessorEvent: processorEvent,
			}

			result, err := m.client.Tail(ctx, opts)
			if err != nil {
				continue
			}

			// Track the best option we've found
			if result.Total > bestTotal {
				bestLookback = lb
				bestTotal = result.Total
			}

			// If we found enough data, stop here and use this lookback
			if result.Total >= targetCount {
				return autoDetectMsg{
					lookback: lb,
					total:    result.Total,
				}
			}
		}

		// Return the best we found (even if < target)
		return autoDetectMsg{
			lookback: bestLookback,
			total:    bestTotal,
		}
	}
}

func (m *Model) fetchFieldCaps() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestFieldCaps, 10*time.Second)
		defer done()

		fields, err := m.client.GetFieldCaps(ctx)
		if err != nil {
			return fieldCapsMsg{err: err}
		}

		return fieldCapsMsg{fields: fields}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// startRequest cancels any in-flight request of the same kind, and returns a timeout-scoped context.
func (m *Model) startRequest(kind requestKind, timeout time.Duration) (context.Context, context.CancelFunc) {
	if m.cancels == nil {
		m.cancels = make(map[requestKind]requestState)
	}
	if st, ok := m.cancels[kind]; ok {
		st.cancel()
	}
	m.requestSeq++
	id := m.requestSeq
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	m.cancels[kind] = requestState{cancel: cancel, id: id}
	return ctx, func() {
		if cur, ok := m.cancels[kind]; ok && cur.id == id {
			delete(m.cancels, kind)
		}
		cancel()
	}
}
