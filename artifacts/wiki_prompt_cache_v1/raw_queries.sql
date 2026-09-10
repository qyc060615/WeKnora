-- =============================================================================
-- Wiki Prompt Cache Controlled Experiment — model_usage extraction queries
-- Repository: qyc060615/WeKnora
-- Branch:     feat/topic3-evaluation
-- BEFORE:     3f9a054ec94e92c3089a6574281aa09760068e38
-- AFTER:      22f9120216d554181db041b24e3e191363c34366
--
-- Scope of one trial:
--   tenant_id     = 10000
--   call_type     = 'chat'
--   purpose       LIKE 'wiki_%'      (excludes embedding / rerank / normal chat)
--   created_at    >  :start_ts  AND  created_at <= :end_ts
--
-- :start_ts / :end_ts are the trial's UTC boundaries, captured as
--   start_ts = MAX(model_usage.created_at) immediately before the trial, and
--   end_ts   = MAX(model_usage.created_at) immediately after the trial.
--
-- Each model_usage row == exactly one logical LLM invocation
--   (LogicalRequests is always 1; ProviderRequests counts outbound HTTP
--   attempts including retries). See internal/models/chat/usage_wrapper.go.
--
-- prompt_cache_status semantics (DeepSeek automatic prefix caching):
--   'hit'          cache_read_tokens > 0            (prefix was reused)
--   'miss'         cache reported, read == 0        (prefix was NOT reused)
--   'unreported'   provider returned no cache counters
--   'unsupported'  provider does not expose cache accounting
-- See internal/models/chat/prompt_cache.go + internal/types/chat.go.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 0) Trial boundary helper — run once per trial to capture the boundaries.
--    Record the emitted values into before_trials.json / after_trials.json.
-- -----------------------------------------------------------------------------
-- SELECT MAX(created_at) AS start_ts_before_trial FROM model_usage;
-- ... run the wiki ingest ...
-- SELECT MAX(created_at) AS end_ts_after_trial FROM model_usage;

-- -----------------------------------------------------------------------------
-- 1) Layered usage metrics (ALL wiki_* + per-purpose)
--    Columns map 1:1 to the experiment's usage-dimension definitions.
-- -----------------------------------------------------------------------------
SELECT
    COALESCE(purpose, 'ALL wiki_*')                     AS layer,
    COUNT(*)                                            AS logical_calls,
    SUM(logical_requests)                               AS logical_requests,
    SUM(provider_requests)                              AS provider_requests,
    SUM(input_tokens)                                   AS input_tokens,
    SUM(output_tokens)                                  AS output_tokens,
    SUM(cache_read_tokens)                              AS cache_read_tokens,
    SUM(cache_write_tokens)                             AS cache_write_tokens,
    SUM(cache_miss_tokens)                              AS cache_miss_tokens,
    COUNT(*) FILTER (WHERE prompt_cache_status = 'hit')   AS hit_calls,
    COUNT(*) FILTER (WHERE prompt_cache_status = 'miss')  AS miss_calls,
    COUNT(*) FILTER (WHERE prompt_cache_status IN ('hit','miss'))
                                                        AS cache_accounted_calls,
    COUNT(*) FILTER (WHERE prompt_cache_status = 'unreported')
                                                        AS unreported_calls,
    COUNT(*) FILTER (WHERE prompt_cache_status = 'unsupported')
                                                        AS unsupported_calls,
    ROUND(AVG(latency_ms), 2)                           AS avg_latency_ms,
    ROUND(percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms)::numeric, 2)
                                                        AS p50_latency_ms,
    ROUND(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)::numeric, 2)
                                                        AS p95_latency_ms,
    MAX(latency_ms)                                     AS max_latency_ms,
    -- token_cache_ratio = cache_read / (cache_read + cache_miss)
    CASE WHEN (SUM(cache_read_tokens) + SUM(cache_miss_tokens)) > 0 THEN
        ROUND(SUM(cache_read_tokens)::numeric /
              (SUM(cache_read_tokens) + SUM(cache_miss_tokens)), 6)
    ELSE NULL END                                       AS token_cache_ratio,
    -- call_hit_rate = hit_calls / cache_accounted_calls
    CASE WHEN COUNT(*) FILTER (WHERE prompt_cache_status IN ('hit','miss')) > 0 THEN
        ROUND(COUNT(*) FILTER (WHERE prompt_cache_status = 'hit')::numeric /
              COUNT(*) FILTER (WHERE prompt_cache_status IN ('hit','miss')), 6)
    ELSE NULL END                                       AS call_hit_rate
FROM model_usage
WHERE tenant_id  = 10000
  AND call_type  = 'chat'
  AND purpose   LIKE 'wiki_%'
  AND created_at >  :start_ts
  AND created_at <= :end_ts
GROUP BY GROUPING SETS ((purpose), ())
ORDER BY layer;

-- -----------------------------------------------------------------------------
-- 2) Cost metrics (LEFT JOIN model_usage_cost), grouped by purpose + currency.
--    NOTE: model_pricing is empty in this environment, so expect
--    status = 'unpriced' and NULL costs. This is reported as a limitation.
-- -----------------------------------------------------------------------------
SELECT
    COALESCE(mu.purpose, 'ALL wiki_*') AS layer,
    muc.currency                        AS currency,
    muc.status                          AS cost_status,
    COUNT(*)                            AS rows,
    SUM(muc.total_cost)                 AS total_cost,
    SUM(muc.input_cost)                 AS input_cost,
    SUM(muc.output_cost)                AS output_cost,
    SUM(muc.cache_read_cost)            AS cache_read_cost,
    SUM(muc.cache_write_cost)           AS cache_write_cost
FROM model_usage mu
LEFT JOIN model_usage_cost muc ON muc.usage_id = mu.id
WHERE mu.tenant_id  = 10000
  AND mu.call_type  = 'chat'
  AND mu.purpose   LIKE 'wiki_%'
  AND mu.created_at >  :start_ts
  AND mu.created_at <= :end_ts
GROUP BY GROUPING SETS ((mu.purpose, muc.currency, muc.status),
                        (muc.currency, muc.status))
ORDER BY layer, currency, cost_status;

-- -----------------------------------------------------------------------------
-- 3) Validity check — verify no abnormal call shrinkage / errors / timeouts.
--    A prompt-cache change must NOT eliminate logical calls. If AFTER has
--    materially fewer logical_calls than BEFORE, that is a red flag, not a win.
-- -----------------------------------------------------------------------------
SELECT
    status,
    COUNT(*)                        AS calls,
    SUM(provider_requests)          AS provider_requests,
    SUM(logical_requests)           AS logical_requests
FROM model_usage
WHERE tenant_id  = 10000
  AND call_type  = 'chat'
  AND purpose   LIKE 'wiki_%'
  AND created_at >  :start_ts
  AND created_at <= :end_ts
GROUP BY status
ORDER BY status;

-- -----------------------------------------------------------------------------
-- 4) Per-purpose status/cache breakdown (sanity + unreported accounting).
-- -----------------------------------------------------------------------------
SELECT
    purpose,
    prompt_cache_status,
    COUNT(*)                        AS calls,
    SUM(cache_read_tokens)          AS cache_read_tokens,
    SUM(cache_miss_tokens)          AS cache_miss_tokens,
    ROUND(AVG(latency_ms), 2)       AS avg_latency_ms
FROM model_usage
WHERE tenant_id  = 10000
  AND call_type  = 'chat'
  AND purpose   LIKE 'wiki_%'
  AND created_at >  :start_ts
  AND created_at <= :end_ts
GROUP BY purpose, prompt_cache_status
ORDER BY purpose, prompt_cache_status;

-- -----------------------------------------------------------------------------
-- 5) Whole-trial wall clock (from the earliest started_at to the latest
--    created_at of the trial's wiki rows). This is the DB-observed task span;
--    the orchestrator also records wall clock from its own timer.
-- -----------------------------------------------------------------------------
SELECT
    MIN(started_at)                                   AS first_call_started_at,
    MAX(created_at)                                   AS last_call_created_at,
    EXTRACT(EPOCH FROM (MAX(created_at) - MIN(started_at))) * 1000
                                                      AS wall_clock_ms
FROM model_usage
WHERE tenant_id  = 10000
  AND call_type  = 'chat'
  AND purpose   LIKE 'wiki_%'
  AND created_at >  :start_ts
  AND created_at <= :end_ts;
