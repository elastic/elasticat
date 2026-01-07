// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

// Package agentbuilder provides a client for the Elastic Agent Builder API.
package agentbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client handles communication with the Elastic Agent Builder API.
type Client struct {
	kibanaURL  string
	apiKey     string
	username   string
	password   string
	httpClient *http.Client
	space      string
}

// ClientOptions holds configuration for creating a new Agent Builder client.
type ClientOptions struct {
	KibanaURL string        // Kibana URL (Agent Builder endpoint is derived from this)
	APIKey    string        // API key for authentication
	Username  string        // Username for basic auth
	Password  string        // Password for basic auth
	Space     string        // Kibana space (optional)
	Timeout   time.Duration // Request timeout
}

// NewClient creates a new Agent Builder client from options.
func NewClient(opts ClientOptions) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &Client{
		kibanaURL: strings.TrimSuffix(opts.KibanaURL, "/"),
		apiKey:    opts.APIKey,
		username:  opts.Username,
		password:  opts.Password,
		space:     opts.Space,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Message represents a chat message in the conversation.
type Message struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // Message content
}

// ConversationContext provides context for the chat request.
type ConversationContext struct {
	SignalType     string            `json:"signal_type,omitempty"`     // logs, traces, metrics
	IndexPattern   string            `json:"index_pattern,omitempty"`   // Current index pattern
	TimeRange      string            `json:"time_range,omitempty"`      // Lookback duration
	Filters        map[string]string `json:"filters,omitempty"`         // Active filters
	SelectedItem   string            `json:"selected_item,omitempty"`   // Selected log/trace/metric details
	AdditionalInfo string            `json:"additional_info,omitempty"` // Any other relevant context
}

// ConverseRequest represents a request to the converse API.
type ConverseRequest struct {
	AgentID        string               `json:"agentId,omitempty"`        // Agent ID (optional, uses default if not set)
	ConversationID string               `json:"conversationId,omitempty"` // Conversation ID for continuity
	Messages       []Message            `json:"messages"`                 // Conversation history
	Context        *ConversationContext `json:"context,omitempty"`        // TUI context
}

// ToolCall represents a tool call made by the agent.
type ToolCall struct {
	Name   string                 `json:"name"`
	Args   map[string]interface{} `json:"args,omitempty"`
	Result string                 `json:"result,omitempty"`
}

// ConverseResponse represents the response from the converse API.
type ConverseResponse struct {
	ConversationID string     `json:"conversationId"`
	Message        Message    `json:"message"`
	ToolCalls      []ToolCall `json:"toolCalls,omitempty"`
	ThinkingSteps  []string   `json:"thinkingSteps,omitempty"`
	Error          string     `json:"error,omitempty"`
}

// Converse sends a message to the Agent Builder and returns the response.
func (c *Client) Converse(ctx context.Context, req ConverseRequest) (*ConverseResponse, error) {
	// Build the API endpoint URL
	endpoint := c.buildEndpoint("/api/agent_builder/converse")

	// Marshal the request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("kbn-xsrf", "true")

	// Set authentication
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "ApiKey "+c.apiKey)
	} else if c.username != "" {
		httpReq.SetBasicAuth(c.username, c.password)
	}

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var converseResp ConverseResponse
	if err := json.Unmarshal(respBody, &converseResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if converseResp.Error != "" {
		return nil, fmt.Errorf("agent error: %s", converseResp.Error)
	}

	return &converseResp, nil
}

// buildEndpoint constructs the full API endpoint URL.
func (c *Client) buildEndpoint(path string) string {
	if c.space != "" {
		return fmt.Sprintf("%s/s/%s%s", c.kibanaURL, c.space, path)
	}
	return c.kibanaURL + path
}

// Ping checks if the Agent Builder API is reachable.
func (c *Client) Ping(ctx context.Context) error {
	// Try to list agents as a health check
	endpoint := c.buildEndpoint("/api/agent_builder/agents")

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("kbn-xsrf", "true")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "ApiKey "+c.apiKey)
	} else if c.username != "" {
		httpReq.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to ping Agent Builder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Agent Builder not available (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// BuildContextFromTUI creates a ConversationContext from TUI state.
func BuildContextFromTUI(signalType, indexPattern, timeRange string, filters map[string]string, selectedItem string) *ConversationContext {
	return &ConversationContext{
		SignalType:   signalType,
		IndexPattern: indexPattern,
		TimeRange:    timeRange,
		Filters:      filters,
		SelectedItem: selectedItem,
	}
}

// FormatContextAsSystemMessage formats the TUI context as a system message prefix.
func FormatContextAsSystemMessage(ctx *ConversationContext) string {
	if ctx == nil {
		return ""
	}

	var parts []string

	if ctx.SignalType != "" {
		parts = append(parts, fmt.Sprintf("Currently viewing: %s", ctx.SignalType))
	}
	if ctx.IndexPattern != "" {
		parts = append(parts, fmt.Sprintf("Index: %s", ctx.IndexPattern))
	}
	if ctx.TimeRange != "" {
		parts = append(parts, fmt.Sprintf("Time range: %s", ctx.TimeRange))
	}
	if len(ctx.Filters) > 0 {
		var filterParts []string
		for k, v := range ctx.Filters {
			filterParts = append(filterParts, fmt.Sprintf("%s=%s", k, v))
		}
		parts = append(parts, fmt.Sprintf("Filters: %s", strings.Join(filterParts, ", ")))
	}
	if ctx.SelectedItem != "" {
		parts = append(parts, fmt.Sprintf("Selected: %s", ctx.SelectedItem))
	}

	if len(parts) == 0 {
		return ""
	}

	return "Current context:\n" + strings.Join(parts, "\n")
}
