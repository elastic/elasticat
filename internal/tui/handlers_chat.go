// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/agentbuilder"
)

// handleChatKey handles key events in the chat view.
func (m Model) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := GetAction(key)

	// Vim-style behavior:
	// - Normal mode (chatInsertMode=false): j/k/arrows scroll, enter/i enters insert, esc exits chat
	// - Insert mode (chatInsertMode=true): type/edit input, enter sends, esc exits insert

	// Normal mode actions
	if !m.chatInsertMode {
		switch action {
		case ActionScrollUp:
			m.chatViewport.ScrollUp(1)
			return m, nil
		case ActionScrollDown:
			m.chatViewport.ScrollDown(1)
			return m, nil
		case ActionPageUp:
			m.chatViewport.HalfPageUp()
			return m, nil
		case ActionPageDown:
			m.chatViewport.HalfPageDown()
			return m, nil
		case ActionGoTop:
			m.chatViewport.GotoTop()
			return m, nil
		case ActionGoBottom:
			m.chatViewport.GotoBottom()
			return m, nil
		case ActionCycleSignal:
			// Allow switching signals (m key) from chat view
			return m, m.cycleSignalType()
		}

		switch key {
		case "i", "enter":
			m.chatInsertMode = true
			m.chatInput.Focus()
			return m, textinput.Blink
		case "esc":
			// Try to pop view - if stack is empty (chat is root), do nothing
			// User can press 'q' to quit or 'm' to switch signals
			m.popView()
			return m, nil
		}

		// Ignore all other keys in normal mode.
		return m, nil
	}

	// Insert mode
	switch key {
	case "enter":
		if m.chatInput.Value() != "" && !m.chatLoading {
			return m.submitChatMessage()
		}
		return m, nil
	case "esc":
		m.chatInsertMode = false
		m.chatInput.Blur()
		return m, nil
	}

	// Update text input in insert mode
	var cmd tea.Cmd
	m.chatInput, cmd = m.chatInput.Update(msg)
	return m, cmd
}

// submitChatMessage sends the current input to the Agent Builder API.
func (m Model) submitChatMessage() (tea.Model, tea.Cmd) {
	userMessage := m.chatInput.Value()
	if userMessage == "" {
		return m, nil
	}

	// Add user message to history
	m.chatMessages = append(m.chatMessages, ChatMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})

	// Clear input and set loading state
	m.chatInput.SetValue("")
	m.chatLoading = true
	m.chatAnalysisContext = "" // Regular message, not item analysis
	m.chatRequestStart = time.Now()

	// Update viewport to show new message
	m.updateChatViewport()

	// Fetch response from Agent Builder
	return m, m.fetchChatResponse(userMessage)
}

// fetchChatResponse creates a command to call the Agent Builder API.
func (m *Model) fetchChatResponse(userMessage string) tea.Cmd {
	return func() tea.Msg {
		ctx, done := m.startRequest(requestChat, m.tuiConfig.ChatTimeout)
		defer done()

		// Create Agent Builder client
		client := agentbuilder.NewClient(agentbuilder.ClientOptions{
			KibanaURL: m.kibanaURL,
			APIKey:    m.esAPIKey,
			Username:  m.esUsername,
			Password:  m.esPassword,
			Space:     m.kibanaSpace,
			Timeout:   m.tuiConfig.ChatTimeout,
		})

		// Build context from current TUI state and prepend to first message
		input := userMessage
		if m.chatConversationID == "" {
			// First message - include TUI context
			tuiContext := m.buildChatContext()
			contextPrefix := agentbuilder.FormatContextAsSystemMessage(tuiContext)
			if contextPrefix != "" {
				input = contextPrefix + "\n\n" + userMessage
			}
		}

		// Call the API with the input string format
		req := agentbuilder.ConverseRequest{
			Input:          input,
			AgentID:        "elastic-ai-agent",
			ConversationID: m.chatConversationID,
		}

		resp, err := client.Converse(ctx, req)
		if err != nil {
			return chatResponseMsg{
				err: err,
			}
		}

		return chatResponseMsg{
			conversationID: resp.ConversationID,
			message: ChatMessage{
				Role:      "assistant",
				Content:   resp.Response.Message,
				Timestamp: time.Now(),
			},
		}
	}
}

// buildChatContext creates a ConversationContext from current TUI state.
func (m Model) buildChatContext() *agentbuilder.ConversationContext {
	filters := make(map[string]string)

	if m.filterService != "" {
		if m.negateService {
			filters["service (excluded)"] = m.filterService
		} else {
			filters["service"] = m.filterService
		}
	}
	if m.filterResource != "" {
		if m.negateResource {
			filters["resource (excluded)"] = m.filterResource
		} else {
			filters["resource"] = m.filterResource
		}
	}
	if m.levelFilter != "" {
		filters["level"] = m.levelFilter
	}
	if m.searchQuery != "" {
		filters["search"] = m.searchQuery
	}

	// Get selected item context
	selectedItem := ""
	if m.signalType == signalLogs && len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
		log := m.logs[m.selectedIndex]
		selectedItem = fmt.Sprintf("Log: %s - %s", log.GetLevel(), log.GetMessage())
	} else if m.signalType == signalTraces && m.selectedTxName != "" {
		selectedItem = fmt.Sprintf("Transaction: %s", m.selectedTxName)
	} else if m.signalType == signalMetrics && m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
		metric := m.aggregatedMetrics.Metrics[m.metricsCursor]
		selectedItem = fmt.Sprintf("Metric: %s", metric.Name)
	}

	return agentbuilder.BuildContextFromTUI(
		m.signalType.String(),
		m.client.GetIndex(),
		m.lookback.String(),
		filters,
		selectedItem,
	)
}

// updateChatViewport refreshes the chat viewport content.
func (m *Model) updateChatViewport() {
	content := m.renderChatMessages()
	m.chatViewport.SetContent(content)
	m.chatViewport.GotoBottom()
}

// handleChatResponseMsg handles responses from the Agent Builder API.
func (m Model) handleChatResponseMsg(msg chatResponseMsg) (Model, tea.Cmd) {
	m.chatLoading = false

	if msg.err != nil {
		// Add error message to chat
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:      "assistant",
			Content:   fmt.Sprintf("Error: %v", msg.err),
			Timestamp: time.Now(),
			Error:     true,
		})
		m.updateChatViewport()
		return m, nil
	}

	// Update conversation ID
	if msg.conversationID != "" {
		m.chatConversationID = msg.conversationID
	}

	// Clear analysis state
	m.chatAnalysisContext = ""
	m.chatRequestStart = time.Time{}

	// Add assistant message to history
	m.chatMessages = append(m.chatMessages, msg.message)
	m.updateChatViewport()

	return m, nil
}

// enterChatView switches to chat view.
func (m Model) enterChatView() (Model, tea.Cmd) {
	m.pushView(viewChat)
	// Start in normal mode; require `i`/Enter to type.
	m.chatInsertMode = false
	m.chatInput.Blur()
	m.chatLoading = false

	// Add welcome message if this is a new conversation
	if len(m.chatMessages) == 0 {
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:      "assistant",
			Content:   "Hello! I'm your AI assistant powered by Elastic Agent Builder. Ask me anything about your observability data - logs, traces, or metrics. I have context about your current filters and selections.",
			Timestamp: time.Now(),
		})
		m.updateChatViewport()
	}

	return m, textinput.Blink
}

// clearChatHistory resets the chat conversation.
func (m *Model) clearChatHistory() {
	m.chatMessages = []ChatMessage{}
	m.chatConversationID = ""
	m.updateChatViewport()
}

// getSelectedItemContext returns a description and JSON/summary of the currently selected item.
// Returns empty strings if no item is selected.
func (m Model) getSelectedItemContext() (description string, content string) {
	switch m.mode {
	case viewLogs, viewDetail, viewDetailJSON:
		// Logs list or detail view - use selected log entry
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			log := m.logs[m.selectedIndex]
			description = fmt.Sprintf("%s log from %s", log.GetLevel(), log.ServiceName)
			if log.ServiceName == "" {
				description = fmt.Sprintf("%s log", log.GetLevel())
			}
			content = log.RawJSON
		}

	case viewMetricsDashboard:
		// Metrics dashboard - use selected aggregated metric summary
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
			metric := m.aggregatedMetrics.Metrics[m.metricsCursor]
			description = fmt.Sprintf("metric %s", metric.Name)
			content = fmt.Sprintf(`{"name": %q, "type": %q, "min": %v, "max": %v, "avg": %v, "latest": %v}`,
				metric.Name, metric.Type, metric.Min, metric.Max, metric.Avg, metric.Latest)
		}

	case viewMetricDetail:
		// Metric detail view - use current metric document
		if len(m.metricDetailDocs) > 0 && m.metricDetailDocCursor < len(m.metricDetailDocs) {
			doc := m.metricDetailDocs[m.metricDetailDocCursor]
			metricName := ""
			if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
				metricName = m.aggregatedMetrics.Metrics[m.metricsCursor].Name
			}
			description = fmt.Sprintf("metric document for %s", metricName)
			content = doc.RawJSON
		}

	case viewTraceNames:
		// Trace names view - ask a question about the transaction name (not JSON analysis)
		if len(m.transactionNames) > 0 && m.traceNamesCursor < len(m.transactionNames) {
			tx := m.transactionNames[m.traceNamesCursor]
			// Return the question as description, empty content signals question-only mode
			description = fmt.Sprintf("What do you know about transactions with transaction.name '%s' in the index '%s'?", tx.Name, m.client.GetIndex())
			content = "" // Empty content = question mode
		}

	default:
		// For other views where logs contain data (e.g., trace transactions/spans)
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			log := m.logs[m.selectedIndex]
			if m.signalType == signalTraces {
				if log.Name != "" {
					description = fmt.Sprintf("span %s", log.Name)
				} else {
					description = "trace span"
				}
			} else {
				description = "selected item"
			}
			content = log.RawJSON
		}
	}
	return
}

// getSelectedItemQuery returns an ES Query DSL JSON string that retrieves the selected item.
// Returns empty string if no meaningful query can be built.
func (m Model) getSelectedItemQuery() string {
	var query map[string]interface{}

	switch m.mode {
	case viewLogs, viewDetail, viewDetailJSON:
		// Logs list or detail view - query by trace_id + span_id if available
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			log := m.logs[m.selectedIndex]

			// Best case: use trace_id + span_id for exact match
			if log.TraceID != "" && log.SpanID != "" {
				query = map[string]interface{}{
					"query": map[string]interface{}{
						"bool": map[string]interface{}{
							"filter": []map[string]interface{}{
								{"term": map[string]interface{}{"trace_id": log.TraceID}},
								{"term": map[string]interface{}{"span_id": log.SpanID}},
							},
						},
					},
					"size": 1,
				}
			} else if log.TraceID != "" {
				// Fallback: just trace_id with timestamp
				query = map[string]interface{}{
					"query": map[string]interface{}{
						"bool": map[string]interface{}{
							"filter": []map[string]interface{}{
								{"term": map[string]interface{}{"trace_id": log.TraceID}},
								{"range": map[string]interface{}{
									"@timestamp": map[string]interface{}{
										"gte": log.Timestamp.Add(-time.Second).Format(time.RFC3339),
										"lte": log.Timestamp.Add(time.Second).Format(time.RFC3339),
									},
								}},
							},
						},
					},
					"size": 1,
				}
			} else {
				// Last resort: timestamp + service name
				filters := []map[string]interface{}{
					{"range": map[string]interface{}{
						"@timestamp": map[string]interface{}{
							"gte": log.Timestamp.Add(-time.Second).Format(time.RFC3339),
							"lte": log.Timestamp.Add(time.Second).Format(time.RFC3339),
						},
					}},
				}
				if log.ServiceName != "" {
					filters = append(filters, map[string]interface{}{
						"term": map[string]interface{}{"resource.attributes.service.name": log.ServiceName},
					})
				}
				query = map[string]interface{}{
					"query": map[string]interface{}{
						"bool": map[string]interface{}{
							"filter": filters,
						},
					},
					"size": 1,
				}
			}
		}

	case viewMetricsDashboard:
		// Metrics dashboard - aggregation query for this specific metric
		if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
			metric := m.aggregatedMetrics.Metrics[m.metricsCursor]
			query = map[string]interface{}{
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"filter": []map[string]interface{}{
							{"exists": map[string]interface{}{"field": metric.Name}},
							{"range": map[string]interface{}{
								"@timestamp": map[string]interface{}{"gte": m.lookback.ESRange()},
							}},
						},
					},
				},
				"size": 10,
				"sort": []map[string]interface{}{
					{"@timestamp": map[string]interface{}{"order": "desc"}},
				},
			}
		}

	case viewMetricDetail:
		// Metric detail view - query for specific metric document
		if len(m.metricDetailDocs) > 0 && m.metricDetailDocCursor < len(m.metricDetailDocs) {
			doc := m.metricDetailDocs[m.metricDetailDocCursor]
			metricName := ""
			if m.aggregatedMetrics != nil && m.metricsCursor < len(m.aggregatedMetrics.Metrics) {
				metricName = m.aggregatedMetrics.Metrics[m.metricsCursor].Name
			}

			filters := []map[string]interface{}{
				{"range": map[string]interface{}{
					"@timestamp": map[string]interface{}{
						"gte": doc.Timestamp.Add(-time.Second).Format(time.RFC3339),
						"lte": doc.Timestamp.Add(time.Second).Format(time.RFC3339),
					},
				}},
			}
			if metricName != "" {
				filters = append(filters, map[string]interface{}{
					"exists": map[string]interface{}{"field": metricName},
				})
			}
			query = map[string]interface{}{
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"filter": filters,
					},
				},
				"size": 1,
			}
		}

	case viewTraceNames:
		// Trace names view - query for transactions with this name
		if len(m.transactionNames) > 0 && m.traceNamesCursor < len(m.transactionNames) {
			tx := m.transactionNames[m.traceNamesCursor]
			query = map[string]interface{}{
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"filter": []map[string]interface{}{
							{"term": map[string]interface{}{"name": tx.Name}},
							{"term": map[string]interface{}{"attributes.processor.event": "transaction"}},
							{"range": map[string]interface{}{
								"@timestamp": map[string]interface{}{"gte": m.lookback.ESRange()},
							}},
						},
					},
				},
				"size": 10,
				"sort": []map[string]interface{}{
					{"@timestamp": map[string]interface{}{"order": "desc"}},
				},
			}
		}

	default:
		// For trace transactions/spans - query by trace_id
		if len(m.logs) > 0 && m.selectedIndex < len(m.logs) {
			log := m.logs[m.selectedIndex]
			if log.TraceID != "" {
				filters := []map[string]interface{}{
					{"term": map[string]interface{}{"trace_id": log.TraceID}},
				}
				if log.SpanID != "" {
					filters = append(filters, map[string]interface{}{
						"term": map[string]interface{}{"span_id": log.SpanID},
					})
				}
				query = map[string]interface{}{
					"query": map[string]interface{}{
						"bool": map[string]interface{}{
							"filter": filters,
						},
					},
					"size": 1,
				}
			}
		}
	}

	if query == nil {
		return ""
	}

	// Format as indented JSON for readability
	jsonBytes, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// enterChatWithSelectedItem opens chat and adds the selected item as context.
func (m Model) enterChatWithSelectedItem() (Model, tea.Cmd) {
	// Get context and query BEFORE pushing view (which changes m.mode)
	description, content := m.getSelectedItemContext()
	itemQuery := m.getSelectedItemQuery()
	index := m.client.GetIndex()

	m.pushView(viewChat)
	// Start in insert mode so user can see/edit the prefilled message.
	m.chatInsertMode = true
	m.chatInput.Focus()

	// Add or update the welcome message
	if len(m.chatMessages) == 0 {
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:      "assistant",
			Content:   "Hello! I'm your AI assistant powered by Elastic Agent Builder. Ask me anything about your observability data - logs, traces, or metrics. I have context about your current filters and selections.",
			Timestamp: time.Now(),
		})
	}

	// Build query context for the specific selected item
	queryContext := ""
	if itemQuery != "" {
		queryContext = fmt.Sprintf("\n\nThis item can be retrieved with this query:\n\n```json\n%s\n```", itemQuery)
	}

	if content != "" {
		// Analysis mode: send JSON data with context
		contextMsg := fmt.Sprintf("I'm looking at this %s from the index '%s'. Here's the data:\n\n```json\n%s\n```%s\n\nPlease provide a brief analysis. What are the key insights?",
			description, index, content, queryContext)
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:      "user",
			Content:   contextMsg,
			Timestamp: time.Now(),
		})
		m.updateChatViewport()

		// Auto-submit to get AI analysis
		m.chatLoading = true
		m.chatAnalysisContext = description // e.g., "INFO log from demo", "metric cpu.usage"
		m.chatRequestStart = time.Now()
		return m, m.fetchChatResponse(contextMsg)
	} else if description != "" {
		// Question mode: description IS the question (e.g., for trace names)
		questionWithContext := description + " Please be brief."
		if queryContext != "" {
			questionWithContext = description + queryContext + "\n\nPlease be brief."
		}
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:      "user",
			Content:   questionWithContext,
			Timestamp: time.Now(),
		})
		m.updateChatViewport()

		// Auto-submit the question
		m.chatLoading = true
		m.chatAnalysisContext = "transaction name"
		m.chatRequestStart = time.Now()
		return m, m.fetchChatResponse(questionWithContext)
	}

	m.updateChatViewport()
	return m, textinput.Blink
}
