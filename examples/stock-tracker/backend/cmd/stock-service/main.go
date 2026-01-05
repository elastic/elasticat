// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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

// Stock represents a stock quote
type Stock struct {
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"change_pct"`
	Volume    int64   `json:"volume"`
	Timestamp string  `json:"timestamp"`
}

// Mock stock data
var stockData = map[string]Stock{
	"AAPL":  {Symbol: "AAPL", Name: "Apple Inc.", Price: 178.50, Change: 2.30, ChangePct: 1.30, Volume: 52341000},
	"GOOG":  {Symbol: "GOOG", Name: "Alphabet Inc.", Price: 141.80, Change: -0.75, ChangePct: -0.53, Volume: 21456000},
	"MSFT":  {Symbol: "MSFT", Name: "Microsoft Corporation", Price: 378.90, Change: 4.20, ChangePct: 1.12, Volume: 18923000},
	"AMZN":  {Symbol: "AMZN", Name: "Amazon.com Inc.", Price: 178.25, Change: 1.15, ChangePct: 0.65, Volume: 31245000},
	"TSLA":  {Symbol: "TSLA", Name: "Tesla Inc.", Price: 248.50, Change: -5.30, ChangePct: -2.09, Volume: 98234000},
	"NVDA":  {Symbol: "NVDA", Name: "NVIDIA Corporation", Price: 875.30, Change: 12.40, ChangePct: 1.44, Volume: 42156000},
	"META":  {Symbol: "META", Name: "Meta Platforms Inc.", Price: 505.75, Change: 8.90, ChangePct: 1.79, Volume: 15678000},
	"NFLX":  {Symbol: "NFLX", Name: "Netflix Inc.", Price: 628.40, Change: -3.20, ChangePct: -0.51, Volume: 3245000},
	"SLOW":  {Symbol: "SLOW", Name: "Slow Response Corp.", Price: 100.00, Change: 0.00, ChangePct: 0.00, Volume: 1000},
	"ERROR": {Symbol: "ERROR", Name: "Error Test Inc.", Price: 0.00, Change: 0.00, ChangePct: 0.00, Volume: 0},
}

var (
	logger      *appOtel.Logger
	tracer      = otel.Tracer("stock-service")
	isAvailable = true
	mu          sync.RWMutex
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OTel
	shutdown, err := appOtel.Setup(ctx, appOtel.Config{
		ServiceName:    "stock-service",
		ServiceVersion: "1.0.0",
	})
	if err != nil {
		panic(err)
	}
	defer shutdown(ctx)

	logger = appOtel.NewLogger("stock-service")
	logger.Info(ctx, "Starting stock service")

	// Setup router
	r := chi.NewRouter()

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		available := isAvailable
		mu.RUnlock()
		if !available {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Get stock quote
	r.Get("/stocks/{symbol}", otelhttp.NewHandler(http.HandlerFunc(getStock), "GetStock").ServeHTTP)

	// List all stocks
	r.Get("/stocks", otelhttp.NewHandler(http.HandlerFunc(listStocks), "ListStocks").ServeHTTP)

	// Chaos endpoints
	r.Post("/chaos/unavailable", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		isAvailable = false
		mu.Unlock()
		logger.Warn(r.Context(), "Service marked as unavailable via chaos endpoint")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Service marked unavailable"))
	})

	r.Post("/chaos/available", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		isAvailable = true
		mu.Unlock()
		logger.Info(r.Context(), "Service restored via chaos endpoint")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Service restored"))
	})

	// Start server
	port := getEnv("PORT", "8081")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info(ctx, "Shutting down stock service")
		server.Shutdown(ctx)
	}()

	logger.Info(ctx, "Stock service listening", attribute.String("port", port))
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error(ctx, "Server error", attribute.String("error", err.Error()))
	}
}

func getStock(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "getStock")
	defer span.End()

	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	span.SetAttributes(attribute.String("stock.symbol", symbol))

	logger.Info(ctx, "Fetching stock quote", attribute.String("symbol", symbol))

	// Check availability
	mu.RLock()
	available := isAvailable
	mu.RUnlock()
	if !available {
		logger.Error(ctx, "Service unavailable", attribute.String("symbol", symbol))
		http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	// Simulate slow response for SLOW symbol
	if symbol == "SLOW" {
		logger.Warn(ctx, "Simulating slow response", attribute.String("symbol", symbol))
		time.Sleep(5 * time.Second)
	}

	// Simulate error for ERROR symbol
	if symbol == "ERROR" {
		logger.Error(ctx, "Simulated error for testing", attribute.String("symbol", symbol))
		http.Error(w, `{"error":"simulated error"}`, http.StatusInternalServerError)
		return
	}

	// Look up stock; if not found, return a synthetic quote unless explicitly testing INVALID
	stock, found := stockData[symbol]
	if !found {
		if symbol == "INVALID" {
			logger.Warn(ctx, "Stock not found", attribute.String("symbol", symbol))
			http.Error(w, `{"error":"stock not found"}`, http.StatusNotFound)
			return
		}
		stock = syntheticStock(symbol)
		logger.Warn(ctx, "Stock not found - returning synthetic quote", attribute.String("symbol", symbol))
	}

	// Add some random variance to make it look live
	stock.Price += (rand.Float64() - 0.5) * 2
	stock.Change = (rand.Float64() - 0.5) * 5
	stock.ChangePct = (stock.Change / stock.Price) * 100
	stock.Timestamp = time.Now().UTC().Format(time.RFC3339)

	logger.Debug(ctx, "Returning stock data",
		attribute.String("symbol", symbol),
		attribute.Float64("price", stock.Price),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stock)
}

func listStocks(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "listStocks")
	defer span.End()

	logger.Info(ctx, "Listing all stocks")

	stocks := make([]Stock, 0, len(stockData))
	for _, stock := range stockData {
		if stock.Symbol != "SLOW" && stock.Symbol != "ERROR" {
			stock.Timestamp = time.Now().UTC().Format(time.RFC3339)
			stocks = append(stocks, stock)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stocks)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// syntheticStock generates a plausible quote so arbitrary symbols don't 404
func syntheticStock(symbol string) Stock {
	price := 50 + rand.Float64()*150
	change := (rand.Float64() - 0.5) * 5
	changePct := (change / price) * 100
	return Stock{
		Symbol:    symbol,
		Name:      fmt.Sprintf("%s Corp.", symbol),
		Price:     price,
		Change:    change,
		ChangePct: changePct,
		Volume:    int64(1_000_000 + rand.Intn(5_000_000)),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

