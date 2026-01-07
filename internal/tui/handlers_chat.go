// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
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

	// Handle navigation in chat viewport (when not focused on input)
	if !m.chatInput.Focused() {
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
		}

		// Focus input on any typing character or enter
		switch key {
		case "i", "enter", "/":
			m.chatInput.Focus()
			return m, textinput.Blink
		case "esc":
			m.popView()
			return m, nil
		}
	}

	// Handle input mode
	switch key {
	case "enter":
		// Submit message
		if m.chatInput.Value() != "" && !m.chatLoading {
			return m.submitChatMessage()
		}
	case "esc":
		// Unfocus input or exit chat
		if m.chatInput.Focused() {
			m.chatInput.Blur()
		} else {
			m.popView()
		}
		return m, nil
	}

	// Update text input
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

		// Build context from current TUI state
		tuiContext := m.buildChatContext()

		// Build messages for the API (include context in first message)
		var messages []agentbuilder.Message
		contextPrefix := agentbuilder.FormatContextAsSystemMessage(tuiContext)

		for i, msg := range m.chatMessages {
			content := msg.Content
			// Prepend context to first user message
			if i == 0 && msg.Role == "user" && contextPrefix != "" {
				content = contextPrefix + "\n\n" + content
			}
			messages = append(messages, agentbuilder.Message{
				Role:    msg.Role,
				Content: content,
			})
		}

		// Call the API
		req := agentbuilder.ConverseRequest{
			ConversationID: m.chatConversationID,
			Messages:       messages,
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
				Role:      resp.Message.Role,
				Content:   resp.Message.Content,
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

	// Add assistant message to history
	m.chatMessages = append(m.chatMessages, msg.message)
	m.updateChatViewport()

	return m, nil
}

// enterChatView switches to chat view.
func (m *Model) enterChatView() tea.Cmd {
	m.pushView(viewChat)
	m.chatInput.Focus()

	// Add welcome message if this is a new conversation
	if len(m.chatMessages) == 0 {
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:      "assistant",
			Content:   "Hello! I'm your AI assistant powered by Elastic Agent Builder. Ask me anything about your observability data - logs, traces, or metrics. I have context about your current filters and selections.",
			Timestamp: time.Now(),
		})
		m.updateChatViewport()
	}

	return textinput.Blink
}

// clearChatHistory resets the chat conversation.
func (m *Model) clearChatHistory() {
	m.chatMessages = []ChatMessage{}
	m.chatConversationID = ""
	m.updateChatViewport()
}
