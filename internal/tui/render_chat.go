// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

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
	availableHeight := height - inputHeight - titleHeight - 2

	// Update viewport dimensions
	m.chatViewport.Width = m.width - 4
	m.chatViewport.Height = availableHeight

	// Chat title bar
	title := chatTitleStyle.Render(" 💬 AI Chat (Agent Builder) ")
	contextInfo := m.renderChatContextBar()
	titleLine := lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", contextInfo)
	b.WriteString(titleLine)
	b.WriteString("\n")

	// Messages viewport
	b.WriteString(m.chatViewport.View())
	b.WriteString("\n")

	// Loading indicator or input
	if m.chatLoading {
		loading := chatLoadingStyle.Render("⏳ Thinking...")
		b.WriteString(loading)
	} else {
		// Input area
		inputBox := chatInputBorderStyle.Width(m.width - 6).Render(m.chatInput.View())
		b.WriteString(inputBox)
	}

	return b.String()
}

// renderChatContextBar shows the current TUI context.
func (m Model) renderChatContextBar() string {
	var parts []string

	// Signal type context
	if m.signalType != signalChat {
		parts = append(parts, fmt.Sprintf("Signal: %s", m.signalType.String()))
	}

	// Time range
	parts = append(parts, fmt.Sprintf("Time: %s", m.lookback.String()))

	// Filters
	if m.filterService != "" {
		prefix := "Service: "
		if m.negateService {
			prefix = "Service (not): "
		}
		parts = append(parts, prefix+m.filterService)
	}

	if m.searchQuery != "" {
		parts = append(parts, fmt.Sprintf("Query: %s", TruncateWithEllipsis(m.searchQuery, 20)))
	}

	if len(parts) == 0 {
		return chatTimestampStyle.Render("No active context")
	}

	return chatTimestampStyle.Render(strings.Join(parts, " │ "))
}

// renderChatMessages formats all chat messages for display.
func (m Model) renderChatMessages() string {
	if len(m.chatMessages) == 0 {
		return chatTimestampStyle.Render("No messages yet. Type a question to get started!")
	}

	var b strings.Builder
	maxWidth := m.width - 8 // Leave some margin

	for i, msg := range m.chatMessages {
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
	if m.chatLoading {
		return chatLoadingStyle.Render("  Waiting for response...")
	}

	if len(m.chatMessages) == 0 {
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
