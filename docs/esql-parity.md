# ES|QL vs Query DSL parity checks

Use these quick checks to spot regressions after the TUI moved to ES|QL while
the CLI still exercises Query DSL.

## Logs / traces list
```bash
export ES_URL="${ES_URL:-http://localhost:9200}"
INDEX="logs-*"
LOOKBACK="now-1h"

# DSL via CLI (current behavior)
./bin/elasticat tail --size 200 --index "$INDEX" --lookback "$LOOKBACK" --json > /tmp/dsl.json

# ES|QL equivalent
curl -s -X POST "$ES_URL/_query" \
  -H 'Content-Type: application/json' \
  -d "{\"query\":\"FROM $INDEX | WHERE @timestamp >= NOW() - 1 hour | SORT @timestamp DESC | LIMIT 200 | KEEP *\"}" \
  | jq '.values' > /tmp/esql.json

echo "DSL count:  $(jq length /tmp/dsl.json)"
echo "ES|QL count: $(jq length /tmp/esql.json)"
```

## Span fetch by trace.id
```bash
TRACE_ID="<put-trace-id-here>"
INDEX="traces-*"

# DSL (CLI span command if available)
./bin/elasticat tail --size 500 --index "$INDEX" --trace-id "$TRACE_ID" --json > /tmp/dsl_spans.json

# ES|QL equivalent
curl -s -X POST "$ES_URL/_query" \
  -H 'Content-Type: application/json' \
  -d "{\"query\":\"FROM $INDEX | WHERE processor.event == \\\"span\\\" AND trace.id == \\\"$TRACE_ID\\\" | SORT @timestamp ASC | LIMIT 500 | KEEP *\"}" \
  | jq '.values' > /tmp/esql_spans.json

echo "DSL spans:  $(jq length /tmp/dsl_spans.json)"
echo "ES|QL spans: $(jq length /tmp/esql_spans.json)"
```

## Perspective counts
```bash
LOOKBACK="now-1h"

# DSL (previous behavior via CLI JSON flag)
./bin/elasticat perspective services --json --lookback "$LOOKBACK" > /tmp/dsl_persp.json

# ES|QL equivalent
curl -s -X POST "$ES_URL/_query" \
  -H 'Content-Type: application/json' \
  -d "{\"query\":\"FROM ${INDEX:-logs-*,traces-*,metrics-*} | WHERE @timestamp >= NOW() - 1 hour | STATS logs = COUNT(CASE(processor.event != \\\"transaction\\\" AND processor.event != \\\"span\\\", 1, null)), traces = COUNT(CASE(processor.event == \\\"transaction\\\", 1, null)), metrics = COUNT(CASE(metrics IS NOT NULL, 1, null)) BY service.name\"}" \
  | jq '.values' > /tmp/esql_persp.json

echo "DSL buckets:  $(jq length /tmp/dsl_persp.json)"
echo "ES|QL buckets: $(jq length /tmp/esql_persp.json)"
```

These checks do not require TUI interaction and let you spot cardinality or
filtering mismatches quickly. Adjust `INDEX`, `LOOKBACK`, and filters to mirror
the scenario you care about.

