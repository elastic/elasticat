# ElastiCat

**An Instant Open Telemetry TUI, powered by Elasticsearch**

[![CI (main)](https://github.com/elastic/elasticat/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/elastic/elasticat/actions/workflows/ci.yml?query=branch%3Amain)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE.txt)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Releases](https://img.shields.io/github/v/release/elastic/elasticat?label=Latest)](https://github.com/elastic/elasticat/releases)

<p align="center">
  <img src="docs/demo.gif" alt="ElastiCat TUI Demo" width="800">
</p>

## Features

- **Log File Watcher** - Tail files like `tail -F` and auto-ingest to Elasticsearch via OTLP
- **Interactive TUI** - Browse logs, metrics, and traces with vim-style navigation
- **CLI Commands** - Query and filter telemetry data in ES from scripts or pipelines with `elasticat {logs|metrics|traces}`
- **One-Command Stack** - Spin up Elasticsearch + OTel Collector with `elasticat up`
- **Multi-Signal Support** - Unified interface for logs, metrics, and traces
- **Perspectives** - Filter by service, host, or any dimension with a single keystroke
- **Kibana Integration** - Open current view in Kibana with `K`

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [The TUI](#the-tui)
- [Demo App](#demo-app)
- [Commands Reference](#commands-reference)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Building from Source](#building-from-source)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| **Docker** or **Podman** | Required for `elasticat up` (local stack) |
| **macOS / Linux / Windows (WSL)** | Pre-built binaries available for all platforms |

No Go installation required for binary installs.

## Quick Start

### 1. Install

```bash
curl -fsSL https://raw.githubusercontent.com/elastic/elasticat/main/install.sh | bash
```

Or download from [GitHub Releases](https://github.com/elastic/elasticat/releases).

### 2. Start the stack

```bash
elasticat up
```

This starts Elasticsearch + OTel Collector (and Kibana by default). Uses Docker if available, otherwise Podman.

### 3. Watch your logs

```bash
# Watch and send to OTLP
elasticat watch ./server.log

# Multiple files / globs
elasticat watch ./logs/*.log ./other.log

# Display only (don't send)
elasticat watch --no-send ./server.log
```

### 4. Open the TUI

```bash
elasticat ui           # Logs (default)
elasticat ui metrics   # Metrics
elasticat ui traces    # Traces
```

### 5. Check status

```bash
elasticat status   # Is Elasticsearch reachable?
elasticat clear    # Delete all telemetry data
```

## The TUI

The terminal UI provides an interactive way to explore your telemetry data with vim-style navigation.

### Logs

<p align="center">
  <img src="docs/images/logs-list.png" alt="Logs View" width="700">
</p>

Browse log entries with syntax highlighting, filter by level, search, and drill into details.

### Metrics

<p align="center">
  <img src="docs/images/metrics-list.png" alt="Metrics List" width="700">
</p>

<p align="center">
  <img src="docs/images/metrics-detail.png" alt="Metrics Detail" width="700">
</p>

View metric summaries and drill into individual metrics with sparkline visualizations.

### Traces

<p align="center">
  <img src="docs/images/trace-detail.png" alt="Trace Detail" width="700">
</p>

Explore distributed traces, view spans, and navigate between transactions.

### Perspectives (Filter by Service)

<p align="center">
  <img src="docs/images/service-list.png" alt="Service Perspective" width="700">
</p>

Press `p` to filter by service, host, or any dimension - works across all signal types.

### Quick Keybindings

| Key | Action | Context |
|-----|--------|---------|
| `j` / `k` | Scroll up/down | All views |
| `Enter` | View details | List views |
| `/` | Search | All views |
| `l` | Change lookback period | All views |
| `p` | Open perspectives (filter by service, host, etc.) | All views |
| `m` | Switch signal (logs/metrics/traces) | All views |
| `f` | Configure visible fields | Logs |
| `s` | Toggle sort order | Logs |
| `0-4` | Filter by log level | Logs |
| `K` | Open in Kibana | All views |
| `h` | Show full help | All views |
| `q` | Quit | All views |

### Detail View

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll content |
| `←` / `→` | Previous/next document |
| `j` | Toggle JSON view |
| `y` | Copy to clipboard |
| `Esc` | Close |

Press `h` in any view to see the complete keybinding reference.

## Demo App

The [Stock Tracker Demo](examples/stock-tracker/) is a complete microservices example (React frontend + Go backend) with OpenTelemetry instrumentation. Great for seeing ElastiCat in action with real data.

```bash
cd examples/stock-tracker
docker compose up -d

# Capture logs to a file
docker compose logs -f gateway stock-service portfolio-service > logs/demo.log 2>&1 &

# From the repo root, watch the captured file
cd ../../
elasticat watch examples/stock-tracker/logs/demo.log

# In another terminal
elasticat ui
```

## Commands Reference

### How It Works

```mermaid
flowchart LR
    subgraph sources [Data Sources]
        LogFiles[Log Files]
        App[Your App]
    end
    
    subgraph elasticat [ElastiCat]
        Watch[watch command]
        TUI[Interactive TUI]
        CLI[CLI Commands]
    end
    
    subgraph infra [Infrastructure]
        OTLP[OTel Collector]
        ES[Elasticsearch]
    end
    
    LogFiles --> Watch
    App --> OTLP
    Watch --> OTLP
    OTLP --> ES
    ES --> TUI
    ES --> CLI
```

### Global Flags

These flags work on all commands:

| Flag | Default | Description |
|------|---------|-------------|
| `--profile` | (none) | Configuration profile to use |
| `--es-url` | `http://localhost:9200` | Elasticsearch URL |
| `--index`, `-i` | `logs-*` | Index pattern (TUI auto-selects based on signal) |
| `--ping-timeout` | `5s` | Elasticsearch ping timeout |

### Stack Management

| Command | Description |
|---------|-------------|
| `elasticat up` | Start Elasticsearch + OTel Collector |
| `elasticat down` | Stop the stack |
| `elasticat status` | Check Elasticsearch connectivity and container status |

**`elasticat up` flags:**
- `--dir`: Compose directory (auto-detected if unset)
- `--kibana` / `--no-kibana`: Enable/disable Kibana (default: enabled)
- `--mcp`: Enable MCP server

**Compose directory auto-detection:**
1. `./docker/docker-compose.yml` (repo checkout)
2. `./docker-compose.yml` (current directory)
3. User data dir (installed binary):
   - macOS: `~/Library/Application Support/elasticat/docker/`
   - Linux: `${XDG_DATA_HOME:-~/.local/share}/elasticat/docker/`
4. Next to executable: `./docker/docker-compose.yml`

### Log Watching

```bash
elasticat watch <file>...
```

Tails files like `tail -F` and sends logs to OTLP.

| Flag | Default | Description |
|------|---------|-------------|
| `--lines`, `-n` | `10` | Initial lines to show from end of file |
| `--service`, `-s` | (from filename) | Service name for OTLP |
| `--otlp` | `localhost:4318` | OTLP/HTTP endpoint |
| `--no-send` | `false` | Display only, don't send to OTLP |
| `--no-color` | `false` | Disable colored output |
| `--oneshot` | `false` | Read all and exit (don't follow) |

**Service name inference:** `gateway.log` becomes `gateway`, `server-err.log` becomes `server`.

### Interactive TUI

```bash
elasticat ui [signal]
```

Signal is one of: `logs` (default), `metrics`, `traces`. The TUI automatically sets the correct index pattern.

### CLI Queries (Non-Interactive)

These commands print tables or JSON - great for scripts and pipelines.

#### `elasticat logs [service]`

Fetch recent log documents.

```bash
elasticat logs                    # Latest logs
elasticat logs gateway            # Filter by service
elasticat logs --json             # Output as NDJSON
elasticat logs -f                 # Follow mode (poll for new)
```

**Custom columns:** Pass field paths after `--`:

```bash
elasticat logs -- @timestamp severity_text body.text
```

#### `elasticat tail [service]`

Same as `logs`, but follow mode is the default.

```bash
elasticat tail
elasticat tail gateway --level ERROR
elasticat tail -s gateway -l WARN
```

#### `elasticat search <query>`

Full-text search using Elasticsearch `query_string`.

```bash
elasticat search "timeout"
elasticat search "connection refused" --service gateway --level ERROR
```

#### `elasticat metrics` / `elasticat traces`

CLI table/JSON output for metrics and traces. **Note:** These require `--index` to be set:

```bash
elasticat metrics --index metrics-*
elasticat traces --index traces-*
```

For interactive exploration, use `elasticat ui metrics` or `elasticat ui traces` instead.

**Shared flags for all CLI queries:**

| Flag | Default | Description |
|------|---------|-------------|
| `--follow`, `-f` | `false` | Poll for new documents |
| `--refresh` | `1000` | Poll interval (ms) |
| `--limit` | `50` | Documents per request |
| `--json` | `false` | Output as NDJSON |
| `--service`, `-s` | - | Filter by service |
| `--level`, `-l` | - | Filter by log level |

### Data Management

```bash
elasticat clear          # Delete all telemetry (prompts for confirmation)
elasticat clear --force  # Delete without prompting
```

Deletes from: your configured index pattern, `metrics-*`, and `traces-*`.

## Configuration

**Precedence:** flags > environment variables > profile > defaults

### Profiles

Profiles let you save and switch between multiple Elasticsearch/Kibana/OTLP configurations (similar to kubectl contexts). Configuration is stored in `~/.config/elasticat/config.yaml`.

#### Quick Start

```bash
# Create a profile
elasticat config set-profile staging \
  --es-url https://staging.es.example.com:9243 \
  --es-api-key '${STAGING_ES_API_KEY}' \
  --kibana-url https://staging.kb.example.com

# Switch to it
elasticat config use-profile staging

# List all profiles
elasticat config get-profiles

# Use a profile for a single command
elasticat --profile staging ui logs
```

#### Profile Commands

| Command | Description |
|---------|-------------|
| `elasticat config set-profile <name>` | Create or update a profile |
| `elasticat config use-profile <name>` | Switch to a profile |
| `elasticat config get-profiles` | List all profiles |
| `elasticat config current-profile` | Show current profile name |
| `elasticat config delete-profile <name>` | Delete a profile |
| `elasticat config view` | Show full config (credentials masked) |
| `elasticat config path` | Show config file path |

#### Profile Settings

| Flag | Description |
|------|-------------|
| `--es-url` | Elasticsearch URL |
| `--es-api-key` | API key (supports `${ENV_VAR}` syntax) |
| `--es-username` | Username for basic auth |
| `--es-password` | Password (supports `${ENV_VAR}` syntax) |
| `--otlp` | OTLP endpoint |
| `--otlp-insecure` | Use insecure OTLP connection |
| `--kibana-url` | Kibana URL |

#### Credential Security

Credentials can be stored as environment variable references (recommended) or plain text:

```yaml
# Recommended: Use env var references
profiles:
  production:
    elasticsearch:
      url: https://prod.es.example.com:9243
      api-key: ${PROD_ES_API_KEY}

# Also works: Plain text (warning shown on creation)
profiles:
  local:
    elasticsearch:
      url: http://localhost:9200
      api-key: actual-key-here
```

Security features:
- Config file is created with mode `0600` (owner read/write only)
- Warnings shown if file has insecure permissions
- `elasticat config view` always masks credential values
- Missing env vars cause immediate errors (fail-fast)

#### Global `--profile` Flag

Override the current profile for a single command:

```bash
elasticat --profile production ui logs
elasticat --profile staging search "error"
```

### Environment Variables

#### Elasticsearch

| Variable | Default | Description |
|----------|---------|-------------|
| `ELASTICAT_ES_URL` | `http://localhost:9200` | Elasticsearch URL |
| `ELASTICAT_ES_INDEX` | `logs-*` | Default index pattern |
| `ELASTICAT_ES_TIMEOUT` | `30s` | Request timeout |
| `ELASTICAT_ES_PING_TIMEOUT` | `5s` | Ping timeout |

#### OTLP

| Variable | Default | Description |
|----------|---------|-------------|
| `ELASTICAT_OTLP_ENDPOINT` | `localhost:4318` | OTLP/HTTP endpoint |
| `ELASTICAT_OTLP_INSECURE` | `true` | Use insecure HTTP |

#### Watch

| Variable | Default | Description |
|----------|---------|-------------|
| `ELASTICAT_WATCH_TAIL_LINES` | `10` | Initial lines to show |
| `ELASTICAT_WATCH_NO_COLOR` | `false` | Disable colors |
| `ELASTICAT_WATCH_NO_SEND` | `false` | Don't send to OTLP |
| `ELASTICAT_WATCH_ONESHOT` | `false` | Read all and exit |
| `ELASTICAT_WATCH_SERVICE` | (empty) | Override service name |

#### TUI

| Variable | Default | Description |
|----------|---------|-------------|
| `ELASTICAT_TUI_TICK_INTERVAL` | `2s` | Auto-refresh interval |
| `ELASTICAT_TUI_LOGS_TIMEOUT` | `10s` | Logs query timeout |
| `ELASTICAT_TUI_METRICS_TIMEOUT` | `30s` | Metrics query timeout |
| `ELASTICAT_TUI_TRACES_TIMEOUT` | `30s` | Traces query timeout |
| `ELASTICAT_TUI_FIELD_CAPS_TIMEOUT` | `10s` | Field caps timeout |
| `ELASTICAT_TUI_AUTO_DETECT_TIMEOUT` | `30s` | Signal auto-detect timeout |

## Troubleshooting

### Elasticsearch not reachable / TUI warns on startup

1. Run `elasticat up`
2. Check `elasticat status`
3. If using a remote cluster, set `--es-url` or `ELASTICAT_ES_URL`

### No metrics/traces showing

- **TUI:** Use `elasticat ui metrics` or `elasticat ui traces` (auto-selects correct index)
- **CLI:** Set `--index metrics-*` or `--index traces-*`

### OTLP sending doesn't work

Ensure the collector is listening on `localhost:4318` and `ELASTICAT_OTLP_INSECURE=true` (default).

### Podman issues

`elasticat up` requires `podman compose` to be installed. Check error output for install hints.

## Building from Source

```bash
make build
./bin/elasticat --help
```

**Development stack commands:**

| Command | Description |
|---------|-------------|
| `make up` / `make down` | Docker Compose wrappers (Docker only) |
| `./bin/elasticat up` | Auto-detects Docker vs Podman |

## Documentation

- [PRFAQ.md](PRFAQ.md) - Project vision and capabilities
- [docs/esql-parity.md](docs/esql-parity.md) - ES|QL parity notes
- [examples/stock-tracker/](examples/stock-tracker/) - Demo microservices app

## Contributing

Contributions welcome! Please read the existing code style and run tests before submitting PRs.

```bash
make test      # Run tests
make lint      # Run linter
make release VERSION=v1.0.0  # Create a release
```

## License

Apache 2.0 - See [LICENSE.txt](LICENSE.txt) and [NOTICE.txt](NOTICE.txt)

