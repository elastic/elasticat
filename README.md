# ElastiCat

AI-powered local development log viewer powered by Elasticsearch and OpenTelemetry.

[![CI (main)](https://github.com/elastic/elasticat/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/elastic/elasticat/actions/workflows/ci.yml?query=branch%3Amain)

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/elastic/elasticat/main/install.sh | bash
```

Or download directly from [GitHub Releases](https://github.com/elastic/elasticat/releases).

Latest main artifacts (CI): [Download the `elasticat-linux-amd64`, `elasticat-darwin-arm64`, or `elasticat-windows-amd64.exe` artifact](https://github.com/elastic/elasticat/actions/workflows/ci.yml?query=branch%3Amain). GitHub login required to access artifacts.

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

## Configuration

- **Precedence**: flags > environment variables > defaults.
- **Environment variables** (Go duration strings where applicable, e.g., `5s`, `1m`):
  - `ELASTICAT_ES_URL`, `ELASTICAT_ES_INDEX`, `ELASTICAT_ES_PING_TIMEOUT`
  - `ELASTICAT_OTLP_ENDPOINT`, `ELASTICAT_OTLP_INSECURE`
  - `ELASTICAT_WATCH_TAIL_LINES`, `ELASTICAT_WATCH_SERVICE`, `ELASTICAT_WATCH_NO_COLOR`, `ELASTICAT_WATCH_NO_SEND`, `ELASTICAT_WATCH_ONESHOT`
  - `ELASTICAT_TUI_TICK_INTERVAL`, `ELASTICAT_TUI_LOGS_TIMEOUT`, `ELASTICAT_TUI_METRICS_TIMEOUT`, `ELASTICAT_TUI_TRACES_TIMEOUT`, `ELASTICAT_TUI_FIELD_CAPS_TIMEOUT`, `ELASTICAT_TUI_AUTO_DETECT_TIMEOUT`
- **Fail-fast**: invalid values (e.g., bad durations) cause startup errors.

## Documentation

- [PRFAQ.md](PRFAQ.md) - Project vision and capabilities
- [COMPETITIVE_ANALYSIS.md](COMPETITIVE_ANALYSIS.md) - How ElastiCat compares to other tools
- [examples/stock-tracker/](examples/stock-tracker/) - Demo microservices app

## Releasing

To create a new release:

```bash
# Run validation and create a release tag
make release VERSION=v1.0.0
```

This will:
1. Run all tests
2. Check code formatting
3. Verify license headers
4. Create an annotated git tag
5. Push the tag to GitHub

GitHub Actions will then automatically:
- Validate all checks pass
- Build binaries for Linux, macOS, and Windows
- Create distribution archives containing the binary, LICENSE.txt, NOTICE.txt, and README.md
- Publish a GitHub Release with auto-generated release notes

**Versioning**: We use [Semantic Versioning](https://semver.org/) (e.g., `v1.0.0`, `v1.0.1`, `v2.0.0`).

## License

Apache 2.0 - See [LICENSE.txt](LICENSE.txt) and [NOTICE.txt](NOTICE.txt)
