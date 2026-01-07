// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/elastic/elasticat/examples/stock-tracker/backend/internal/otel"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logging returns middleware that logs HTTP requests
func Logging(logger *otel.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Log the request
			attrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
				attribute.Int("http.status_code", wrapped.statusCode),
				attribute.Int64("http.duration_ms", duration.Milliseconds()),
				attribute.String("http.user_agent", r.UserAgent()),
			}

			if wrapped.statusCode >= 500 {
				logger.Error(r.Context(), "HTTP request completed", attrs...)
			} else if wrapped.statusCode >= 400 {
				logger.Warn(r.Context(), "HTTP request completed", attrs...)
			} else {
				logger.Info(r.Context(), "HTTP request completed", attrs...)
			}
		})
	}
}

// CORS returns middleware that handles CORS
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, traceparent, tracestate")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
