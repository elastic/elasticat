# Stock Tracker Demo

A demo application showcasing ElastiCat's observability capabilities with a microservices architecture.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Stock Tracker Demo                          │
│                                                                 │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐     │
│   │   Frontend  │────▶│   Gateway   │────▶│   Stock     │     │
│   │   (React)   │     │   (Go)      │     │   Service   │     │
│   │   :3000     │     │   :8080     │     │   (Go)      │     │
│   └─────────────┘     └──────┬──────┘     └─────────────┘     │
│                              │                                  │
│                              │            ┌─────────────┐      │
│                              └───────────▶│  Portfolio  │      │
│                                           │  Service    │      │
│                                           │  (Go)       │      │
│                                           └─────────────┘      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                         JSON logs (stdout)
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     ElastiCat Collection                        │
│                                                                 │
│   docker logs ──▶ log file ──▶ elasticat watch ──▶ OTLP        │
│                                                                 │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐     │
│   │   Log File  │────▶│  ElastiCat  │────▶│    OTel     │     │
│   │  demo.log   │     │   watch     │     │  Collector  │     │
│   └─────────────┘     └─────────────┘     │   :4318     │     │
│                                           └──────┬──────┘     │
│                                                  │              │
│                                                  ▼              │
│                       ┌─────────────┐     ┌─────────────┐      │
│                       │  ElastiCat  │◀────│Elasticsearch│      │
│                       │    TUI      │     │   :9200     │      │
│                       └─────────────┘     └─────────────┘      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- ElastiCat CLI built (`make build` from project root)

### Option A: Use Existing ElastiCat Stack (recommended)

If you already have ElastiCat running (via `make up`), the demo will connect to it:

```bash
# 1. Make sure ElastiCat stack is running (from project root)
make up

# 2. Start the demo app (from stock-tracker directory)
cd examples/stock-tracker
docker compose up -d
```

### Option B: Standalone Mode (includes its own ES + OTel)

If you want an isolated demo with its own Elasticsearch:

```bash
# From the stock-tracker directory
docker compose --profile standalone up -d

# Wait for Elasticsearch to be healthy (about 30-60 seconds)
docker compose ps
```

### Open the App

- **Frontend**: http://localhost:3000
- **Elasticsearch**: http://localhost:9200

### Collect Logs with ElastiCat

The demo services output structured JSON logs to stdout. Use ElastiCat's `watch` command to collect them and send to Elasticsearch.

**Option 1: Watch Docker logs directly (recommended)**

```bash
# Terminal 1: Capture all service logs to a file
docker compose logs -f gateway stock-service portfolio-service > logs/demo.log 2>&1 &

# Terminal 2: Use ElastiCat to watch and send logs to Elasticsearch
cd /path/to/elasticat
./bin/elasticat watch examples/stock-tracker/logs/demo.log
```

**Option 2: Watch individual service logs**

```bash
# Create a logs directory
mkdir -p logs

# In separate terminals, capture each service's logs
docker compose logs -f gateway > logs/gateway.log 2>&1 &
docker compose logs -f stock-service > logs/stock.log 2>&1 &
docker compose logs -f portfolio-service > logs/portfolio.log 2>&1 &

# Watch all log files with ElastiCat
cd /path/to/elasticat
./bin/elasticat watch examples/stock-tracker/logs/*.log
```

**Option 3: Quick one-liner for development**

```bash
# From the elasticat root directory, after starting docker compose
docker compose -f examples/stock-tracker/docker-compose.yml logs -f gateway stock-service portfolio-service 2>&1 | \
  ./bin/elasticat watch --service stock-tracker /dev/stdin
```

### Traffic Generation (automated)

The stock tracker Docker Compose stack now launches the Playwright-based traffic generator automatically. This service walks a headless Chromium browser through searches, portfolio/watchlist updates, chaos commands, and error scenarios so that OpenTelemetry traces, metrics, and logs are continually produced without manual input.

When you run `docker compose up -d`, you can inspect its output with:

```bash
docker compose logs traffic-generator
```

If you need to override the target host (for example you are running the frontend on a different URL), set `STOCK_TRACKER_URL` before bringing the stack up:

```bash
STOCK_TRACKER_URL=http://localhost:3000 docker compose up -d
```

### View Logs in ElastiCat TUI

Once logs are being collected, open another terminal to view them:

```bash
# From the main elasticat directory
./bin/elasticat logs
```

You should see logs from all three services with trace IDs for correlation.

## Demo Scenarios

### 1. Normal Flow - Search and Track Stocks

1. Open http://localhost:3000
2. Search for popular stocks: AAPL, GOOG, MSFT, NVDA
3. Watch logs appear in ElastiCat showing requests flowing through services
4. Add stocks to portfolio and watchlist
5. In ElastiCat, filter by service name to see each service's logs

### 2. Error Handling - Stock Not Found

1. Search for "INVALID" in the app
2. Watch ElastiCat show WARN logs from the stock service
3. See how the error propagates through the gateway

### 3. Slow Response Simulation

1. Search for "SLOW" in the app
2. Notice the 5-second delay
3. In ElastiCat, see WARN logs about slow response simulation

### 4. Service Error

1. Search for "ERROR" in the app
2. Watch ERROR logs appear in ElastiCat
3. See the 500 response flow through the gateway

### 5. Chaos Engineering - Service Failure

1. Click "Kill Stock Service" button in the UI
2. Try searching for any stock - you'll get errors
3. Watch ERROR logs cascade in ElastiCat
4. Click "Restore Stock Service" to recover
5. Search again - service is working

### 6. Rate Limiting

1. Rapidly search for stocks (click search multiple times quickly)
2. After 10 requests/second, you'll see rate limit errors
3. ElastiCat will show WARN logs about rate limiting

## ElastiCat Features to Explore

### Filtering by Service

In ElastiCat TUI, you can filter logs by service:
- `api-gateway` - API routing and rate limiting
- `stock-service` - Stock quote fetching
- `portfolio-service` - Portfolio and watchlist management
- `stock-tracker-frontend` - Browser traces (if configured)

### Trace Correlation

Each request generates a trace ID that flows through all services. In ElastiCat:
1. Find a log entry
2. Note the trace_id
3. Search for that trace_id to see all related logs across services

### Log Levels

- **INFO**: Normal operations, request completion
- **WARN**: Rate limiting, not-found errors, slow responses
- **ERROR**: Service failures, upstream errors
- **DEBUG**: Detailed internal state (if enabled)

## Services

| Service | Port | Description |
|---------|------|-------------|
| Frontend | 3000 | React UI for stock tracking |
| Gateway | 8080 | API gateway with rate limiting |
| Stock Service | 8081 | Stock quote provider (mock data) |
| Portfolio Service | 8082 | Portfolio and watchlist management |
| OTel Collector | 4318 | Telemetry collection |
| Elasticsearch | 9200 | Log and trace storage |

## API Endpoints

### Stocks
- `GET /api/stocks` - List all available stocks
- `GET /api/stocks/:symbol` - Get quote for a symbol

### Portfolio
- `GET /api/portfolio` - Get portfolio holdings
- `POST /api/portfolio/:symbol` - Add to portfolio (body: `{"shares": 10, "price": 150.00}`)
- `DELETE /api/portfolio/:symbol` - Remove from portfolio

### Watchlist
- `GET /api/watchlist` - Get watchlist
- `POST /api/watchlist/:symbol` - Add to watchlist
- `DELETE /api/watchlist/:symbol` - Remove from watchlist

### Chaos Engineering
- `POST /api/chaos/kill-stock-service` - Mark stock service unavailable
- `POST /api/chaos/restore-stock-service` - Restore stock service

## Special Test Symbols

| Symbol | Behavior |
|--------|----------|
| `SLOW` | 5-second delay before response |
| `ERROR` | Returns 500 error |
| `INVALID` | Returns 404 not found |

## Stopping

```bash
docker compose down

# To also remove volumes (Elasticsearch data):
docker compose down -v
```

## Development

### Run Backend Services Locally

```bash
cd backend

# Run each service in separate terminals
go run ./cmd/gateway
go run ./cmd/stock-service
go run ./cmd/portfolio-service
```

### Run Frontend Locally

```bash
cd frontend
bun install
bun run dev
```

### Environment Variables

**Backend Services:**
- `PORT` - Server port
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTel collector endpoint

**Frontend:**
- `VITE_API_URL` - Backend API URL
- `VITE_OTLP_ENDPOINT` - OTel collector endpoint for browser traces

