package perspectives

// PerspectiveAgg represents aggregate counts for a service or resource
type PerspectiveAgg struct {
	Name        string
	LogCount    int64
	TraceCount  int64
	MetricCount int64
}
