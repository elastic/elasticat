# ElastiCat

AI-powered local development log viewer powered by Elasticsearch and OpenTelemetry.

## Quick Start

```bash
# Build ElastiCat
make build

# Start the Elasticsearch + OTel Collector stack
make up

# Watch a log file and send to Elasticsearch
./bin/elasticat watch /path/to/your/app.log

# View logs in the TUI
./bin/elasticat logs
```

## Example: Watch Your App's Logs

```bash
# Watch a single log file
./bin/elasticat watch server.log

# Watch multiple log files
./bin/elasticat watch *.log

# Watch and display only (don't send to ES)
./bin/elasticat watch --no-send server.log

# Import existing logs (oneshot mode)
./bin/elasticat watch --oneshot old-logs.log
```

## Features

- **AI-Native Debugging**: Built-in Elasticsearch MCP Server for AI assistant integration
- **Universal Log Collection**: Watch any log file and send to Elasticsearch via OTLP
- **Powerful TUI**: Terminal interface with live tailing, full-text search, and ES query DSL support
- **One-Command Setup**: `elasticat up` starts everything you need

## Demo App

Check out the [Stock Tracker Demo](examples/stock-tracker/) - a complete microservices example with:
- React frontend + Go backend (3 services)
- OpenTelemetry instrumentation
- Error scenarios for debugging demos
- Full docker-compose setup

```bash
# Run the demo
cd examples/stock-tracker
docker compose up -d

# Capture logs
docker compose logs -f gateway stock-service portfolio-service > logs/demo.log 2>&1 &

# Watch with ElastiCat (from project root)
cd ../../
./bin/elasticat watch examples/stock-tracker/logs/demo.log

# In another terminal
./bin/elasticat logs
```

## Commands

| Command | Description |
|---------|-------------|
| `elasticat up` | Start Elasticsearch + OTel Collector |
| `elasticat down` | Stop the stack |
| `elasticat watch <file>` | Watch log files and send to ES |
| `elasticat logs` | Open the log viewer TUI |
| `elasticat traces` | Open the traces viewer TUI |
| `elasticat metrics` | Open the metrics viewer TUI |
| `elasticat clear` | Delete all collected logs |

## Documentation

- [PRFAQ.md](PRFAQ.md) - Project vision and capabilities
- [COMPETITIVE_ANALYSIS.md](COMPETITIVE_ANALYSIS.md) - How ElastiCat compares to other tools
- [examples/stock-tracker/](examples/stock-tracker/) - Demo microservices app

## License

Apache 2.0 - See [LICENSE](LICENSE)
