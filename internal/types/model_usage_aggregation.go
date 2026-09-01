package types

// CallCounts counts persisted logical invocations. Total is always the number
// of model_usage rows, never the sum of logical_requests.
type CallCounts struct {
	Total     int64 `json:"total"`
	Chat      int64 `json:"chat"`
	Embedding int64 `json:"embedding"`
	Rerank    int64 `json:"rerank"`
}

// NullableMetricAggregate preserves the difference between no observation and
// an explicitly observed zero. ApplicableCalls excludes call types for which a
// metric has no meaning.
type NullableMetricAggregate struct {
	Sum             *int64 `json:"sum"`
	ObservedCalls   int64  `json:"observed_calls"`
	ApplicableCalls int64  `json:"applicable_calls"`
}

type NonNullUsageCounters struct {
	LogicalRequests  int64 `json:"logical_requests"`
	ProviderRequests int64 `json:"provider_requests"`
	ProviderInputs   int64 `json:"provider_inputs"`
	ProviderPairs    int64 `json:"provider_pairs"`
	EmbeddingInputs  int64 `json:"embedding_inputs"`
	CacheHits        int64 `json:"cache_hits"`
	CacheMisses      int64 `json:"cache_misses"`
	CacheReadErrors  int64 `json:"cache_read_errors"`
	CacheWriteErrors int64 `json:"cache_write_errors"`
	Queries          int64 `json:"queries"`
	Documents        int64 `json:"documents"`
	Pairs            int64 `json:"pairs"`
}

type PromptCacheStatusCounts struct {
	NotRecorded int64 `json:"not_recorded"`
	Unreported  int64 `json:"unreported"`
	Unsupported int64 `json:"unsupported"`
	Miss        int64 `json:"miss"`
	Hit         int64 `json:"hit"`
}

type EmbeddingCacheStatusCounts struct {
	NotRecorded int64 `json:"not_recorded"`
	Disabled    int64 `json:"disabled"`
	FullHit     int64 `json:"full_hit"`
	Partial     int64 `json:"partial"`
	Miss        int64 `json:"miss"`
}

type LatencyAggregate struct {
	SumMS           *int64   `json:"sum_ms"`
	AverageMS       *float64 `json:"avg_ms"`
	MaxMS           *int64   `json:"max_ms"`
	ObservedCalls   int64    `json:"observed_calls"`
	ApplicableCalls int64    `json:"applicable_calls"`
}

// CallTypeCostAggregate keeps fully-priced totals separate from all known
// components. KnownCost can therefore include the determined portion of a
// partial cost row without presenting it as fully priced.
type CallTypeCostAggregate struct {
	PricedCost    Decimal `json:"priced_cost"`
	KnownCost     Decimal `json:"known_cost"`
	PricedCalls   int64   `json:"priced_calls"`
	PartialCalls  int64   `json:"partial_calls"`
	UnpricedCalls int64   `json:"unpriced_calls"`
}

type CurrencyCostAggregate struct {
	Currency  string                `json:"currency"`
	Chat      CallTypeCostAggregate `json:"chat"`
	Embedding CallTypeCostAggregate `json:"embedding"`
	Rerank    CallTypeCostAggregate `json:"rerank"`
	Total     CallTypeCostAggregate `json:"total"`
}

// EvaluationModelUsageAggregate is dynamically derived from model_usage and
// its optional one-to-one model_usage_cost extension for one tenant/run scope.
type EvaluationModelUsageAggregate struct {
	Calls CallCounts `json:"calls"`

	InputTokens  NullableMetricAggregate `json:"input_tokens"`
	OutputTokens NullableMetricAggregate `json:"output_tokens"`
	TotalTokens  NullableMetricAggregate `json:"total_tokens"`

	Counters         NonNullUsageCounters       `json:"counters"`
	CacheReadTokens  NullableMetricAggregate    `json:"cache_read_tokens"`
	CacheWriteTokens NullableMetricAggregate    `json:"cache_write_tokens"`
	CacheMissTokens  NullableMetricAggregate    `json:"cache_miss_tokens"`
	PromptCache      PromptCacheStatusCounts    `json:"prompt_cache_statuses"`
	EmbeddingCache   EmbeddingCacheStatusCounts `json:"embedding_cache_statuses"`
	Latency          LatencyAggregate           `json:"latency"`

	CostByCurrency          []CurrencyCostAggregate `json:"cost_by_currency"`
	NoCostRowCalls          CallCounts              `json:"no_cost_row_calls"`
	CostRowsWithoutCurrency CallCounts              `json:"cost_rows_without_currency"`
}
