// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/elastic/elasticat/internal/config"
	"github.com/elastic/elasticat/internal/es"
)

// Model is the main TUI model containing all application state.
//
// State is organized into embedded structs for better organization:
//   - Core: client, ctx, requests (remain flat - used everywhere)
//   - Filters: service, resource, level, search query, signal type
//   - UI: mode, dimensions, loading, error state
//   - Query: last query display state
//   - Logs: log entries and selection
//   - Fields: field selection state
//   - Metrics: dashboard and detail state
//   - Traces: navigation hierarchy state
//   - Perspective: filtering by service/resource
//   - Chat: AI chat state
//   - Creds: credentials modal state
//   - Otel: OTel config modal state
//   - Components: text inputs and viewports
type Model struct {
	// === Core (flat - used everywhere) ===
	client      DataSource       // Data source (interface for testability)
	ctx         context.Context  // Parent context (canceled when app exits)
	requests    *requestManager  // In-flight request management
	tuiConfig   config.TUIConfig // TUI timing/config
	kibanaURL   string           // Kibana base URL for "Open in Kibana" feature
	kibanaSpace string           // Kibana space (e.g., "elasticat") for URL path prefix
	esAPIKey    string           // ES/Kibana API key for Agent Builder auth
	esUsername  string           // ES/Kibana username for Agent Builder auth
	esPassword  string           // ES/Kibana password for Agent Builder auth

	// === Embedded State ===
	Filters     FilterState
	UI          UIState
	Query       QueryState
	Logs        LogsState
	Fields      FieldsState
	Metrics     MetricsState
	Traces      TracesState
	Perspective PerspectiveState
	Chat        ChatState
	Creds       CredsState
	Otel        OtelState
	Components  UIComponents
}

// Highlighter returns a Highlighter configured with the current search query
func (m Model) Highlighter() *Highlighter {
	return NewHighlighter(m.Filters.Query)
}

// setViewportContent wraps content to the viewport width before rendering.
func (m *Model) setViewportContent(content string) {
	wrapped := WrapText(content, m.Components.Viewport.Width)
	m.Components.Viewport.SetContent(wrapped)
}

// NewModel creates a new TUI model.
// The client parameter accepts any DataSource implementation, enabling
// mock data sources for testing.
// NewModelOpts holds optional configuration for NewModel.
type NewModelOpts struct {
	ESAPIKey   string
	ESUsername string
	ESPassword string
}

func NewModel(ctx context.Context, client DataSource, signal SignalType, tuiCfg config.TUIConfig, kibanaURL, kibanaSpace string) Model {
	return NewModelWithOpts(ctx, client, signal, tuiCfg, kibanaURL, kibanaSpace, NewModelOpts{})
}

func NewModelWithOpts(ctx context.Context, client DataSource, signal SignalType, tuiCfg config.TUIConfig, kibanaURL, kibanaSpace string, opts NewModelOpts) Model {
	ti := textinput.New()
	ti.Placeholder = "Search... (supports ES query syntax)"
	ti.CharLimit = 256
	ti.Width = 50

	ii := textinput.New()
	ii.Placeholder = "Index pattern (e.g., logs, traces, metrics)"
	ii.CharLimit = 128
	ii.Width = 50

	ci := textinput.New()
	ci.Placeholder = "Ask a question about your o11y data..."
	ci.CharLimit = 1024
	ci.Width = 70

	// Set the client's index pattern based on the signal type
	if signal != SignalChat {
		client.SetIndex(signal.IndexPattern())
	}
	ii.SetValue(client.GetIndex())

	vp := viewport.New(80, 20)
	errorVp := viewport.New(70, 15) // Viewport for error modal
	helpVp := viewport.New(70, 15)  // Viewport for help overlay
	chatVp := viewport.New(80, 15)  // Viewport for chat history

	// Determine initial view mode based on signal type
	var initialMode viewMode
	switch signal {
	case SignalTraces:
		initialMode = viewTraceNames
	case SignalMetrics:
		initialMode = viewMetricsDashboard
	case SignalChat:
		initialMode = viewChat
	default:
		initialMode = viewLogs
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// Use default Kibana URL if not configured
	if kibanaURL == "" {
		kibanaURL = config.DefaultKibanaURL
	}

	m := Model{
		ctx:         ctx,
		client:      client,
		tuiConfig:   tuiCfg,
		kibanaURL:   kibanaURL,
		kibanaSpace: kibanaSpace,
		esAPIKey:    opts.ESAPIKey,
		esUsername:  opts.ESUsername,
		esPassword:  opts.ESPassword,
		requests:    newRequestManager(),

		Filters: FilterState{
			Signal:   signal,
			Lookback: lookback24h,
		},
		UI: UIState{
			Mode:            initialMode,
			AutoRefresh:     true,
			TimeDisplayMode: timeDisplayRelative,
			Width:           80,
			Height:          24,
		},
		Logs: LogsState{
			Entries: []es.LogEntry{},
		},
		Fields: FieldsState{
			Display: DefaultFields(signal),
		},
		Metrics: MetricsState{
			ViewMode: metricsViewAggregated,
		},
		Traces: TracesState{
			ViewLevel: traceViewNames,
		},
		Chat: ChatState{
			Messages: []ChatMessage{},
			Input:    ci,
			Viewport: chatVp,
		},
		Components: UIComponents{
			SearchInput:   ti,
			IndexInput:    ii,
			Viewport:      vp,
			ErrorViewport: errorVp,
			HelpViewport:  helpVp,
		},
	}

	// If we start in chat view, initialize chat state like enterChatView would.
	if initialMode == viewChat {
		m.Chat.InsertMode = false
		m.Chat.Input.Blur()
		m.Chat.Loading = false
		if len(m.Chat.Messages) == 0 {
			m.Chat.Messages = append(m.Chat.Messages, ChatMessage{
				Role:      "assistant",
				Content:   "Hello! I'm your AI assistant powered by Elastic Agent Builder. Ask me anything about your observability data - logs, traces, or metrics. I have context about your current filters and selections.",
				Timestamp: time.Now(),
			})
		}
		m.updateChatViewport()
	}

	return m
}
