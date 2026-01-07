// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"

	"github.com/elastic/elasticat/examples/stock-tracker/backend/internal/middleware"
	appOtel "github.com/elastic/elasticat/examples/stock-tracker/backend/internal/otel"
)

var (
	logger              *appOtel.Logger
	tracer              = otel.Tracer("api-gateway")
	stockServiceURL     = getEnv("STOCK_SERVICE_URL", "http://localhost:8081")
	portfolioServiceURL = getEnv("PORTFOLIO_SERVICE_URL", "http://localhost:8082")
	httpClient          *http.Client

	// Rate limiting
	rateLimiter = &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    10,
		window:   time.Second,
	}
)

// RateLimiter implements simple rate limiting
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old requests
	var valid []time.Time
	for _, t := range rl.requests[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.requests[key] = valid

	// Check limit
	if len(valid) >= rl.limit {
		return false
	}

	rl.requests[key] = append(rl.requests[key], now)
	return true
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OTel
	shutdown, err := appOtel.Setup(ctx, appOtel.Config{
		ServiceName:    "api-gateway",
		ServiceVersion: "1.0.0",
	})
	if err != nil {
		panic(err)
	}
	defer shutdown(ctx)

	logger = appOtel.NewLogger("api-gateway")
	logger.Info(ctx, "Starting API gateway")

	// HTTP client with OTel instrumentation
	httpClient = &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   10 * time.Second,
	}

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.CORS)
	r.Use(middleware.Logging(logger))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Stock routes
	r.Route("/api/stocks", func(r chi.Router) {
		r.Get("/", proxyHandler(stockServiceURL, "/stocks"))
		r.Get("/{symbol}", proxyHandler(stockServiceURL, "/stocks/{symbol}"))
	})

	// Portfolio routes
	r.Route("/api/portfolio", func(r chi.Router) {
		r.Get("/", proxyHandler(portfolioServiceURL, "/portfolio"))
		r.Post("/{symbol}", proxyHandler(portfolioServiceURL, "/portfolio/{symbol}"))
		r.Delete("/{symbol}", proxyHandler(portfolioServiceURL, "/portfolio/{symbol}"))
	})

	// Watchlist routes
	r.Route("/api/watchlist", func(r chi.Router) {
		r.Get("/", proxyHandler(portfolioServiceURL, "/watchlist"))
		r.Post("/{symbol}", proxyHandler(portfolioServiceURL, "/watchlist/{symbol}"))
		r.Delete("/{symbol}", proxyHandler(portfolioServiceURL, "/watchlist/{symbol}"))
	})

	// Chaos engineering endpoints
	r.Route("/api/chaos", func(r chi.Router) {
		r.Post("/kill-stock-service", chaosKillStockService)
		r.Post("/restore-stock-service", chaosRestoreStockService)
	})

	// Start server
	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info(ctx, "Shutting down API gateway")
		server.Shutdown(ctx)
	}()

	logger.Info(ctx, "API gateway listening",
		attribute.String("port", port),
		attribute.String("stock_service", stockServiceURL),
		attribute.String("portfolio_service", portfolioServiceURL),
	)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error(ctx, "Server error", attribute.String("error", err.Error()))
	}
}

func proxyHandler(serviceURL, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "proxy")
		defer span.End()

		// Rate limiting
		clientIP := r.RemoteAddr
		if !rateLimiter.Allow(clientIP) {
			logger.Warn(ctx, "Rate limit exceeded", attribute.String("client_ip", clientIP))
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// Build target URL
		targetPath := path
		if symbol := chi.URLParam(r, "symbol"); symbol != "" {
			targetPath = fmt.Sprintf("/stocks/%s", symbol)
			if path == "/portfolio/{symbol}" {
				targetPath = fmt.Sprintf("/portfolio/%s", symbol)
			} else if path == "/watchlist/{symbol}" {
				targetPath = fmt.Sprintf("/watchlist/%s", symbol)
			}
		}
		targetURL := serviceURL + targetPath

		span.SetAttributes(
			attribute.String("proxy.target_url", targetURL),
			attribute.String("proxy.method", r.Method),
		)

		logger.Debug(ctx, "Proxying request",
			attribute.String("target", targetURL),
			attribute.String("method", r.Method),
		)

		// Create proxy request
		proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, r.Body)
		if err != nil {
			logger.Error(ctx, "Failed to create proxy request", attribute.String("error", err.Error()))
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}

		// Copy headers and inject trace context
		for key, values := range r.Header {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(proxyReq.Header))

		// Make request
		resp, err := httpClient.Do(proxyReq)
		if err != nil {
			logger.Error(ctx, "Upstream request failed",
				attribute.String("target", targetURL),
				attribute.String("error", err.Error()),
			)
			http.Error(w, `{"error":"upstream service unavailable"}`, http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Write status and body
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)

		logger.Info(ctx, "Proxy request completed",
			attribute.String("target", targetURL),
			attribute.Int("status", resp.StatusCode),
		)
	}
}

func chaosKillStockService(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "chaosKillStockService")
	defer span.End()

	logger.Warn(ctx, "Chaos: Marking stock service as unavailable")

	// Call stock service chaos endpoint
	req, _ := http.NewRequestWithContext(ctx, "POST", stockServiceURL+"/chaos/unavailable", nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error(ctx, "Failed to reach stock service", attribute.String("error", err.Error()))
		http.Error(w, `{"error":"failed to reach stock service"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stock service marked unavailable"})
}

func chaosRestoreStockService(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "chaosRestoreStockService")
	defer span.End()

	logger.Info(ctx, "Chaos: Restoring stock service")

	// Call stock service chaos endpoint
	req, _ := http.NewRequestWithContext(ctx, "POST", stockServiceURL+"/chaos/available", nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error(ctx, "Failed to reach stock service", attribute.String("error", err.Error()))
		http.Error(w, `{"error":"failed to reach stock service"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stock service restored"})
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
