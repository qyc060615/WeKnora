export type ModelUsageAnalyticsInterval = 'hour' | 'day'

export interface CallCounts {
  total: number
  chat: number
  embedding: number
  rerank: number
}

export interface NullableMetricAggregate {
  sum: number | null
  observed_calls: number
  applicable_calls: number
}

export interface LatencyAggregate {
  sum_ms: number | null
  avg_ms: number | null
  max_ms: number | null
  observed_calls: number
  applicable_calls: number
}

export interface PromptCacheAnalytics {
  hit_calls: number
  miss_calls: number
  unsupported_calls: number
  unreported_calls: number
  not_recorded_calls: number
  eligible_calls: number
  call_hit_rate: number | null
  cache_read_tokens: number
  cache_write_tokens: number
  cache_miss_tokens: number
  token_denominator: number
  token_cache_ratio: number | null
}

export interface EmbeddingCacheAnalytics {
  full_hit_calls: number
  partial_calls: number
  miss_calls: number
  disabled_calls: number
  not_recorded_calls: number
  cache_hits: number
  cache_misses: number
  eligible_inputs: number
  input_hit_rate: number | null
}

export interface ModelUsageAnalyticsAggregate {
  calls: CallCounts
  input_tokens: NullableMetricAggregate
  output_tokens: NullableMetricAggregate
  total_tokens: NullableMetricAggregate
  latency: LatencyAggregate
  prompt_cache: PromptCacheAnalytics
  embedding_cache: EmbeddingCacheAnalytics
}

export interface ModelUsageAnalyticsBucket extends ModelUsageAnalyticsAggregate {
  bucket_start: string
}

export interface ModelUsageAnalyticsResult {
  time_basis: 'created_at'
  interval: ModelUsageAnalyticsInterval
  start_time: string
  end_time: string
  model_id?: string
  summary: ModelUsageAnalyticsAggregate
  trend: ModelUsageAnalyticsBucket[]
}

export interface ModelUsageAnalyticsParams {
  modelId?: string
  startTime?: string
  endTime?: string
  interval: ModelUsageAnalyticsInterval
}

export interface ModelUsageAnalyticsQuery {
  model_id?: string
  start_time?: string
  end_time?: string
  interval: ModelUsageAnalyticsInterval
}

export function toModelUsageAnalyticsQuery(params: ModelUsageAnalyticsParams): ModelUsageAnalyticsQuery {
  const query: ModelUsageAnalyticsQuery = { interval: params.interval }
  if (params.modelId) query.model_id = params.modelId
  if (params.startTime) query.start_time = params.startTime
  if (params.endTime) query.end_time = params.endTime
  return query
}
