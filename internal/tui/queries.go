// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"sync"
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
	requestMetricDetailDocs
	requestTransactionNames
	requestSpans
	requestPerspective
	requestFieldCaps
	requestAutoDetect
	requestChat
)

type requestState struct {
	cancel context.CancelFunc
	id     int64
}

// requestManager handles in-flight request tracking with thread-safe access.
// It's stored as a pointer in Model so it remains shared when Model is copied
// (which happens frequently in bubbletea's value-receiver pattern).
type requestManager struct {
	mu      sync.Mutex
	cancels map[requestKind]requestState
	seq     int64
}

func newRequestManager() *requestManager {
	return &requestManager{
		cancels: make(map[requestKind]requestState),
	}
}

func (m *Model) fetchLogs() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestLogs, m.tuiConfig.LogsTimeout)
		defer done()

		var result *es.SearchResult
		var err error
		var queryString string
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
			result, queryString, err = m.client.SearchESQL(ctx, m.searchQuery, opts)
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
			result, queryString, err = m.client.TailESQL(ctx, opts)
		}

		if err != nil {
			return logsMsg{err: err}
		}

		return logsMsg{logs: result.Logs, total: result.Total, queryJSON: queryString, index: index}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.tuiConfig.TickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchAggregatedMetrics() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestMetricsAgg, m.tuiConfig.MetricsTimeout)
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

// fetchMetricDetailDocs fetches the latest 10 documents containing the selected metric
func (m *Model) fetchMetricDetailDocs() tea.Cmd {
	// Capture the metric info before returning the command
	if m.aggregatedMetrics == nil || m.metricsCursor >= len(m.aggregatedMetrics.Metrics) {
		return nil
	}
	metric := m.aggregatedMetrics.Metrics[m.metricsCursor]

	return func() tea.Msg {
		ctx, done := m.startRequest(requestMetricDetailDocs, m.tuiConfig.MetricsTimeout)
		defer done()

		opts := es.TailOptions{
			Size:        10,
			Lookback:    m.lookback.ESRange(),
			MetricField: metric.Name, // Filter for docs containing this metric
			SortAsc:     false,       // Latest first
		}

		var result *es.SearchResult
		var err error

		// Histogram fields can't be filtered with ES|QL `IS NOT NULL` because
		// ES|QL doesn't support histogram types in that context. Use Query DSL
		// which supports `exists` queries on histogram fields.
		if metric.Type == "histogram" {
			result, err = m.client.Tail(ctx, opts)
		} else {
			result, _, err = m.client.TailESQL(ctx, opts)
		}

		if err != nil {
			return metricDetailDocsMsg{err: err}
		}

		return metricDetailDocsMsg{docs: result.Logs}
	}
}

func (m *Model) fetchTransactionNames() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestTransactionNames, m.tuiConfig.TracesTimeout)
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
		ctx, done := m.startRequest(requestSpans, m.tuiConfig.TracesTimeout)
		defer done()

		opts := es.TailOptions{
			Size:           1000,
			TraceID:        traceID,
			ProcessorEvent: "span",
			SortAsc:        true,
		}

		result, _, err := m.client.TailESQL(ctx, opts)
		if err != nil {
			return spansMsg{err: err}
		}

		return spansMsg{spans: result.Logs}
	}
}

func (m *Model) fetchPerspectiveData() tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestPerspective, m.tuiConfig.LogsTimeout)
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
		ctx, done := m.startRequest(requestAutoDetect, m.tuiConfig.AutoDetectTimeout)
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
				Lookback:       lb.ESRange(),
				ProcessorEvent: processorEvent,
			}

			total, _, err := m.client.CountESQL(ctx, opts)
			if err != nil {
				continue
			}

			// Track the best option we've found
			if total > bestTotal {
				bestLookback = lb
				bestTotal = total
			}

			// If we found enough data, stop here and use this lookback
			if total >= targetCount {
				return autoDetectMsg{
					lookback: lb,
					total:    total,
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
		ctx, done := m.startRequest(requestFieldCaps, m.tuiConfig.FieldCapsTimeout)
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
// This method is safe to call concurrently from multiple goroutines (e.g., batch commands).
func (m *Model) startRequest(kind requestKind, timeout time.Duration) (context.Context, context.CancelFunc) {
	rm := m.requests
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if st, ok := rm.cancels[kind]; ok {
		st.cancel()
	}
	rm.seq++
	id := rm.seq
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	rm.cancels[kind] = requestState{cancel: cancel, id: id}
	return ctx, func() {
		rm.mu.Lock()
		defer rm.mu.Unlock()
		if cur, ok := rm.cancels[kind]; ok && cur.id == id {
			delete(rm.cancels, kind)
		}
		cancel()
	}
}
