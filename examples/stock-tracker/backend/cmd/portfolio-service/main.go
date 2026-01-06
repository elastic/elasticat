// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	appOtel "github.com/elastic/elasticat/examples/stock-tracker/backend/internal/otel"
)

// Holding represents a stock holding in a portfolio
type Holding struct {
	Symbol  string    `json:"symbol"`
	Shares  int       `json:"shares"`
	AvgCost float64   `json:"avg_cost"`
	AddedAt time.Time `json:"added_at"`
}

// Portfolio represents a user's portfolio
type Portfolio struct {
	Holdings []Holding `json:"holdings"`
	Cash     float64   `json:"cash"`
}

// WatchlistItem represents a stock in the watchlist
type WatchlistItem struct {
	Symbol  string    `json:"symbol"`
	AddedAt time.Time `json:"added_at"`
}

var (
	logger    *appOtel.Logger
	tracer    = otel.Tracer("portfolio-service")
	portfolio = Portfolio{
		Holdings: []Holding{
			{Symbol: "AAPL", Shares: 10, AvgCost: 150.00, AddedAt: time.Now().Add(-30 * 24 * time.Hour)},
			{Symbol: "MSFT", Shares: 5, AvgCost: 350.00, AddedAt: time.Now().Add(-15 * 24 * time.Hour)},
		},
		Cash: 10000.00,
	}
	watchlist = []WatchlistItem{
		{Symbol: "NVDA", AddedAt: time.Now().Add(-7 * 24 * time.Hour)},
		{Symbol: "TSLA", AddedAt: time.Now().Add(-3 * 24 * time.Hour)},
	}
	mu sync.RWMutex
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OTel
	shutdown, err := appOtel.Setup(ctx, appOtel.Config{
		ServiceName:    "portfolio-service",
		ServiceVersion: "1.0.0",
	})
	if err != nil {
		panic(err)
	}
	defer shutdown(ctx)

	logger = appOtel.NewLogger("portfolio-service")
	logger.Info(ctx, "Starting portfolio service")

	// Setup router
	r := chi.NewRouter()

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Portfolio endpoints
	r.Get("/portfolio", otelhttp.NewHandler(http.HandlerFunc(getPortfolio), "GetPortfolio").ServeHTTP)
	r.Post("/portfolio/{symbol}", otelhttp.NewHandler(http.HandlerFunc(addToPortfolio), "AddToPortfolio").ServeHTTP)
	r.Delete("/portfolio/{symbol}", otelhttp.NewHandler(http.HandlerFunc(removeFromPortfolio), "RemoveFromPortfolio").ServeHTTP)

	// Watchlist endpoints
	r.Get("/watchlist", otelhttp.NewHandler(http.HandlerFunc(getWatchlist), "GetWatchlist").ServeHTTP)
	r.Post("/watchlist/{symbol}", otelhttp.NewHandler(http.HandlerFunc(addToWatchlist), "AddToWatchlist").ServeHTTP)
	r.Delete("/watchlist/{symbol}", otelhttp.NewHandler(http.HandlerFunc(removeFromWatchlist), "RemoveFromWatchlist").ServeHTTP)

	// Start server
	port := getEnv("PORT", "8082")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info(ctx, "Shutting down portfolio service")
		server.Shutdown(ctx)
	}()

	logger.Info(ctx, "Portfolio service listening", attribute.String("port", port))
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error(ctx, "Server error", attribute.String("error", err.Error()))
	}
}

func getPortfolio(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "getPortfolio")
	defer span.End()

	logger.Info(ctx, "Fetching portfolio")

	mu.RLock()
	defer mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(portfolio)
}

func addToPortfolio(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "addToPortfolio")
	defer span.End()

	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	span.SetAttributes(attribute.String("stock.symbol", symbol))

	// Parse request body
	var req struct {
		Shares int     `json:"shares"`
		Price  float64 `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error(ctx, "Invalid request body", attribute.String("error", err.Error()))
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Shares <= 0 {
		logger.Warn(ctx, "Invalid shares count", attribute.Int("shares", req.Shares))
		http.Error(w, `{"error":"shares must be positive"}`, http.StatusBadRequest)
		return
	}

	logger.Info(ctx, "Adding to portfolio",
		attribute.String("symbol", symbol),
		attribute.Int("shares", req.Shares),
		attribute.Float64("price", req.Price),
	)

	mu.Lock()
	defer mu.Unlock()

	// Check if already in portfolio
	for i, h := range portfolio.Holdings {
		if h.Symbol == symbol {
			// Update existing holding
			totalCost := h.AvgCost*float64(h.Shares) + req.Price*float64(req.Shares)
			totalShares := h.Shares + req.Shares
			portfolio.Holdings[i].Shares = totalShares
			portfolio.Holdings[i].AvgCost = totalCost / float64(totalShares)

			logger.Info(ctx, "Updated existing holding",
				attribute.String("symbol", symbol),
				attribute.Int("total_shares", totalShares),
			)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(portfolio.Holdings[i])
			return
		}
	}

	// Add new holding
	holding := Holding{
		Symbol:  symbol,
		Shares:  req.Shares,
		AvgCost: req.Price,
		AddedAt: time.Now(),
	}
	portfolio.Holdings = append(portfolio.Holdings, holding)

	logger.Info(ctx, "Added new holding", attribute.String("symbol", symbol))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(holding)
}

func removeFromPortfolio(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "removeFromPortfolio")
	defer span.End()

	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	span.SetAttributes(attribute.String("stock.symbol", symbol))

	logger.Info(ctx, "Removing from portfolio", attribute.String("symbol", symbol))

	mu.Lock()
	defer mu.Unlock()

	for i, h := range portfolio.Holdings {
		if h.Symbol == symbol {
			portfolio.Holdings = append(portfolio.Holdings[:i], portfolio.Holdings[i+1:]...)
			logger.Info(ctx, "Removed holding", attribute.String("symbol", symbol))
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	logger.Warn(ctx, "Holding not found", attribute.String("symbol", symbol))
	http.Error(w, `{"error":"holding not found"}`, http.StatusNotFound)
}

func getWatchlist(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "getWatchlist")
	defer span.End()

	logger.Info(ctx, "Fetching watchlist")

	mu.RLock()
	defer mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(watchlist)
}

func addToWatchlist(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "addToWatchlist")
	defer span.End()

	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	span.SetAttributes(attribute.String("stock.symbol", symbol))

	logger.Info(ctx, "Adding to watchlist", attribute.String("symbol", symbol))

	mu.Lock()
	defer mu.Unlock()

	// Check if already in watchlist
	for _, item := range watchlist {
		if item.Symbol == symbol {
			logger.Warn(ctx, "Already in watchlist", attribute.String("symbol", symbol))
			http.Error(w, `{"error":"already in watchlist"}`, http.StatusConflict)
			return
		}
	}

	item := WatchlistItem{
		Symbol:  symbol,
		AddedAt: time.Now(),
	}
	watchlist = append(watchlist, item)

	logger.Info(ctx, "Added to watchlist", attribute.String("symbol", symbol))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func removeFromWatchlist(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "removeFromWatchlist")
	defer span.End()

	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	span.SetAttributes(attribute.String("stock.symbol", symbol))

	logger.Info(ctx, "Removing from watchlist", attribute.String("symbol", symbol))

	mu.Lock()
	defer mu.Unlock()

	for i, item := range watchlist {
		if item.Symbol == symbol {
			watchlist = append(watchlist[:i], watchlist[i+1:]...)
			logger.Info(ctx, "Removed from watchlist", attribute.String("symbol", symbol))
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	logger.Warn(ctx, "Not in watchlist", attribute.String("symbol", symbol))
	http.Error(w, `{"error":"not in watchlist"}`, http.StatusNotFound)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
