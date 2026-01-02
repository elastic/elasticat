---
name: esql-query-helper
description: Generates, validates, and executes ES|QL queries against Elasticsearch. Use when working with ES|QL queries, querying traces/logs/metrics, building aggregations, troubleshooting query errors, or optimizing query performance.
allowed-tools: Read, Bash(curl:*), Bash(./telasticat:*), WebFetch(domain:www.elastic.co), Grep, Glob
---

# ES|QL Query Helper

## Overview

This skill helps you write, validate, execute, and optimize ES|QL queries against your Elasticsearch cluster. ES|QL is Elasticsearch's piped query language for analyzing data with SQL-like syntax.

## Key Capabilities

- **Query Generation**: Create ES|QL queries with proper syntax for traces, logs, and metrics
- **Query Validation**: Test queries via curl against `/_query` endpoint
- **Query Execution**: Run queries and parse results
- **Performance Optimization**: Suggest improvements for large datasets
- **Error Diagnosis**: Fix syntax errors and explain ES responses
- **Field Discovery**: Help find available fields using field_caps

## ES|QL Syntax Reference

### Basic Structure

```esql
FROM index_pattern
| WHERE condition
| STATS aggregation BY field
| EVAL calculated_field = expression
| SORT field DESC
| LIMIT n
```

### Core Commands

- **FROM**: Specify index pattern (e.g., `FROM traces-*`, `FROM logs-*`)
- **WHERE**: Filter rows (use `AND`, `OR`, `==`, `!=`, `>`, `<`, `>=`, `<=`)
- **KEEP**: Select specific fields (e.g., `KEEP field1, field2`)
- **DROP**: Exclude fields
- **STATS**: Aggregate data (e.g., `COUNT(*)`, `AVG(field)`, `SUM(field)`)
- **EVAL**: Create calculated fields (e.g., `EVAL rate = errors / total * 100`)
- **SORT**: Order results (e.g., `SORT @timestamp DESC`)
- **LIMIT**: Restrict result count (default max: 10,000 rows)

### Time Filters

```esql
WHERE @timestamp >= NOW() - 1 hour
WHERE @timestamp >= NOW() - 24 hours
WHERE @timestamp >= NOW() - 7 days
```

### Aggregation Functions

- **COUNT()**: Count rows
- **COUNT_DISTINCT(field)**: Unique values (uses HyperLogLog++)
- **AVG(field)**: Average
- **MIN(field)**: Minimum
- **MAX(field)**: Maximum
- **SUM(field)**: Sum
- **PERCENTILE(field, percentile)**: e.g., `PERCENTILE(latency, 95)`
- **MEDIAN(field)**: 50th percentile

### Conditional Aggregation

```esql
COUNT(CASE(condition, 1, null))
```

Example:
```esql
COUNT(CASE(event.outcome == "failure", 1, null))
```

## Common Patterns

### Pattern 1: Filter and Count

```esql
FROM logs-*
| WHERE @timestamp >= NOW() - 1 hour
  AND level == "ERROR"
| STATS error_count = COUNT(*) BY service.name
| SORT error_count DESC
```

### Pattern 2: Transaction Analysis (OpenTelemetry)

```esql
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 24 hours
| STATS
    tx_count = COUNT(*),
    unique_traces = COUNT_DISTINCT(trace.id),
    min_duration = MIN(transaction.duration.us),
    avg_duration = AVG(transaction.duration.us),
    max_duration = MAX(transaction.duration.us)
  BY transaction.name
| EVAL avg_duration_ms = avg_duration / 1000000
| SORT tx_count DESC
| LIMIT 100
```

### Pattern 3: Correlation Across Document Types

When you need to correlate data (e.g., count spans per transaction name):

**Approach**: Use multiple queries with client-side correlation

```esql
-- Query 1: Get transaction stats
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 24 hours
| STATS unique_traces = COUNT_DISTINCT(trace.id) BY transaction.name

-- Query 2: Map trace.id to transaction.name
FROM traces-*
| WHERE processor.event == "transaction"
  AND @timestamp >= NOW() - 24 hours
| KEEP transaction.name, trace.id
| LIMIT 100000

-- Query 3: Count spans per trace.id
FROM traces-*
| WHERE processor.event == "span"
  AND @timestamp >= NOW() - 24 hours
| STATS span_count = COUNT(*) BY trace.id

-- Then correlate in code to sum spans per transaction name
```

### Pattern 4: Metrics Aggregation

```esql
FROM metrics-*
| WHERE @timestamp >= NOW() - 5 minutes
| STATS
    min_value = MIN(metrics.cpu.usage),
    max_value = MAX(metrics.cpu.usage),
    avg_value = AVG(metrics.cpu.usage)
  BY service.name
```

## How to Use This Skill

### 1. Generate a Query

**User**: "Write an ES|QL query to find all errors in traces from the past hour"

**Skill will**:
- Analyze the requirement
- Generate proper ES|QL syntax
- Include appropriate filters and aggregations
- Add comments explaining the query

### 2. Validate the Query

```bash
curl -X POST "http://localhost:9200/_query" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "FROM traces-* | WHERE @timestamp >= NOW() - 1 hour | STATS count = COUNT(*)"
  }' | jq .
```

### 3. Test with Real Data

```bash
# Get column names and first few rows
curl -s -X POST "http://localhost:9200/_query" \
  -H 'Content-Type: application/json' \
  -d '{"query": "YOUR_QUERY | LIMIT 5"}' \
  | jq '{columns: .columns | map(.name), sample: .values[0:3]}'
```

### 4. Optimize Performance

**Before optimization**:
```esql
FROM traces-*
| STATS count = COUNT(*) BY transaction.name
```

**After optimization** (add time filter):
```esql
FROM traces-*
| WHERE @timestamp >= NOW() - 24 hours
| STATS count = COUNT(*) BY transaction.name
```

## Best Practices

### Always Specify Time Ranges

Without time filters, ES|QL scans all data:

❌ **Bad**:
```esql
FROM logs-* | STATS count = COUNT(*)
```

✅ **Good**:
```esql
FROM logs-*
| WHERE @timestamp >= NOW() - 24 hours
| STATS count = COUNT(*)
```

### Filter Before Aggregating

Put WHERE clauses before STATS to reduce data processed:

✅ **Good**:
```esql
FROM traces-*
| WHERE @timestamp >= NOW() - 1 hour
  AND processor.event == "transaction"
| STATS count = COUNT(*) BY transaction.name
```

### Use KEEP for Large Result Sets

When fetching raw documents, select only needed fields:

```esql
FROM traces-*
| WHERE @timestamp >= NOW() - 1 hour
| KEEP transaction.name, trace.id, @timestamp
| LIMIT 10000
```

### Handle Duration Conversions

OpenTelemetry stores durations in nanoseconds. Convert to milliseconds:

```esql
| EVAL duration_ms = transaction.duration.us / 1000000
```

### Check Field Names First

Use field_caps or grep through code to find exact field names:

```bash
curl -s "http://localhost:9200/traces-*/_field_caps?fields=*" \
  | jq '.fields | keys' | grep -i duration
```

## Troubleshooting

### Syntax Errors

**Error**: `line 1:X: extraneous input 'X' expecting Y`

**Fix**: Check for:
- Missing pipes between commands
- Typos in command names (FROM, WHERE, STATS, etc.)
- Incorrect operator syntax (`==` not `=` for equality)

### Field Not Found

**Error**: `Unknown column [field_name]`

**Fix**: Verify field exists in index mapping:

```bash
curl -s "http://localhost:9200/traces-*/_field_caps?fields=field_name"
```

### Results Limited to 10,000 Rows

ES|QL has a default limit of 10,000 rows for KEEP/SELECT queries.

**Solutions**:
- Use aggregations (STATS) instead of raw documents
- Add more specific WHERE filters to reduce result set
- Use pagination if the ES Go client supports it

### Performance Issues

If queries are slow:
1. Add time range filters (`@timestamp >= NOW() - duration`)
2. Add more specific WHERE conditions
3. Reduce aggregation cardinality (fewer unique values in BY clause)
4. Use COUNT_DISTINCT sparingly (it's approximate via HyperLogLog++)

## Integration with Telasticat

This project has ES|QL integration in `internal/es/client.go`:

```go
// Execute ES|QL query
func (c *Client) executeESQLQuery(ctx context.Context, query string) (*ESQLResult, error)

// Example: Get transaction stats with ES|QL
func (c *Client) GetTransactionNamesESQL(ctx, lookback, service, resource string) ([]TransactionNameAgg, error)
```

See `internal/es/client.go` lines 1751-1956 for implementation reference.

## Examples

For detailed examples, see [examples.md](examples.md)

## Resources

- [ES|QL Reference](https://www.elastic.co/docs/reference/query-languages/esql)
- [ES|QL Functions](https://www.elastic.co/docs/reference/query-languages/esql/functions-operators/aggregation-functions)
- [ES|QL Joins (9.2+)](https://www.elastic.co/blog/esql-lookup-join-elasticsearch)
