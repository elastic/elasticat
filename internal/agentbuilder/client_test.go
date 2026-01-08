// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package agentbuilder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("creates client with defaults", func(t *testing.T) {
		t.Parallel()
		client := NewClient(ClientOptions{
			KibanaURL: "http://localhost:5601",
		})

		if client.kibanaURL != "http://localhost:5601" {
			t.Errorf("expected kibanaURL 'http://localhost:5601', got %q", client.kibanaURL)
		}
		if client.httpClient.Timeout != 60*time.Second {
			t.Errorf("expected default timeout 60s, got %v", client.httpClient.Timeout)
		}
	})

	t.Run("trims trailing slash from URL", func(t *testing.T) {
		t.Parallel()
		client := NewClient(ClientOptions{
			KibanaURL: "http://localhost:5601/",
		})

		if client.kibanaURL != "http://localhost:5601" {
			t.Errorf("expected trailing slash trimmed, got %q", client.kibanaURL)
		}
	})

	t.Run("uses custom timeout", func(t *testing.T) {
		t.Parallel()
		client := NewClient(ClientOptions{
			KibanaURL: "http://localhost:5601",
			Timeout:   30 * time.Second,
		})

		if client.httpClient.Timeout != 30*time.Second {
			t.Errorf("expected timeout 30s, got %v", client.httpClient.Timeout)
		}
	})

	t.Run("stores authentication credentials", func(t *testing.T) {
		t.Parallel()
		client := NewClient(ClientOptions{
			KibanaURL: "http://localhost:5601",
			APIKey:    "test-api-key",
			Username:  "elastic",
			Password:  "changeme",
			Space:     "my-space",
		})

		if client.apiKey != "test-api-key" {
			t.Errorf("expected apiKey 'test-api-key', got %q", client.apiKey)
		}
		if client.username != "elastic" {
			t.Errorf("expected username 'elastic', got %q", client.username)
		}
		if client.password != "changeme" {
			t.Errorf("expected password 'changeme', got %q", client.password)
		}
		if client.space != "my-space" {
			t.Errorf("expected space 'my-space', got %q", client.space)
		}
	})
}

func TestBuildEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("without space", func(t *testing.T) {
		t.Parallel()
		client := NewClient(ClientOptions{
			KibanaURL: "http://localhost:5601",
		})

		endpoint := client.buildEndpoint("/api/agent_builder/converse")
		expected := "http://localhost:5601/api/agent_builder/converse"
		if endpoint != expected {
			t.Errorf("expected %q, got %q", expected, endpoint)
		}
	})

	t.Run("with space", func(t *testing.T) {
		t.Parallel()
		client := NewClient(ClientOptions{
			KibanaURL: "http://localhost:5601",
			Space:     "my-space",
		})

		endpoint := client.buildEndpoint("/api/agent_builder/converse")
		expected := "http://localhost:5601/s/my-space/api/agent_builder/converse"
		if endpoint != expected {
			t.Errorf("expected %q, got %q", expected, endpoint)
		}
	})
}

func TestConverse(t *testing.T) {
	t.Parallel()

	t.Run("successful conversation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/agent_builder/converse" {
				t.Errorf("expected path /api/agent_builder/converse, got %s", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
			}
			if r.Header.Get("kbn-xsrf") != "true" {
				t.Errorf("expected kbn-xsrf header")
			}

			// Parse request body
			var req ConverseRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
			}
			if req.Input != "Hello" {
				t.Errorf("expected input 'Hello', got %q", req.Input)
			}

			// Send response
			resp := ConverseResponse{
				ConversationID: "conv-123",
				Response: ResponseMessage{
					Message: "Hello! How can I help?",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
		})

		resp, err := client.Converse(context.Background(), ConverseRequest{
			Input: "Hello",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ConversationID != "conv-123" {
			t.Errorf("expected conversation ID 'conv-123', got %q", resp.ConversationID)
		}
		if resp.Response.Message != "Hello! How can I help?" {
			t.Errorf("unexpected response message: %q", resp.Response.Message)
		}
	})

	t.Run("uses API key authentication", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "ApiKey test-key" {
				t.Errorf("expected 'ApiKey test-key', got %q", authHeader)
			}

			resp := ConverseResponse{Response: ResponseMessage{Message: "OK"}}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
			APIKey:    "test-key",
		})

		_, err := client.Converse(context.Background(), ConverseRequest{Input: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("uses basic authentication", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok {
				t.Error("expected basic auth")
			}
			if username != "elastic" || password != "changeme" {
				t.Errorf("expected elastic:changeme, got %s:%s", username, password)
			}

			resp := ConverseResponse{Response: ResponseMessage{Message: "OK"}}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
			Username:  "elastic",
			Password:  "changeme",
		})

		_, err := client.Converse(context.Background(), ConverseRequest{Input: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("handles HTTP error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized"))
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
		})

		_, err := client.Converse(context.Background(), ConverseRequest{Input: "test"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "API error (status 401)") {
			t.Errorf("expected API error message, got: %v", err)
		}
	})

	t.Run("handles agent error in response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ConverseResponse{
				Error: "Agent is unavailable",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
		})

		_, err := client.Converse(context.Background(), ConverseRequest{Input: "test"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "agent error: Agent is unavailable") {
			t.Errorf("expected agent error message, got: %v", err)
		}
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
		})

		_, err := client.Converse(context.Background(), ConverseRequest{Input: "test"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to parse response") {
			t.Errorf("expected parse error, got: %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			resp := ConverseResponse{Response: ResponseMessage{Message: "OK"}}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.Converse(ctx, ConverseRequest{Input: "test"})
		if err == nil {
			t.Fatal("expected error due to cancelled context")
		}
	})
}

func TestPing(t *testing.T) {
	t.Parallel()

	t.Run("successful ping", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/api/agent_builder/agents" {
				t.Errorf("expected path /api/agent_builder/agents, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
		})

		err := client.Ping(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ping with space", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/s/my-space/api/agent_builder/agents" {
				t.Errorf("expected path with space, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
			Space:     "my-space",
		})

		err := client.Ping(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ping failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("service unavailable"))
		}))
		defer server.Close()

		client := NewClient(ClientOptions{
			KibanaURL: server.URL,
		})

		err := client.Ping(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "Agent Builder not available (status 503)") {
			t.Errorf("expected availability error, got: %v", err)
		}
	})
}

func TestBuildContextFromTUI(t *testing.T) {
	t.Parallel()

	t.Run("creates context with all fields", func(t *testing.T) {
		t.Parallel()

		filters := map[string]string{
			"service": "my-service",
			"level":   "ERROR",
		}

		ctx := BuildContextFromTUI("logs", "logs-*", "now-15m", filters, "selected log entry")

		if ctx.SignalType != "logs" {
			t.Errorf("expected SignalType 'logs', got %q", ctx.SignalType)
		}
		if ctx.IndexPattern != "logs-*" {
			t.Errorf("expected IndexPattern 'logs-*', got %q", ctx.IndexPattern)
		}
		if ctx.TimeRange != "now-15m" {
			t.Errorf("expected TimeRange 'now-15m', got %q", ctx.TimeRange)
		}
		if len(ctx.Filters) != 2 {
			t.Errorf("expected 2 filters, got %d", len(ctx.Filters))
		}
		if ctx.Filters["service"] != "my-service" {
			t.Errorf("expected service filter 'my-service', got %q", ctx.Filters["service"])
		}
		if ctx.SelectedItem != "selected log entry" {
			t.Errorf("expected SelectedItem 'selected log entry', got %q", ctx.SelectedItem)
		}
	})

	t.Run("creates context with empty fields", func(t *testing.T) {
		t.Parallel()

		ctx := BuildContextFromTUI("", "", "", nil, "")

		if ctx.SignalType != "" {
			t.Errorf("expected empty SignalType, got %q", ctx.SignalType)
		}
		if ctx.Filters != nil {
			t.Errorf("expected nil Filters, got %v", ctx.Filters)
		}
	})
}

func TestFormatContextAsSystemMessage(t *testing.T) {
	t.Parallel()

	t.Run("formats full context", func(t *testing.T) {
		t.Parallel()

		ctx := &ConversationContext{
			SignalType:   "logs",
			IndexPattern: "logs-*",
			TimeRange:    "now-15m",
			Filters: map[string]string{
				"service": "my-service",
			},
			SelectedItem: "ERROR log from my-service",
		}

		result := FormatContextAsSystemMessage(ctx)

		if !strings.Contains(result, "Current context:") {
			t.Error("expected 'Current context:' header")
		}
		if !strings.Contains(result, "Currently viewing: logs") {
			t.Error("expected signal type")
		}
		if !strings.Contains(result, "Index: logs-*") {
			t.Error("expected index pattern")
		}
		if !strings.Contains(result, "Time range: now-15m") {
			t.Error("expected time range")
		}
		if !strings.Contains(result, "Filters: service=my-service") {
			t.Error("expected filters")
		}
		if !strings.Contains(result, "Selected: ERROR log from my-service") {
			t.Error("expected selected item")
		}
	})

	t.Run("returns empty string for nil context", func(t *testing.T) {
		t.Parallel()

		result := FormatContextAsSystemMessage(nil)
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("returns empty string for empty context", func(t *testing.T) {
		t.Parallel()

		ctx := &ConversationContext{}
		result := FormatContextAsSystemMessage(ctx)
		if result != "" {
			t.Errorf("expected empty string for empty context, got %q", result)
		}
	})

	t.Run("handles partial context", func(t *testing.T) {
		t.Parallel()

		ctx := &ConversationContext{
			SignalType: "traces",
		}

		result := FormatContextAsSystemMessage(ctx)

		if !strings.Contains(result, "Currently viewing: traces") {
			t.Error("expected signal type")
		}
		if strings.Contains(result, "Index:") {
			t.Error("should not include empty index")
		}
	})

	t.Run("handles multiple filters", func(t *testing.T) {
		t.Parallel()

		ctx := &ConversationContext{
			Filters: map[string]string{
				"service": "svc1",
				"level":   "ERROR",
			},
		}

		result := FormatContextAsSystemMessage(ctx)

		if !strings.Contains(result, "Filters:") {
			t.Error("expected filters section")
		}
		// Note: map iteration order is not guaranteed, so we check both filters are present
		if !strings.Contains(result, "service=svc1") {
			t.Error("expected service filter")
		}
		if !strings.Contains(result, "level=ERROR") {
			t.Error("expected level filter")
		}
	})
}
