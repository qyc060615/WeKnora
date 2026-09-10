CREATE TABLE IF NOT EXISTS model_usage (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL CHECK (tenant_id > 0),
    model_tenant_id INTEGER NOT NULL CHECK (model_tenant_id > 0),
    evaluation_run_id VARCHAR(36),
    model_id VARCHAR(64) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    model_type VARCHAR(32) NOT NULL,
    model_source VARCHAR(32) NOT NULL,
    resolved_provider VARCHAR(64) NOT NULL,
    call_type VARCHAR(16) NOT NULL,
    purpose VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL,
    token_provenance VARCHAR(32) NOT NULL,
    latency_ms INTEGER CHECK (latency_ms IS NULL OR latency_ms >= 0),
    started_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    input_tokens INTEGER,
    output_tokens INTEGER,
    total_tokens INTEGER,
    prompt_cache_status VARCHAR(32),
    cache_read_tokens INTEGER,
    cache_write_tokens INTEGER,
    cache_miss_tokens INTEGER,
    logical_requests INTEGER NOT NULL DEFAULT 0,
    embedding_inputs INTEGER NOT NULL DEFAULT 0,
    cache_hits INTEGER NOT NULL DEFAULT 0,
    cache_misses INTEGER NOT NULL DEFAULT 0,
    provider_requests INTEGER NOT NULL DEFAULT 0,
    provider_inputs INTEGER NOT NULL DEFAULT 0,
    cache_read_errors INTEGER NOT NULL DEFAULT 0,
    cache_write_errors INTEGER NOT NULL DEFAULT 0,
    embedding_cache_status VARCHAR(32),
    queries INTEGER NOT NULL DEFAULT 0,
    documents INTEGER NOT NULL DEFAULT 0,
    pairs INTEGER NOT NULL DEFAULT 0,
    provider_pairs INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (evaluation_run_id) REFERENCES evaluation_runs (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_model_usage_tenant_created
    ON model_usage (tenant_id, created_at);

CREATE INDEX IF NOT EXISTS idx_model_usage_tenant_model_created
    ON model_usage (tenant_id, model_id, created_at);

CREATE INDEX IF NOT EXISTS idx_model_usage_tenant_evaluation_created
    ON model_usage (tenant_id, evaluation_run_id, created_at);

CREATE INDEX IF NOT EXISTS idx_model_usage_evaluation_run
    ON model_usage (evaluation_run_id);
