// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package shared

// FilterBuilder provides a fluent interface for constructing
// Elasticsearch bool query clauses with must/must_not logic.
//
// DESIGN PRINCIPLE: Support Multiple Log Formats
// Filter clauses use "should" with minimum_should_match for fields that
// may appear in different locations depending on the log format (e.g., OTel
// semconv vs. ECS vs. custom formats).
type FilterBuilder struct {
	must    []map[string]interface{}
	mustNot []map[string]interface{}
}

// NewFilterBuilder creates a new FilterBuilder.
func NewFilterBuilder() *FilterBuilder {
	return &FilterBuilder{
		must:    []map[string]interface{}{},
		mustNot: []map[string]interface{}{},
	}
}

// AddMust adds a clause to the must array.
func (fb *FilterBuilder) AddMust(clause map[string]interface{}) *FilterBuilder {
	fb.must = append(fb.must, clause)
	return fb
}

// AddMustNot adds a clause to the must_not array.
func (fb *FilterBuilder) AddMustNot(clause map[string]interface{}) *FilterBuilder {
	fb.mustNot = append(fb.mustNot, clause)
	return fb
}

// AddClause adds a clause to must or must_not based on the negate flag.
func (fb *FilterBuilder) AddClause(clause map[string]interface{}, negate bool) *FilterBuilder {
	if negate {
		return fb.AddMustNot(clause)
	}
	return fb.AddMust(clause)
}

// AddServiceFilter adds a service filter clause that checks both OTel and flat formats.
// If negate is true, the service is excluded instead of included.
func (fb *FilterBuilder) AddServiceFilter(service string, negate bool) *FilterBuilder {
	if service == "" {
		return fb
	}
	clause := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []map[string]interface{}{
				{"term": map[string]interface{}{"resource.attributes.service.name": service}},
				{"term": map[string]interface{}{"resource.service.name": service}},
			},
			"minimum_should_match": 1,
		},
	}
	return fb.AddClause(clause, negate)
}

// AddResourceFilter adds a resource/environment filter clause.
// If negate is true, the resource is excluded instead of included.
func (fb *FilterBuilder) AddResourceFilter(resource string, negate bool) *FilterBuilder {
	if resource == "" {
		return fb
	}
	clause := map[string]interface{}{
		"term": map[string]interface{}{
			"resource.attributes.deployment.environment": resource,
		},
	}
	return fb.AddClause(clause, negate)
}

// AddLevelFilter adds a log level/severity filter that checks both OTel and standard formats.
func (fb *FilterBuilder) AddLevelFilter(level string) *FilterBuilder {
	if level == "" {
		return fb
	}
	return fb.AddMust(map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []map[string]interface{}{
				{"term": map[string]interface{}{"severity_text": level}},
				{"term": map[string]interface{}{"level": level}},
			},
			"minimum_should_match": 1,
		},
	})
}

// AddProcessorEventFilter adds a filter for processor.event (e.g., "transaction", "span").
func (fb *FilterBuilder) AddProcessorEventFilter(event string) *FilterBuilder {
	if event == "" {
		return fb
	}
	return fb.AddMust(map[string]interface{}{
		"term": map[string]interface{}{
			"attributes.processor.event": event,
		},
	})
}

// AddTransactionNameFilter adds a transaction name filter that checks both formats.
func (fb *FilterBuilder) AddTransactionNameFilter(name string) *FilterBuilder {
	if name == "" {
		return fb
	}
	return fb.AddMust(map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []map[string]interface{}{
				{"term": map[string]interface{}{"transaction.name": name}},
				{"term": map[string]interface{}{"name": name}},
			},
			"minimum_should_match": 1,
		},
	})
}

// AddTraceIDFilter adds a trace ID filter.
func (fb *FilterBuilder) AddTraceIDFilter(traceID string) *FilterBuilder {
	if traceID == "" {
		return fb
	}
	return fb.AddMust(map[string]interface{}{
		"term": map[string]interface{}{
			"trace_id": traceID,
		},
	})
}

// AddTimeRangeFilter adds a time range filter using ES time expressions.
// gte/lte can be ES time expressions like "now-1h" or RFC3339 timestamps.
func (fb *FilterBuilder) AddTimeRangeFilter(gte, lte string) *FilterBuilder {
	if gte == "" && lte == "" {
		return fb
	}
	timeRange := map[string]interface{}{}
	if gte != "" {
		timeRange["gte"] = gte
	}
	if lte != "" {
		timeRange["lte"] = lte
	}
	return fb.AddMust(map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": timeRange,
		},
	})
}

// AddExistsFilter adds a filter for documents where a field exists.
func (fb *FilterBuilder) AddExistsFilter(field string) *FilterBuilder {
	if field == "" {
		return fb
	}
	return fb.AddMust(map[string]interface{}{
		"exists": map[string]interface{}{
			"field": field,
		},
	})
}

// AddPrefixFilter adds a prefix filter for a field.
func (fb *FilterBuilder) AddPrefixFilter(field, prefix string) *FilterBuilder {
	if field == "" || prefix == "" {
		return fb
	}
	return fb.AddMust(map[string]interface{}{
		"prefix": map[string]interface{}{
			field: prefix,
		},
	})
}

// AddQueryString adds a full-text query_string clause.
func (fb *FilterBuilder) AddQueryString(query string, fields []string) *FilterBuilder {
	if query == "" {
		return fb
	}
	if len(fields) == 0 {
		fields = []string{"body.text", "body", "message", "event_name"}
	}
	// Wrap query in wildcards for partial matching on keyword fields
	wildcardQuery := "*" + query + "*"
	return fb.AddMust(map[string]interface{}{
		"query_string": map[string]interface{}{
			"query":            wildcardQuery,
			"fields":           fields,
			"default_operator": "AND",
			"analyze_wildcard": true,
		},
	})
}

// Build returns the completed bool query.
func (fb *FilterBuilder) Build() map[string]interface{} {
	boolQuery := map[string]interface{}{
		"must": fb.must,
	}
	if len(fb.mustNot) > 0 {
		boolQuery["must_not"] = fb.mustNot
	}
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
	}
}

// Must returns the must clauses (for inspection/testing).
func (fb *FilterBuilder) Must() []map[string]interface{} {
	return fb.must
}

// MustNot returns the must_not clauses (for inspection/testing).
func (fb *FilterBuilder) MustNot() []map[string]interface{} {
	return fb.mustNot
}
