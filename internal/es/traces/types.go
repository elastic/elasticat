package traces

// TransactionNameAgg represents aggregated statistics for a transaction name
type TransactionNameAgg struct {
	Name        string  // Transaction name (e.g., "GET /api/users")
	Count       int64   // Number of transactions
	AvgDuration float64 // Average duration in milliseconds
	MinDuration float64 // Minimum duration in milliseconds
	MaxDuration float64 // Maximum duration in milliseconds
	TraceCount  int64   // Number of unique traces
	AvgSpans    float64 // Average number of spans per trace
	ErrorRate   float64 // Percentage of errors (0-100)
}

// ESQLResult represents the response from an ES|QL query
type ESQLResult struct {
	Columns   []ESQLColumn    `json:"columns"`
	Values    [][]interface{} `json:"values"`
	Took      int             `json:"took"`
	IsPartial bool            `json:"is_partial"`
}

// ESQLColumn describes a column in an ES|QL result
type ESQLColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
