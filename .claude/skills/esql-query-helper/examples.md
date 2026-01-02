# ES|QL Query Examples

This document contains real-world ES|QL query examples based on the turbodevlog/telasticat project.

## Table of Contents

- [Transaction Analysis](#transaction-analysis)
- [Span Analysis](#span-analysis)
- [Metrics Queries](#metrics-queries)
- [Log Analysis](#log-analysis)
- [Multi-Query Correlation](#multi-query-correlation)
- [Field Discovery](#field-discovery)

## Transaction Analysis

### Get Transaction Names with Stats

```esql
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 24 hours
| STATS
    tx_count = COUNT(*),
    unique_traces = COUNT_DISTINCT(trace.id),
    min_duration = MIN(transaction.duration.us),
    avg_duration = AVG(transaction.duration.us),
    max_duration = MAX(transaction.duration.us),
    error_count = COUNT(CASE(event.outcome == "failure", 1, null))
  BY transaction.name
| EVAL error_rate = error_count / tx_count * 100
| SORT tx_count DESC
| LIMIT 100
```

**Purpose**: Get comprehensive stats for each transaction name, including count, durations, and error rates.

**Example Output**:
```json
{
  "columns": [
    {"name": "tx_count", "type": "long"},
    {"name": "unique_traces", "type": "long"},
    {"name": "min_duration", "type": "long"},
    {"name": "avg_duration", "type": "double"},
    {"name": "max_duration", "type": "long"},
    {"name": "error_count", "type": "long"},
    {"name": "transaction.name", "type": "keyword"},
    {"name": "error_rate", "type": "double"}
  ],
  "values": [
    [58680, 59122, 1552, 1896.73, 7882, 0, "db.get_item_metadata", 0]
  ]
}
```

### Filter by Service

```esql
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 1 hour
  AND service.name == "frontend"
| STATS
    count = COUNT(*),
    avg_latency_ms = AVG(transaction.duration.us) / 1000000
  BY transaction.name
| SORT avg_latency_ms DESC
```

**Purpose**: Analyze transactions for a specific service.

### Filter by Resource/Environment

```esql
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 24 hours
  AND resource.attributes.deployment.environment == "production"
| STATS count = COUNT(*) BY transaction.name
```

**Purpose**: Analyze production traffic only.

## Span Analysis

### Count Spans per Trace ID

```esql
FROM traces-*
| WHERE processor.event == "span"
  AND @timestamp >= NOW() - 24 hours
| STATS span_count = COUNT(*) BY trace.id
| LIMIT 10000
```

**Purpose**: Get span count for each trace (for correlation with transactions).

**Example Output**:
```json
{
  "values": [
    [2, "5b38d61e7761e9b69f07fc935809e45c"],
    [601, "1283de7f8be68f37d3aa837a31889c04"],
    [548, "42ea72d8b05a3a5f0ce192edb419dae5"]
  ]
}
```

### Get Spans for Specific Trace

```esql
FROM traces-*
| WHERE processor.event == "span"
  AND trace.id == "1283de7f8be68f37d3aa837a31889c04"
| KEEP span.name, span.id, duration, @timestamp
| SORT @timestamp ASC
```

**Purpose**: Get all spans for a trace (for waterfall view).

### Find Slow Spans

```esql
FROM traces-*
| WHERE processor.event == "span"
  AND @timestamp >= NOW() - 1 hour
  AND duration > 1000000000
| STATS
    count = COUNT(*),
    avg_duration_ms = AVG(duration) / 1000000,
    max_duration_ms = MAX(duration) / 1000000
  BY span.name
| SORT max_duration_ms DESC
```

**Purpose**: Find spans that take >1 second (1 billion nanoseconds).

## Metrics Queries

### Get Metric Values Over Time

```esql
FROM metrics-*
| WHERE @timestamp >= NOW() - 5 minutes
  AND EXISTS(metrics.cpu.usage)
| STATS
    min_cpu = MIN(metrics.cpu.usage),
    max_cpu = MAX(metrics.cpu.usage),
    avg_cpu = AVG(metrics.cpu.usage)
  BY service.name
```

**Purpose**: Aggregate metric values by service.

### Time-Series Buckets

```esql
FROM metrics-*
| WHERE @timestamp >= NOW() - 1 hour
  AND EXISTS(metrics.memory.used)
| STATS avg_memory = AVG(metrics.memory.used)
  BY service.name, bucket = DATE_TRUNC(5 minutes, @timestamp)
| SORT bucket ASC
```

**Purpose**: Create 5-minute buckets for graphing metrics over time.

## Log Analysis

### Error Analysis

```esql
FROM logs-*
| WHERE @timestamp >= NOW() - 1 hour
  AND level == "ERROR"
| STATS
    error_count = COUNT(*),
    unique_messages = COUNT_DISTINCT(message)
  BY service.name
| SORT error_count DESC
```

**Purpose**: Count errors per service in the last hour.

### Search Log Messages

```esql
FROM logs-*
| WHERE @timestamp >= NOW() - 24 hours
  AND message LIKE "*database*"
| KEEP @timestamp, service.name, level, message
| SORT @timestamp DESC
| LIMIT 100
```

**Purpose**: Find logs containing specific keywords.

### Group by Log Level

```esql
FROM logs-*
| WHERE @timestamp >= NOW() - 1 hour
| STATS count = COUNT(*) BY level
| SORT count DESC
```

**Purpose**: See distribution of log levels.

## Multi-Query Correlation

### Calculate Spans per Transaction Name

This requires 3 queries because spans don't have `transaction.name`:

**Query 1: Transaction Stats**
```esql
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 24 hours
| STATS
    tx_count = COUNT(*),
    unique_traces = COUNT_DISTINCT(trace.id)
  BY transaction.name
| SORT tx_count DESC
| LIMIT 100
```

**Query 2: Trace ID to Transaction Name Mapping**
```esql
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 24 hours
| KEEP transaction.name, trace.id
| LIMIT 100000
```

**Query 3: Span Counts per Trace ID**
```esql
FROM traces-*
| WHERE processor.event == "span"
  AND @timestamp >= NOW() - 24 hours
| STATS span_count = COUNT(*) BY trace.id
```

**Correlation Logic (Go)**:
```go
// Build map: trace.id -> transaction.name (from Query 2)
traceToTxName := make(map[string]string)

// Build map: trace.id -> span_count (from Query 3)
traceToSpanCount := make(map[string]int64)

// Sum spans per transaction name
txNameToTotalSpans := make(map[string]int64)
for traceID, txName := range traceToTxName {
    spanCount := traceToSpanCount[traceID]
    txNameToTotalSpans[txName] += spanCount
}

// Calculate avg_spans for each transaction
for i := range txStats {
    totalSpans := txNameToTotalSpans[txStats[i].Name]
    txStats[i].AvgSpans = float64(totalSpans) / float64(txStats[i].TraceCount)
}
```

**Purpose**: Calculate accurate average spans per transaction name (not possible in single ES|QL query without LOOKUP JOIN).

## Field Discovery

### List All Available Fields

```bash
curl -s "http://localhost:9200/traces-*/_field_caps?fields=*" \
  | jq '.fields | keys' | head -20
```

### Find Fields Matching Pattern

```bash
curl -s "http://localhost:9200/traces-*/_field_caps?fields=*" \
  | jq '.fields | keys | .[] | select(contains("duration"))'
```

**Output**:
```
"duration"
"transaction.duration.us"
"span.duration.us"
```

### Get Field Type Information

```bash
curl -s "http://localhost:9200/traces-*/_field_caps?fields=transaction.duration.us" \
  | jq .
```

**Output**:
```json
{
  "fields": {
    "transaction.duration.us": {
      "long": {
        "type": "long",
        "searchable": true,
        "aggregatable": true
      }
    }
  }
}
```

## Testing Queries

### Test Query Syntax

```bash
curl -X POST "http://localhost:9200/_query" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "FROM traces-* | WHERE @timestamp >= NOW() - 1 hour | STATS count = COUNT(*)"
  }' | jq .
```

### Get Column Names and Sample Data

```bash
curl -s -X POST "http://localhost:9200/_query" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "FROM traces-* | WHERE @timestamp >= NOW() - 1 hour | LIMIT 5"
  }' | jq '{columns: .columns | map(.name), sample: .values[0:3]}'
```

### Check Query Performance

```bash
curl -s -X POST "http://localhost:9200/_query" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "YOUR_QUERY_HERE"
  }' | jq '{took_ms: .took, rows: (.values | length), partial: .is_partial}'
```

**Example Output**:
```json
{
  "took_ms": 411,
  "rows": 100,
  "partial": false
}
```

## Common Errors and Fixes

### Error: "Unknown column"

**Query**:
```esql
FROM traces-* | WHERE transaction_name == "api"
```

**Error**:
```
Unknown column [transaction_name]
```

**Fix**: Use correct field name:
```esql
FROM traces-* | WHERE transaction.name == "api"
```

### Error: "extraneous input"

**Query**:
```esql
FROM traces-* WHERE @timestamp >= NOW() - 1 hour
```

**Error**:
```
line 1:17: extraneous input 'WHERE' expecting <EOF>
```

**Fix**: Add pipe before WHERE:
```esql
FROM traces-* | WHERE @timestamp >= NOW() - 1 hour
```

### Error: "cannot use != with multi-value field"

**Query**:
```esql
FROM logs-* | WHERE tags != "test"
```

**Fix**: Use NOT IN or check if tags is multi-valued. If needed, filter differently:
```esql
FROM logs-* | WHERE NOT tags == "test"
```

## Performance Tips

### Before Optimization (Slow)

```esql
FROM traces-*
| STATS count = COUNT(*) BY transaction.name
```

**Issues**: Scans ALL data in traces-* indices (could be millions of docs).

### After Optimization (Fast)

```esql
FROM traces-*
| WHERE @timestamp >= NOW() - 24 hours
  AND processor.event == "transaction"
| STATS count = COUNT(*) BY transaction.name
```

**Improvements**:
- Added time filter (reduces data scanned)
- Added processor.event filter (excludes spans)
- Result: 100x faster on large datasets

## Reference Implementation

See `/home/andrewvc/projects/turbodevlog/internal/es/client.go`:
- Lines 1751-1792: `executeESQLQuery()` method
- Lines 1794-1956: `GetTransactionNamesESQL()` with 3-query correlation

This shows real-world ES|QL integration in Go.
