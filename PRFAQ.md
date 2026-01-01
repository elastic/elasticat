# TurboDevLog PRFAQ

---

## Press Release

### TurboDevLog: AI-Powered Local Development Log Analysis

**The first local development log tool with built-in AI integration via Elasticsearch MCP Server**

---

**FOR IMMEDIATE RELEASE**

Today we announce TurboDevLog, an open-source tool that transforms how developers debug their local applications. With a single command, developers get a complete log collection and analysis stack—powered by Elasticsearch and OpenTelemetry—with first-class AI assistant integration.

### The Problem

Developers spend an estimated 20-50% of their time debugging. A significant portion of that time is wasted on log analysis: grepping through scattered log files, correlating events across services, and manually piecing together what went wrong.

Existing solutions fall short:

- **ELK Stack**: Powerful but complex to set up, resource-heavy, overkill for local development
- **Dozzle/Lazydocker**: Simple but no persistent storage, search, or structured log support
- **Cloud observability tools**: Require sending data off-machine, have cost/privacy concerns for local dev
- **grep/less/tail**: Manual, tedious, doesn't scale to multiple services

Most critically, **none of these tools integrate with AI assistants**—even though AI excels at pattern recognition, correlation, and summarization tasks that are core to log analysis.

### The Solution

TurboDevLog provides:

1. **One-command setup**: `turbodevlog up` starts Elasticsearch, OpenTelemetry Collector, and optional Kibana
2. **Universal log collection**: Captures both OTel SDK instrumented logs and Docker container stdout/stderr
3. **Powerful TUI**: Vim-like terminal interface with live tailing, full-text search, and ES query DSL support
4. **AI-Native Debugging**: Built-in Elasticsearch MCP Server allows Claude, Cursor, and VS Code to query your logs directly

### Key Differentiator: AI-Assisted Debugging

TurboDevLog is the first local development tool to include the [Elasticsearch MCP Server](https://www.elastic.co/docs/solutions/search/agent-builder/mcp-server). This enables a revolutionary debugging workflow:

**Before TurboDevLog:**
```
Developer: *sees 500 error in browser*
Developer: *opens terminal, runs docker logs*
Developer: *scrolls through output, tries to find relevant lines*
Developer: *opens another terminal for the database service*
Developer: *cross-references timestamps manually*
Developer: *20 minutes later, maybe finds the root cause*
```

**With TurboDevLog:**
```
Developer: *sees 500 error in browser*
Developer: "Hey Claude, find all errors in my payment service
            from the last 5 minutes and explain what's happening"
Claude: *queries logs via MCP*
Claude: "I found 3 related errors. The root cause appears to be
         a database connection timeout. Here's the trace..."
```

### How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                    TurboDevLog Stack                         │
│                                                              │
│   Your App ──► OTel Collector ──► Elasticsearch             │
│   (OTel SDK)      :4317/:4318         :9200                 │
│                       │                  │                   │
│   Docker Logs ────────┘                  │                   │
│   (via filelog)                          ▼                   │
│                                    ┌──────────┐              │
│                                    │ MCP      │◄── AI Tools  │
│                                    │ Server   │   (Claude,   │
│                                    └──────────┘    Cursor)   │
│                                          │                   │
└──────────────────────────────────────────│───────────────────┘
                                           ▼
                                    ┌──────────┐
                                    │ TUI      │◄── Developer
                                    │ (Go CLI) │
                                    └──────────┘
```

### Customer Quotes

> "I used to spend 20 minutes grepping through logs. Now I just ask Claude 'what's causing the 500 errors in my API?' and it finds the root cause in seconds. It's like having a senior engineer who's read every line of my logs."
> — *Sarah Chen, Senior Backend Engineer*

> "The MCP integration is a game-changer. My AI assistant can now see what's actually happening in my services, not just guess based on my descriptions."
> — *Marcus Johnson, Full-Stack Developer*

> "We tried Dozzle, it was too basic. We tried the full ELK stack, it was overkill. TurboDevLog is the Goldilocks solution—powerful enough for real debugging, simple enough for local dev."
> — *Alex Rivera, DevOps Lead*

### Getting Started

```bash
# Install the CLI
brew install turbodevlog

# Start the stack
turbodevlog up

# Configure your AI assistant (one-time)
turbodevlog mcp-config >> ~/.config/claude/config.json

# Open the TUI
turbodevlog logs
```

### Availability

TurboDevLog is open-source under the Apache 2.0 license. Available now at [github.com/andrewvc/turbodevlog](https://github.com/andrewvc/turbodevlog).

---

## Frequently Asked Questions

### Customer FAQ

**Q: Why should I use Elasticsearch instead of something lighter like Loki or DuckDB?**

A: Elasticsearch provides three key advantages for log analysis:
1. **Full-text search**: Natural language queries across all your logs
2. **Aggregations**: Complex analytics (error rates, percentiles, trends)
3. **MCP Server**: The Elasticsearch MCP Server enables AI assistant integration—this is the killer feature that lighter alternatives don't support

If you don't need AI integration or complex queries, lighter alternatives may be sufficient. But if you want to debug with AI assistance, Elasticsearch is currently the only option with a production-ready MCP server.

**Q: How much memory does this require?**

A: Approximately 2-3GB total:
- Elasticsearch: ~1.5-2GB (JVM heap)
- OTel Collector: ~100MB
- Kibana (optional): ~500MB

This is appropriate for modern development machines. For resource-constrained environments, you can skip Kibana and tune ES heap settings.

**Q: Does this work with my existing OpenTelemetry instrumentation?**

A: Yes. TurboDevLog exposes standard OTLP endpoints (gRPC on :4317, HTTP on :4318). If your application is already instrumented with the OTel SDK, just point it at `http://localhost:4317` and your logs will appear in TurboDevLog.

**Q: What about traces and metrics?**

A: The initial release focuses on logs. Traces and metrics support are on the roadmap—Elasticsearch can store all three signal types, and the architecture supports adding them incrementally.

**Q: Can I use this in production?**

A: TurboDevLog is designed for local development. While the underlying technologies (Elasticsearch, OTel Collector) are production-ready, the default configuration prioritizes simplicity over durability and security. For production, consider Elastic Cloud or a properly configured self-hosted deployment.

**Q: How is this different from just running docker-elk?**

A: Several ways:
1. **Integrated OTel Collector**: docker-elk uses Logstash; we use the OTel Collector with native support for OTLP, Docker logs, and modern observability patterns
2. **MCP Server pre-configured**: Out-of-the-box AI assistant integration
3. **Purpose-built TUI**: Optimized for developer workflows, not ops dashboards
4. **Opinionated defaults**: Tuned for local development (no auth, minimal resource usage, simple index patterns)

**Q: What AI assistants are supported?**

A: Any MCP-compatible client:
- Claude Desktop
- Cursor
- VS Code (with MCP extension)
- Any tool supporting the Model Context Protocol

**Q: Is my log data sent anywhere?**

A: No. All data stays on your local machine. The MCP server runs locally and only your AI assistant (running locally or via API) can query it with your permission.

### Internal/Technical FAQ

**Q: Why Go for the TUI instead of Rust?**

A: Trade-off between development speed and performance:
- **Go**: Bubble Tea is the most mature TUI framework, excellent developer experience, fast enough for this use case, easier contributor onboarding
- **Rust**: Ratatui is good but less mature ecosystem, steeper learning curve
- The Elasticsearch queries are the bottleneck, not the TUI rendering

**Q: Why not use Kibana as the primary interface?**

A: Different target users and workflows:
- **Kibana**: Designed for ops teams, dashboard-focused, browser-based
- **TUI**: Designed for developers, code-focused, terminal-native

Developers live in their terminals. A TUI fits their workflow; switching to a browser breaks flow. We include Kibana as optional for when you need its advanced visualization capabilities.

**Q: How do you handle Docker log rotation and cleanup?**

A: The OTel Collector's filelog receiver handles this automatically—it tracks file positions and handles rotation. For Elasticsearch storage, we implement index lifecycle management (ILM) policies that automatically delete logs older than 7 days by default.

**Q: What's the security model for the MCP server?**

A: For local development, we prioritize simplicity:
- MCP server binds to localhost only
- No authentication required (local traffic only)
- API keys can optionally be configured for shared environments

This matches the threat model of local development tools—if an attacker has local access, authentication isn't your primary concern.

**Q: How does the OTel Collector parse Docker logs?**

A: Docker logs (in JSON format) are parsed using the filelog receiver with JSON operators:
1. Receiver tails `/var/lib/docker/containers/*/*.log`
2. JSON parser extracts timestamp, stream (stdout/stderr), and log content
3. Attributes are mapped to Elasticsearch fields via the ES exporter

Container labels are also captured, enabling service-name filtering based on Docker Compose service names.

**Q: What's the index strategy in Elasticsearch?**

A: Simple and opinionated for local dev:
- Single index pattern: `turbodevlog-logs-*`
- Daily index rollover
- 7-day retention by default
- No index templates or complex mappings—rely on dynamic mapping

This trades some query performance for setup simplicity, appropriate for the local dev use case.

**Q: Can this scale beyond local development?**

A: The architecture can scale, but that's not the goal. For scaling:
- Elasticsearch: Add nodes, configure replicas
- OTel Collector: Deploy as an agent/gateway pattern
- Use managed Elastic Cloud for production

We intentionally keep the default config simple. Scaling complexity belongs in production tooling like Elastic Cloud or managed Kubernetes operators.
