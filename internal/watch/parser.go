// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// LogLevel represents a parsed log level
type LogLevel string

const (
	LevelTrace   LogLevel = "TRACE"
	LevelDebug   LogLevel = "DEBUG"
	LevelInfo    LogLevel = "INFO"
	LevelWarn    LogLevel = "WARN"
	LevelError   LogLevel = "ERROR"
	LevelFatal   LogLevel = "FATAL"
	LevelUnknown LogLevel = "UNKNOWN"
)

// ParsedLog represents a parsed log line
type ParsedLog struct {
	Timestamp  time.Time
	Level      LogLevel
	Message    string
	Service    string
	Source     string // filename
	Attributes map[string]interface{}
	RawLine    string
	IsJSON     bool
}

// Common timestamp patterns
var timestampPatterns = []struct {
	pattern string
	layout  string
}{
	{`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z`, time.RFC3339Nano},
	{`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`, time.RFC3339},
	{`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+`, "2006-01-02 15:04:05.000"},
	{`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`, "2006-01-02 15:04:05"},
	{`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`, "2006/01/02 15:04:05"},
}

// Level detection patterns
var levelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(TRACE)\b`),
	regexp.MustCompile(`(?i)\b(DEBUG)\b`),
	regexp.MustCompile(`(?i)\b(INFO)\b`),
	regexp.MustCompile(`(?i)\b(WARN(?:ING)?)\b`),
	regexp.MustCompile(`(?i)\b(ERROR|ERR)\b`),
	regexp.MustCompile(`(?i)\b(FATAL|CRITICAL)\b`),
}

// ParseLine parses a single log line
func ParseLine(line, filename, service string) ParsedLog {
	parsed := ParsedLog{
		Timestamp:  time.Now(),
		Level:      LevelUnknown,
		RawLine:    line,
		Source:     filename,
		Service:    service,
		Attributes: make(map[string]interface{}),
	}

	// Try JSON first
	if strings.HasPrefix(strings.TrimSpace(line), "{") {
		if jsonLog := parseJSONLog(line); jsonLog != nil {
			parsed.IsJSON = true
			parsed.Message = jsonLog.Message
			parsed.Level = jsonLog.Level
			parsed.Timestamp = jsonLog.Timestamp
			parsed.Attributes = jsonLog.Attributes
			return parsed
		}
	}

	// Plain text parsing
	parsed.Message = line
	parsed.Timestamp = parseTimestamp(line)
	parsed.Level = parseLevel(line)

	return parsed
}

// JSONLog represents common JSON log structures
type JSONLog struct {
	Message    string
	Level      LogLevel
	Timestamp  time.Time
	Attributes map[string]interface{}
}

func parseJSONLog(line string) *JSONLog {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil
	}

	log := &JSONLog{
		Timestamp:  time.Now(),
		Level:      LevelUnknown,
		Attributes: make(map[string]interface{}),
	}

	// Extract message from common fields
	for _, key := range []string{"message", "msg", "log", "text", "body"} {
		if v, ok := raw[key].(string); ok {
			log.Message = v
			delete(raw, key)
			break
		}
	}

	// Extract level from common fields
	for _, key := range []string{"level", "severity", "lvl", "log.level", "loglevel"} {
		if v, ok := raw[key].(string); ok {
			log.Level = normalizeLevel(v)
			delete(raw, key)
			break
		}
	}

	// Extract timestamp from common fields
	for _, key := range []string{"timestamp", "time", "ts", "@timestamp", "datetime"} {
		if v, ok := raw[key]; ok {
			switch t := v.(type) {
			case string:
				if parsed := parseTimestampString(t); !parsed.IsZero() {
					log.Timestamp = parsed
				}
			case float64:
				// Unix timestamp (seconds or milliseconds)
				if t > 1e12 {
					log.Timestamp = time.UnixMilli(int64(t))
				} else {
					log.Timestamp = time.Unix(int64(t), 0)
				}
			}
			delete(raw, key)
			break
		}
	}

	// Remaining fields become attributes
	log.Attributes = raw

	return log
}

func parseTimestamp(line string) time.Time {
	for _, p := range timestampPatterns {
		re := regexp.MustCompile(p.pattern)
		if match := re.FindString(line); match != "" {
			if t, err := time.Parse(p.layout, match); err == nil {
				return t
			}
		}
	}
	return time.Now()
}

func parseTimestampString(s string) time.Time {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseLevel(line string) LogLevel {
	for i, re := range levelPatterns {
		if re.MatchString(line) {
			switch i {
			case 0:
				return LevelTrace
			case 1:
				return LevelDebug
			case 2:
				return LevelInfo
			case 3:
				return LevelWarn
			case 4:
				return LevelError
			case 5:
				return LevelFatal
			}
		}
	}
	return LevelUnknown
}

func normalizeLevel(s string) LogLevel {
	upper := strings.ToUpper(strings.TrimSpace(s))
	switch {
	case upper == "TRACE":
		return LevelTrace
	case upper == "DEBUG":
		return LevelDebug
	case upper == "INFO" || upper == "INFORMATION":
		return LevelInfo
	case upper == "WARN" || upper == "WARNING":
		return LevelWarn
	case upper == "ERROR" || upper == "ERR":
		return LevelError
	case upper == "FATAL" || upper == "CRITICAL" || upper == "PANIC":
		return LevelFatal
	default:
		return LevelUnknown
	}
}

// ServiceFromFilename extracts service name from a log filename
// e.g., "server-err.log" -> "server", "api.log" -> "api"
func ServiceFromFilename(filename string) string {
	// Remove path
	name := filename
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		name = filename[idx+1:]
	}

	// Remove extension
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[:idx]
	}

	// Remove common suffixes
	for _, suffix := range []string{"-err", "-error", "-out", "-info", "-debug", "-log"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			name = name[:len(name)-len(suffix)]
			break
		}
	}

	if name == "" {
		return "unknown"
	}
	return name
}

// LevelColor returns ANSI color code for a log level
func (l LogLevel) Color() string {
	switch l {
	case LevelTrace:
		return "\033[90m" // Gray
	case LevelDebug:
		return "\033[36m" // Cyan
	case LevelInfo:
		return "\033[32m" // Green
	case LevelWarn:
		return "\033[33m" // Yellow
	case LevelError:
		return "\033[31m" // Red
	case LevelFatal:
		return "\033[35m" // Magenta
	default:
		return "\033[0m" // Reset
	}
}

// Reset returns ANSI reset code
func ColorReset() string {
	return "\033[0m"
}
