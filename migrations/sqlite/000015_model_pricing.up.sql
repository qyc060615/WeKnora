ALTER TABLE model_usage ADD COLUMN resolved_model_name VARCHAR(255);

CREATE TABLE IF NOT EXISTS model_pricing (
    id VARCHAR(36) PRIMARY KEY,
    resolved_provider VARCHAR(64) NOT NULL,
    resolved_model_name VARCHAR(255) NOT NULL,
    call_type VARCHAR(16) NOT NULL,
    currency VARCHAR(16) NOT NULL,
    billing_mode VARCHAR(64) NOT NULL,
    input_token_price TEXT,
    output_token_price TEXT,
    total_token_price TEXT,
    cache_read_token_price TEXT,
    cache_write_token_price TEXT,
    per_request_price TEXT,
    per_input_price TEXT,
    per_pair_price TEXT,
    unit_scale TEXT NOT NULL CHECK (CAST(unit_scale AS NUMERIC) > 0),
    effective_from DATETIME NOT NULL,
    effective_to DATETIME,
    pricing_version VARCHAR(128) NOT NULL,
    source_name VARCHAR(128) NOT NULL,
    source_reference TEXT,
    source_retrieved_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE INDEX IF NOT EXISTS idx_model_pricing_identity_time
    ON model_pricing (resolved_provider, resolved_model_name, call_type, effective_from);

CREATE TRIGGER IF NOT EXISTS trg_model_pricing_no_overlap_insert
BEFORE INSERT ON model_pricing
WHEN EXISTS (
    SELECT 1 FROM model_pricing existing
    WHERE existing.resolved_provider = NEW.resolved_provider
      AND existing.resolved_model_name = NEW.resolved_model_name
      AND existing.call_type = NEW.call_type
      AND existing.effective_from < COALESCE(NEW.effective_to, '9999-12-31T23:59:59Z')
      AND NEW.effective_from < COALESCE(existing.effective_to, '9999-12-31T23:59:59Z')
)
BEGIN
    SELECT RAISE(ABORT, 'overlapping model_pricing effective interval');
END;

CREATE TRIGGER IF NOT EXISTS trg_model_pricing_no_overlap_update
BEFORE UPDATE ON model_pricing
WHEN EXISTS (
    SELECT 1 FROM model_pricing existing
    WHERE existing.resolved_provider = NEW.resolved_provider
      AND existing.resolved_model_name = NEW.resolved_model_name
      AND existing.call_type = NEW.call_type
      AND existing.id <> NEW.id
      AND existing.effective_from < COALESCE(NEW.effective_to, '9999-12-31T23:59:59Z')
      AND NEW.effective_from < COALESCE(existing.effective_to, '9999-12-31T23:59:59Z')
)
BEGIN
    SELECT RAISE(ABORT, 'overlapping model_pricing effective interval');
END;

CREATE TABLE IF NOT EXISTS model_usage_cost (
    id VARCHAR(36) PRIMARY KEY,
    usage_id VARCHAR(36) NOT NULL UNIQUE REFERENCES model_usage(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL CHECK (status IN ('priced', 'unpriced', 'partial')),
    currency VARCHAR(16),
    total_cost TEXT,
    known_cost TEXT,
    input_cost TEXT,
    output_cost TEXT,
    cache_read_cost TEXT,
    cache_write_cost TEXT,
    request_cost TEXT,
    provider_input_cost TEXT,
    provider_pair_cost TEXT,
    pricing_rule_id VARCHAR(36) REFERENCES model_pricing(id) ON DELETE SET NULL,
    pricing_version VARCHAR(128),
    pricing_snapshot TEXT NOT NULL DEFAULT '{}',
    calculator_version VARCHAR(32) NOT NULL,
    calculated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (status <> 'priced' OR total_cost IS NOT NULL),
    CHECK (status = 'priced' OR total_cost IS NULL)
);
