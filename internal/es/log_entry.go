// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// extractLogEntry extracts a LogEntry from raw Elasticsearch document.
//
// DESIGN PRINCIPLE: Progressive Enhancement with Graceful Degradation
// This function handles logs in ANY format - from completely unstructured to fully
// semconv-compliant OTel logs. It should NEVER fail or panic. In the worst case,
// it returns a minimal LogEntry with just the raw data preserved in Attributes.
//
// Enhancement levels (from least to most structured):
// 1. Raw/Unknown: Just preserve everything in Attributes, use current time
// 2. Basic: Has @timestamp and some message field
// 3. Standard: Has level/severity, service name in common locations
// 4. OTel Semconv: Full resource.attributes.service.name, severity_text, body.text
//
// The function tries multiple field locations for each piece of data, preferring
// the most semantically correct location but falling back to alternatives.
func extractLogEntry(raw map[string]interface{}) LogEntry {
	entry := LogEntry{
		Attributes: make(map[string]interface{}),
		Timestamp:  time.Now(), // Default to now if no timestamp found
	}

	// === TIMESTAMP EXTRACTION ===
	// Try multiple formats: OTel epoch millis, epoch micros, ISO strings
	if ts := raw["@timestamp"]; ts != nil {
		switch v := ts.(type) {
		case float64:
			// Numeric epoch milliseconds with fractional microseconds
			millis := int64(v)
			micros := int64((v - float64(millis)) * 1000)
			entry.Timestamp = time.UnixMilli(millis).Add(time.Duration(micros) * time.Microsecond)
		case string:
			// First, try parsing as epoch milliseconds string (OTel format: "1767303679749.488427")
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 1000000000000 {
				// Looks like epoch millis (> year 2001)
				millis := int64(f)
				micros := int64((f - float64(millis)) * 1000)
				entry.Timestamp = time.UnixMilli(millis).Add(time.Duration(micros) * time.Microsecond)
			} else {
				// Try multiple string date formats
				for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
					if t, err := time.Parse(layout, v); err == nil {
						entry.Timestamp = t
						break
					}
				}
			}
		}
	}

	// === MESSAGE/BODY EXTRACTION ===
	// Priority: body.text (OTel) > body (string) > message > event_name > _source as string
	extracted := false

	// OTel format: body is an object with text field
	if body, ok := raw["body"].(map[string]interface{}); ok {
		if text, ok := body["text"].(string); ok && text != "" {
			entry.Body = text
			extracted = true
		}
	}
	// Simple body string
	if !extracted {
		if body, ok := raw["body"].(string); ok && body != "" {
			entry.Body = body
			extracted = true
		}
	}
	// Standard message field
	if msg, ok := raw["message"].(string); ok && msg != "" {
		entry.Message = msg
		if !extracted {
			entry.Body = msg
			extracted = true
		}
	}
	// OTel event_name as fallback
	if !extracted {
		if eventName, ok := raw["event_name"].(string); ok && eventName != "" {
			entry.Body = eventName
		}
	}

	// === LEVEL/SEVERITY EXTRACTION ===
	// Priority: severity_text (OTel) > log.level > level > infer from severity_number
	if severity, ok := raw["severity_text"].(string); ok && severity != "" {
		entry.Level = severity
	} else if level, ok := raw["log.level"].(string); ok && level != "" {
		entry.Level = level
	} else if level, ok := raw["level"].(string); ok && level != "" {
		entry.Level = level
	} else if sevNum, ok := raw["severity_number"].(float64); ok {
		// OTel severity numbers: 1-4=TRACE, 5-8=DEBUG, 9-12=INFO, 13-16=WARN, 17-20=ERROR, 21-24=FATAL
		switch {
		case sevNum <= 4:
			entry.Level = "TRACE"
		case sevNum <= 8:
			entry.Level = "DEBUG"
		case sevNum <= 12:
			entry.Level = "INFO"
		case sevNum <= 16:
			entry.Level = "WARN"
		case sevNum <= 20:
			entry.Level = "ERROR"
		default:
			entry.Level = "FATAL"
		}
	}

	// === SERVICE NAME EXTRACTION ===
	// Priority: resource.attributes.service.name (OTel) > resource.service.name > attributes.service.name > service.name
	if resource, ok := raw["resource"].(map[string]interface{}); ok {
		entry.Resource = resource
		// OTel semconv: resource.attributes.service.name (flat key)
		if attrs, ok := resource["attributes"].(map[string]interface{}); ok {
			if svcName, ok := attrs["service.name"].(string); ok && svcName != "" {
				entry.ServiceName = svcName
			}
			// Nested: resource.attributes.service.name
			if entry.ServiceName == "" {
				if service, ok := attrs["service"].(map[string]interface{}); ok {
					if name, ok := service["name"].(string); ok && name != "" {
						entry.ServiceName = name
					}
				}
			}
		}
		// Flat resource.service.name
		if entry.ServiceName == "" {
			if svcName, ok := resource["service.name"].(string); ok && svcName != "" {
				entry.ServiceName = svcName
			}
		}
	}
	// Attributes-level service name (flat key)
	if entry.ServiceName == "" {
		if attrs, ok := raw["attributes"].(map[string]interface{}); ok {
			if svcName, ok := attrs["service.name"].(string); ok && svcName != "" {
				entry.ServiceName = svcName
			}
			// Nested: attributes.service.name
			if entry.ServiceName == "" {
				if service, ok := attrs["service"].(map[string]interface{}); ok {
					if name, ok := service["name"].(string); ok && name != "" {
						entry.ServiceName = name
					}
				}
			}
		}
	}
	// Top-level service.name (flat key)
	if entry.ServiceName == "" {
		if svcName, ok := raw["service.name"].(string); ok && svcName != "" {
			entry.ServiceName = svcName
		}
	}
	// Nested service object: service.name
	if entry.ServiceName == "" {
		if service, ok := raw["service"].(map[string]interface{}); ok {
			if name, ok := service["name"].(string); ok && name != "" {
				entry.ServiceName = name
			}
		}
	}

	// === CONTAINER ID ===
	if containerID, ok := raw["container_id"].(string); ok {
		entry.ContainerID = containerID
	} else if containerID, ok := raw["container.id"].(string); ok {
		entry.ContainerID = containerID
	}

	// === ATTRIBUTES ===
	// Preserve all attributes for detailed view
	if attrs, ok := raw["attributes"].(map[string]interface{}); ok {
		entry.Attributes = attrs
	}

	// === TRACE-SPECIFIC FIELDS ===
	if traceID, ok := raw["trace_id"].(string); ok {
		entry.TraceID = traceID
	}
	if spanID, ok := raw["span_id"].(string); ok {
		entry.SpanID = spanID
	}
	if name, ok := raw["name"].(string); ok {
		entry.Name = name
	}
	if kind, ok := raw["kind"].(string); ok {
		entry.Kind = kind
	}
	// Duration can be float64 or int
	if dur, ok := raw["duration"].(float64); ok {
		entry.Duration = int64(dur)
	}
	// Status is a nested object
	if status, ok := raw["status"].(map[string]interface{}); ok {
		entry.Status = status
	}

	// === METRICS-SPECIFIC FIELDS ===
	if metrics, ok := raw["metrics"].(map[string]interface{}); ok {
		entry.Metrics = metrics
	}
	if scope, ok := raw["scope"].(map[string]interface{}); ok {
		entry.Scope = scope
	}

	return entry
}

// GetMessage returns the log message, checking multiple possible fields
func (l *LogEntry) GetMessage() string {
	if l.Body != "" {
		return l.Body
	}
	if l.Message != "" {
		return l.Message
	}
	if l.EventName != "" {
		return l.EventName
	}
	// For traces, use span name
	if l.Name != "" {
		return l.Name
	}
	return ""
}

// GetLevel returns the log level with a default
func (l *LogEntry) GetLevel() string {
	if l.Level != "" {
		return l.Level
	}
	return "INFO"
}

// GetResource returns a displayable resource identifier
// Prioritizes: service.namespace > deployment.environment > host.name > first available attribute
func (l *LogEntry) GetResource() string {
	if len(l.Resource) == 0 {
		return ""
	}

	// Check for attributes sub-map (OTel format)
	if attrs, ok := l.Resource["attributes"].(map[string]interface{}); ok {
		// Priority list of resource attributes to display
		priorities := []string{
			"service.namespace",
			"deployment.environment",
			"host.name",
			"k8s.namespace.name",
			"cloud.region",
		}
		for _, key := range priorities {
			if val, ok := attrs[key].(string); ok && val != "" {
				return val
			}
		}
		// Fallback: return first string attribute that isn't service.name
		for k, v := range attrs {
			if k == "service.name" {
				continue
			}
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}

	// Check flat resource fields
	priorities := []string{
		"service.namespace",
		"deployment.environment",
		"host.name",
	}
	for _, key := range priorities {
		if val, ok := l.Resource[key].(string); ok && val != "" {
			return val
		}
	}

	return ""
}

// GetFieldValue extracts a field value from a log entry by field path
func (l *LogEntry) GetFieldValue(fieldPath string) string {
	// Handle built-in fields first
	switch fieldPath {
	case "@timestamp":
		return fmt.Sprintf("%02d:%02d:%02d", l.Timestamp.Hour(), l.Timestamp.Minute(), l.Timestamp.Second())
	case "level", "severity_text":
		return l.GetLevel()
	case "service.name", "resource.attributes.service.name":
		return l.ServiceName
	case "body", "body.text", "message":
		return l.GetMessage()
	case "container_id", "container.id":
		return l.ContainerID
	// Trace-specific fields
	case "trace_id":
		return l.TraceID
	case "span_id":
		return l.SpanID
	case "name":
		if l.Name != "" {
			return l.Name
		}
		return l.GetMessage() // Fallback to message for logs
	case "duration":
		if l.Duration > 0 {
			return fmt.Sprintf("%d", l.Duration)
		}
		return ""
	case "duration_ms":
		if l.Duration > 0 {
			// Duration is in nanoseconds, convert to milliseconds
			ms := float64(l.Duration) / 1_000_000.0
			if ms < 1 {
				return fmt.Sprintf("%.3f", ms)
			}
			return fmt.Sprintf("%.1f", ms)
		}
		return ""
	case "kind":
		return l.Kind
	case "status.code":
		if l.Status != nil {
			if code, ok := l.Status["code"].(string); ok {
				return code
			}
		}
		return ""
	case "scope.name":
		if l.Scope != nil {
			if name, ok := l.Scope["name"].(string); ok {
				return name
			}
		}
		return ""
	case "_metrics":
		// Format metrics as key=value pairs
		if len(l.Metrics) == 0 {
			return ""
		}
		var parts []string
		for k, v := range l.Metrics {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		return strings.Join(parts, ", ")
	}

	// Check attributes - try multiple approaches
	if val, ok := l.Attributes[fieldPath]; ok {
		return fmt.Sprintf("%v", val)
	}

	// Check nested attribute paths (e.g., "attributes.http.method" or "attributes.event.name")
	if len(fieldPath) > 11 && fieldPath[:11] == "attributes." {
		key := fieldPath[11:]
		// First, try direct key lookup (for keys with dots like "event.name")
		if val, ok := l.Attributes[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// Then try traversing nested structure
		if v, ok := GetNestedPath(l.Attributes, key); ok {
			if v == nil {
				return ""
			}
			return fmt.Sprintf("%v", v)
		}
	}

	// Try the field path directly as a nested path in attributes
	if v, ok := GetNestedPath(l.Attributes, fieldPath); ok {
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}

	// Check resource attributes
	if len(l.Resource) > 0 {
		if attrs, ok := l.Resource["attributes"].(map[string]interface{}); ok {
			// Handle "resource.attributes.X" paths
			if len(fieldPath) > 20 && fieldPath[:20] == "resource.attributes." {
				key := fieldPath[20:]
				if val, ok := attrs[key]; ok {
					return fmt.Sprintf("%v", val)
				}
			}
			// Direct key lookup
			if val, ok := attrs[fieldPath]; ok {
				return fmt.Sprintf("%v", val)
			}
		}
		// Check flat resource fields
		if val, ok := l.Resource[fieldPath]; ok {
			return fmt.Sprintf("%v", val)
		}
	}

	return ""
}

// Note: getNestedValue has been replaced by GetNestedString in json_helpers.go
