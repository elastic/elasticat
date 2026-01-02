# ElastiCat

AI-powered local development log viewer powered by Elasticsearch and OpenTelemetry.

## Quick Start

```bash
# Start the stack
make up

# View logs
make logs

# Or build and run directly
make build
./bin/elasticat logs
```

## Features

- **AI-Native Debugging**: Built-in Elasticsearch MCP Server for AI assistant integration
- **Universal Log Collection**: Captures both OTel SDK instrumented logs and Docker container logs
- **Powerful TUI**: Terminal interface with live tailing, full-text search, and ES query DSL support
- **One-Command Setup**: `elasticat up` starts everything you need

## Documentation

See [PRFAQ.md](PRFAQ.md) for detailed information about the project vision and capabilities.

See [COMPETITIVE_ANALYSIS.md](COMPETITIVE_ANALYSIS.md) for how ElastiCat compares to other tools.

