// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"testing"
	"time"
)

func TestExtractLogEntry_Timestamp(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]interface{}
		wantTime time.Time
		fuzzy    bool // If true, just check that it's not default time (recent)
	}{
		{
			name: "epoch millis as float64",
			raw: map[string]interface{}{
				"@timestamp": float64(1705315845000),
			},
			wantTime: time.UnixMilli(1705315845000),
		},
		{
			name: "epoch millis with fractional microseconds",
			raw: map[string]interface{}{
				"@timestamp": float64(1705315845123.456),
			},
			wantTime: time.UnixMilli(1705315845123).Add(456 * time.Microsecond),
		},
		{
			name: "epoch millis as string",
			raw: map[string]interface{}{
				"@timestamp": "1705315845000.123",
			},
			wantTime: time.UnixMilli(1705315845000).Add(123 * time.Microsecond),
		},
		{
			name: "RFC3339 string",
			raw: map[string]interface{}{
				"@timestamp": "2024-01-15T10:30:45Z",
			},
			wantTime: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name: "RFC3339Nano string",
			raw: map[string]interface{}{
				"@timestamp": "2024-01-15T10:30:45.123456789Z",
			},
			wantTime: time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC),
		},
		{
			name: "custom format with millis",
			raw: map[string]interface{}{
				"@timestamp": "2024-01-15T10:30:45.000Z",
			},
			wantTime: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:  "no timestamp defaults to now",
			raw:   map[string]interface{}{},
			fuzzy: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := extractLogEntry(tc.raw)
			if tc.fuzzy {
				if time.Since(entry.Timestamp) > time.Second {
					t.Errorf("Timestamp = %v, want recent time", entry.Timestamp)
				}
			} else {
				if !entry.Timestamp.Equal(tc.wantTime) {
					t.Errorf("Timestamp = %v, want %v", entry.Timestamp, tc.wantTime)
				}
			}
		})
	}
}

func TestExtractLogEntry_Message(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]interface{}
		wantBody string
		wantMsg  string
	}{
		{
			name: "OTel body.text",
			raw: map[string]interface{}{
				"body": map[string]interface{}{
					"text": "OTel log message",
				},
			},
			wantBody: "OTel log message",
		},
		{
			name: "body as string",
			raw: map[string]interface{}{
				"body": "simple body",
			},
			wantBody: "simple body",
		},
		{
			name: "message field",
			raw: map[string]interface{}{
				"message": "standard message",
			},
			wantBody: "standard message",
			wantMsg:  "standard message",
		},
		{
			name: "event_name fallback",
			raw: map[string]interface{}{
				"event_name": "user.created",
			},
			wantBody: "user.created",
		},
		{
			name: "body.text takes priority over message",
			raw: map[string]interface{}{
				"body": map[string]interface{}{
					"text": "body text wins",
				},
				"message": "message field",
			},
			wantBody: "body text wins",
			wantMsg:  "message field",
		},
		{
			name:     "no message fields",
			raw:      map[string]interface{}{},
			wantBody: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := extractLogEntry(tc.raw)
			if entry.Body != tc.wantBody {
				t.Errorf("Body = %q, want %q", entry.Body, tc.wantBody)
			}
			if entry.Message != tc.wantMsg {
				t.Errorf("Message = %q, want %q", entry.Message, tc.wantMsg)
			}
		})
	}
}

func TestExtractLogEntry_Level(t *testing.T) {
	tests := []struct {
		name      string
		raw       map[string]interface{}
		wantLevel string
	}{
		{
			name: "severity_text (OTel)",
			raw: map[string]interface{}{
				"severity_text": "ERROR",
			},
			wantLevel: "ERROR",
		},
		{
			name: "log.level",
			raw: map[string]interface{}{
				"log.level": "WARN",
			},
			wantLevel: "WARN",
		},
		{
			name: "level field",
			raw: map[string]interface{}{
				"level": "DEBUG",
			},
			wantLevel: "DEBUG",
		},
		{
			name: "severity_number TRACE (1-4)",
			raw: map[string]interface{}{
				"severity_number": float64(4),
			},
			wantLevel: "TRACE",
		},
		{
			name: "severity_number DEBUG (5-8)",
			raw: map[string]interface{}{
				"severity_number": float64(8),
			},
			wantLevel: "DEBUG",
		},
		{
			name: "severity_number INFO (9-12)",
			raw: map[string]interface{}{
				"severity_number": float64(12),
			},
			wantLevel: "INFO",
		},
		{
			name: "severity_number WARN (13-16)",
			raw: map[string]interface{}{
				"severity_number": float64(16),
			},
			wantLevel: "WARN",
		},
		{
			name: "severity_number ERROR (17-20)",
			raw: map[string]interface{}{
				"severity_number": float64(20),
			},
			wantLevel: "ERROR",
		},
		{
			name: "severity_number FATAL (21+)",
			raw: map[string]interface{}{
				"severity_number": float64(21),
			},
			wantLevel: "FATAL",
		},
		{
			name: "severity_text takes priority over severity_number",
			raw: map[string]interface{}{
				"severity_text":   "INFO",
				"severity_number": float64(20), // ERROR level
			},
			wantLevel: "INFO",
		},
		{
			name:      "no level",
			raw:       map[string]interface{}{},
			wantLevel: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := extractLogEntry(tc.raw)
			if entry.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", entry.Level, tc.wantLevel)
			}
		})
	}
}

func TestExtractLogEntry_ServiceName(t *testing.T) {
	tests := []struct {
		name            string
		raw             map[string]interface{}
		wantServiceName string
	}{
		{
			name: "OTel semconv: resource.attributes.service.name",
			raw: map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"service.name": "my-service",
					},
				},
			},
			wantServiceName: "my-service",
		},
		{
			name: "flat resource.service.name",
			raw: map[string]interface{}{
				"resource": map[string]interface{}{
					"service.name": "flat-service",
				},
			},
			wantServiceName: "flat-service",
		},
		{
			name: "attributes.service.name",
			raw: map[string]interface{}{
				"attributes": map[string]interface{}{
					"service.name": "attr-service",
				},
			},
			wantServiceName: "attr-service",
		},
		{
			name: "top-level service.name",
			raw: map[string]interface{}{
				"service.name": "top-level-service",
			},
			wantServiceName: "top-level-service",
		},
		{
			name: "OTel takes priority",
			raw: map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"service.name": "otel-wins",
					},
				},
				"service.name": "top-level",
			},
			wantServiceName: "otel-wins",
		},
		{
			name:            "no service name",
			raw:             map[string]interface{}{},
			wantServiceName: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := extractLogEntry(tc.raw)
			if entry.ServiceName != tc.wantServiceName {
				t.Errorf("ServiceName = %q, want %q", entry.ServiceName, tc.wantServiceName)
			}
		})
	}
}

func TestExtractLogEntry_TraceFields(t *testing.T) {
	raw := map[string]interface{}{
		"trace_id": "abc123",
		"span_id":  "def456",
		"name":     "GET /api/users",
		"kind":     "SERVER",
		"duration": float64(1500000), // nanoseconds
		"status": map[string]interface{}{
			"code": "OK",
		},
	}

	entry := extractLogEntry(raw)

	if entry.TraceID != "abc123" {
		t.Errorf("TraceID = %q, want %q", entry.TraceID, "abc123")
	}
	if entry.SpanID != "def456" {
		t.Errorf("SpanID = %q, want %q", entry.SpanID, "def456")
	}
	if entry.Name != "GET /api/users" {
		t.Errorf("Name = %q, want %q", entry.Name, "GET /api/users")
	}
	if entry.Kind != "SERVER" {
		t.Errorf("Kind = %q, want %q", entry.Kind, "SERVER")
	}
	if entry.Duration != 1500000 {
		t.Errorf("Duration = %d, want %d", entry.Duration, 1500000)
	}
	if entry.Status == nil || entry.Status["code"] != "OK" {
		t.Errorf("Status = %v, want map with code=OK", entry.Status)
	}
}

func TestExtractLogEntry_ContainerID(t *testing.T) {
	tests := []struct {
		name            string
		raw             map[string]interface{}
		wantContainerID string
	}{
		{
			name: "container_id",
			raw: map[string]interface{}{
				"container_id": "abc123",
			},
			wantContainerID: "abc123",
		},
		{
			name: "container.id",
			raw: map[string]interface{}{
				"container.id": "def456",
			},
			wantContainerID: "def456",
		},
		{
			name: "container_id takes priority",
			raw: map[string]interface{}{
				"container_id": "preferred",
				"container.id": "fallback",
			},
			wantContainerID: "preferred",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := extractLogEntry(tc.raw)
			if entry.ContainerID != tc.wantContainerID {
				t.Errorf("ContainerID = %q, want %q", entry.ContainerID, tc.wantContainerID)
			}
		})
	}
}

func TestLogEntry_GetMessage(t *testing.T) {
	tests := []struct {
		name     string
		entry    LogEntry
		expected string
	}{
		{
			name:     "Body takes priority",
			entry:    LogEntry{Body: "body text", Message: "message text"},
			expected: "body text",
		},
		{
			name:     "Falls back to Message",
			entry:    LogEntry{Message: "message text"},
			expected: "message text",
		},
		{
			name:     "Falls back to EventName",
			entry:    LogEntry{EventName: "event.happened"},
			expected: "event.happened",
		},
		{
			name:     "Falls back to Name (for traces)",
			entry:    LogEntry{Name: "GET /api/users"},
			expected: "GET /api/users",
		},
		{
			name:     "Returns empty if nothing set",
			entry:    LogEntry{},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.entry.GetMessage()
			if result != tc.expected {
				t.Errorf("GetMessage() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestLogEntry_GetLevel(t *testing.T) {
	tests := []struct {
		name     string
		entry    LogEntry
		expected string
	}{
		{
			name:     "Returns Level if set",
			entry:    LogEntry{Level: "ERROR"},
			expected: "ERROR",
		},
		{
			name:     "Defaults to INFO",
			entry:    LogEntry{},
			expected: "INFO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.entry.GetLevel()
			if result != tc.expected {
				t.Errorf("GetLevel() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestLogEntry_GetResource(t *testing.T) {
	tests := []struct {
		name     string
		entry    LogEntry
		expected string
	}{
		{
			name: "service.namespace from attributes",
			entry: LogEntry{
				Resource: map[string]interface{}{
					"attributes": map[string]interface{}{
						"service.namespace": "production",
					},
				},
			},
			expected: "production",
		},
		{
			name: "deployment.environment from attributes",
			entry: LogEntry{
				Resource: map[string]interface{}{
					"attributes": map[string]interface{}{
						"deployment.environment": "staging",
					},
				},
			},
			expected: "staging",
		},
		{
			name: "host.name from attributes",
			entry: LogEntry{
				Resource: map[string]interface{}{
					"attributes": map[string]interface{}{
						"host.name": "server-01",
					},
				},
			},
			expected: "server-01",
		},
		{
			name: "k8s.namespace.name from attributes",
			entry: LogEntry{
				Resource: map[string]interface{}{
					"attributes": map[string]interface{}{
						"k8s.namespace.name": "kube-system",
					},
				},
			},
			expected: "kube-system",
		},
		{
			name: "priority order: service.namespace over deployment.environment",
			entry: LogEntry{
				Resource: map[string]interface{}{
					"attributes": map[string]interface{}{
						"deployment.environment": "staging",
						"service.namespace":      "namespace-wins",
					},
				},
			},
			expected: "namespace-wins",
		},
		{
			name: "flat resource field",
			entry: LogEntry{
				Resource: map[string]interface{}{
					"host.name": "flat-host",
				},
			},
			expected: "flat-host",
		},
		{
			name:     "empty resource",
			entry:    LogEntry{},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.entry.GetResource()
			if result != tc.expected {
				t.Errorf("GetResource() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestLogEntry_GetFieldValue(t *testing.T) {
	entry := LogEntry{
		Timestamp:   time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Level:       "ERROR",
		ServiceName: "my-service",
		Body:        "test message",
		ContainerID: "container123",
		TraceID:     "trace123",
		SpanID:      "span456",
		Name:        "GET /api",
		Duration:    1500000000, // 1.5 seconds in nanoseconds
		Kind:        "SERVER",
		Status: map[string]interface{}{
			"code": "OK",
		},
		Scope: map[string]interface{}{
			"name": "my-scope",
		},
		Attributes: map[string]interface{}{
			"http.method":      "GET",
			"http.status_code": float64(200),
			"event.name":       "request.completed",
		},
		Resource: map[string]interface{}{
			"attributes": map[string]interface{}{
				"service.name":           "resource-service",
				"deployment.environment": "production",
			},
		},
		Metrics: map[string]interface{}{
			"cpu.usage": float64(45.5),
		},
	}

	tests := []struct {
		fieldPath string
		expected  string
	}{
		// Built-in fields
		{"@timestamp", "10:30:45"},
		{"level", "ERROR"},
		{"severity_text", "ERROR"},
		{"service.name", "my-service"},
		{"resource.attributes.service.name", "my-service"},
		{"body", "test message"},
		{"body.text", "test message"},
		{"message", "test message"},
		{"container_id", "container123"},
		{"container.id", "container123"},

		// Trace fields
		{"trace_id", "trace123"},
		{"span_id", "span456"},
		{"name", "GET /api"},
		{"duration", "1500000000"},
		{"duration_ms", "1500.0"},
		{"kind", "SERVER"},
		{"status.code", "OK"},
		{"scope.name", "my-scope"},

		// Attributes
		{"http.method", "GET"},
		{"attributes.http.method", "GET"},
		{"http.status_code", "200"},
		{"event.name", "request.completed"},
		{"attributes.event.name", "request.completed"},

		// Resource attributes
		{"resource.attributes.deployment.environment", "production"},
		{"deployment.environment", "production"},

		// Metrics
		{"_metrics", "cpu.usage=45.5"},

		// Non-existent
		{"nonexistent.field", ""},
	}

	for _, tc := range tests {
		t.Run(tc.fieldPath, func(t *testing.T) {
			result := entry.GetFieldValue(tc.fieldPath)
			if result != tc.expected {
				t.Errorf("GetFieldValue(%q) = %q, want %q", tc.fieldPath, result, tc.expected)
			}
		})
	}
}

func TestGetNestedValue(t *testing.T) {
	data := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"value": "nested value",
			},
		},
		"simple": "simple value",
	}

	tests := []struct {
		path     string
		expected string
	}{
		{"simple", "simple value"},
		{"level1.level2.value", "nested value"},
		{"nonexistent", ""},
		{"level1.nonexistent", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			result := getNestedValue(data, tc.path)
			if result != tc.expected {
				t.Errorf("getNestedValue(%q) = %q, want %q", tc.path, result, tc.expected)
			}
		})
	}

	// Test nil map
	if result := getNestedValue(nil, "any.path"); result != "" {
		t.Errorf("getNestedValue(nil, ...) = %q, want empty string", result)
	}
}

func TestLogEntry_GetFieldValue_DurationFormats(t *testing.T) {
	tests := []struct {
		name     string
		duration int64
		wantMs   string
	}{
		{
			name:     "milliseconds",
			duration: 1500000000, // 1.5s in ns
			wantMs:   "1500.0",
		},
		{
			name:     "sub-millisecond",
			duration: 500000, // 0.5ms in ns
			wantMs:   "0.500",
		},
		{
			name:     "zero duration",
			duration: 0,
			wantMs:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := LogEntry{Duration: tc.duration}
			result := entry.GetFieldValue("duration_ms")
			if result != tc.wantMs {
				t.Errorf("GetFieldValue(duration_ms) = %q, want %q", result, tc.wantMs)
			}
		})
	}
}
