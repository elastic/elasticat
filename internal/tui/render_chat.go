// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Chat styles
var (
	chatUserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	chatAssistantStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)

	chatErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	chatMessageStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	chatTimestampStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true)

	chatLoadingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Italic(true)

	chatInputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(0, 1)

	chatInputBorderInactiveStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("240")). // Gray border when inactive
					Padding(0, 1)

	chatTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)
)

// renderChatView renders the full chat interface.
func (m Model) renderChatView(height int) string {
	var b strings.Builder

	// Calculate available heights
	inputHeight := 3 // Border + input + padding
	titleHeight := 1
	availableHeight := height - inputHeight - titleHeight
	if availableHeight < 1 {
		availableHeight = 1
	}

	// Update viewport dimensions
	m.Chat.Viewport.Width = m.UI.Width - 4
	m.Chat.Viewport.Height = availableHeight

	// Chat title bar
	title := chatTitleStyle.Render(" 💬 AI Chat (Agent Builder) ")
	contextInfo := m.renderChatContextBar()
	titleLine := lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", contextInfo)
	b.WriteString(titleLine)
	b.WriteString("\n")

	// Messages viewport
	// Ensure the viewport area consumes its full height so the input stays pinned above the help bar.
	messagesView := lipgloss.NewStyle().
		Width(m.Chat.Viewport.Width).
		Height(m.Chat.Viewport.Height).
		Render(m.Chat.Viewport.View())
	b.WriteString(messagesView)
	b.WriteString("\n")

	// Loading indicator or input
	isLoading := false
	if m.requests != nil {
		isLoading = m.requests.inFlight(requestChat)
	}

	if isLoading {
		loading := m.renderChatLoadingIndicator()
		b.WriteString(loading)
	} else {
		// Input area - style depends on whether we're in insert mode
		var inputBox string
		if m.Chat.InsertMode {
			// Active: colored border, normal placeholder
			inputBox = chatInputBorderStyle.Width(m.UI.Width - 6).Render(m.Chat.Input.View())
		} else {
			// Inactive: gray border, hint to activate
			inactiveHint := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true).
				Render("Press 'i' or [Enter] to type...")
			inputBox = chatInputBorderInactiveStyle.Width(m.UI.Width - 6).Render(inactiveHint)
		}
		b.WriteString(inputBox)
	}

	return b.String()
}

// renderChatLoadingIndicator returns the loading message for chat requests.
// Shows a dynamic timer and context-aware message when analyzing selected items.
func (m Model) renderChatLoadingIndicator() string {
	// Calculate elapsed time
	elapsed := ""
	if !m.Chat.RequestStart.IsZero() {
		duration := time.Since(m.Chat.RequestStart)
		elapsed = fmt.Sprintf(" (%.1fs)", duration.Seconds())
	}

	// Build message based on context
	var message string
	if m.Chat.AnalysisContext != "" {
		// Analyzing a specific item (from "C" key)
		message = fmt.Sprintf("⏳ Analyzing the %s as requested...%s", m.Chat.AnalysisContext, elapsed)
	} else {
		// Regular chat message
		message = fmt.Sprintf("⏳ Thinking...%s", elapsed)
	}

	return chatLoadingStyle.Render(message)
}

// renderChatContextBar shows the current TUI context.
func (m Model) renderChatContextBar() string {
	var parts []string

	// Signal type context
	if m.Filters.Signal != signalChat {
		parts = append(parts, fmt.Sprintf("Signal: %s", m.Filters.Signal.String()))
	}

	// Time range
	parts = append(parts, fmt.Sprintf("Time: %s", m.Filters.Lookback.String()))

	// Filters
	if m.Filters.Service != "" {
		prefix := "Service: "
		if m.Filters.NegateService {
			prefix = "Service (not): "
		}
		parts = append(parts, prefix+m.Filters.Service)
	}

	if m.Filters.Query != "" {
		parts = append(parts, fmt.Sprintf("Query: %s", TruncateWithEllipsis(m.Filters.Query, 20)))
	}

	if len(parts) == 0 {
		return chatTimestampStyle.Render("No active context")
	}

	return chatTimestampStyle.Render(strings.Join(parts, " │ "))
}

// renderChatMessages formats all chat messages for display.
func (m Model) renderChatMessages() string {
	if len(m.Chat.Messages) == 0 {
		return chatTimestampStyle.Render("No messages yet. Type a question to get started!")
	}

	var b strings.Builder
	maxWidth := m.UI.Width - 8 // Leave some margin

	for i, msg := range m.Chat.Messages {
		if i > 0 {
			b.WriteString("\n\n")
		}

		// Role header
		var roleStyle lipgloss.Style
		var roleLabel string
		switch msg.Role {
		case "user":
			roleStyle = chatUserStyle
			roleLabel = "You"
		case "assistant":
			if msg.Error {
				roleStyle = chatErrorStyle
				roleLabel = "Error"
			} else {
				roleStyle = chatAssistantStyle
				roleLabel = "Assistant"
			}
		default:
			roleStyle = chatTimestampStyle
			roleLabel = msg.Role
		}

		// Timestamp
		timestamp := msg.Timestamp.Format("15:04:05")
		header := fmt.Sprintf("%s %s",
			roleStyle.Render(roleLabel+":"),
			chatTimestampStyle.Render(timestamp),
		)
		b.WriteString(header)
		b.WriteString("\n")

		// Message content with word wrapping
		wrapped := WrapText(msg.Content, maxWidth)
		content := chatMessageStyle.Render(wrapped)
		b.WriteString(content)
	}

	return b.String()
}

// renderChatCompactDetail renders a compact preview below the chat (if needed).
func (m Model) renderChatCompactDetail() string {
	if m.Chat.Loading {
		return chatLoadingStyle.Render("  Waiting for response...")
	}

	if len(m.Chat.Messages) == 0 {
		return chatTimestampStyle.Render("  Press 'i' or Enter to start typing")
	}

	// Show hint about available commands
	hints := []string{
		"↑↓ scroll",
		"i/Enter type",
		"esc back",
	}
	return chatTimestampStyle.Render("  " + strings.Join(hints, " │ "))
}
