ALTER TABLE model_usage
    ADD COLUMN IF NOT EXISTS resolved_model_name VARCHAR(255);

CREATE TABLE IF NOT EXISTS model_pricing (
    id VARCHAR(36) PRIMARY KEY,
    resolved_provider VARCHAR(64) NOT NULL,
    resolved_model_name VARCHAR(255) NOT NULL,
    call_type VARCHAR(16) NOT NULL,
    currency VARCHAR(16) NOT NULL,
    billing_mode VARCHAR(64) NOT NULL,
    input_token_price NUMERIC(38,18),
    output_token_price NUMERIC(38,18),
    total_token_price NUMERIC(38,18),
    cache_read_token_price NUMERIC(38,18),
    cache_write_token_price NUMERIC(38,18),
    per_request_price NUMERIC(38,18),
    per_input_price NUMERIC(38,18),
    per_pair_price NUMERIC(38,18),
    unit_scale NUMERIC(38,9) NOT NULL CHECK (unit_scale > 0),
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ,
    pricing_version VARCHAR(128) NOT NULL,
    source_name VARCHAR(128) NOT NULL,
    source_reference TEXT,
    source_retrieved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE INDEX IF NOT EXISTS idx_model_pricing_identity_time
    ON model_pricing (resolved_provider, resolved_model_name, call_type, effective_from);

CREATE OR REPLACE FUNCTION reject_overlapping_model_pricing()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(
        to_jsonb(ARRAY[NEW.resolved_provider, NEW.resolved_model_name, NEW.call_type])::text,
        0
    ));
    IF EXISTS (
        SELECT 1 FROM model_pricing existing
        WHERE existing.resolved_provider = NEW.resolved_provider
          AND existing.resolved_model_name = NEW.resolved_model_name
          AND existing.call_type = NEW.call_type
          AND existing.id <> NEW.id
          AND existing.effective_from < COALESCE(NEW.effective_to, 'infinity'::timestamptz)
          AND NEW.effective_from < COALESCE(existing.effective_to, 'infinity'::timestamptz)
    ) THEN
        RAISE EXCEPTION 'overlapping model_pricing effective interval';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_model_pricing_no_overlap ON model_pricing;
CREATE TRIGGER trg_model_pricing_no_overlap
BEFORE INSERT OR UPDATE ON model_pricing
FOR EACH ROW EXECUTE FUNCTION reject_overlapping_model_pricing();

CREATE TABLE IF NOT EXISTS model_usage_cost (
    id VARCHAR(36) PRIMARY KEY,
    usage_id VARCHAR(36) NOT NULL UNIQUE REFERENCES model_usage(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL CHECK (status IN ('priced', 'unpriced', 'partial')),
    currency VARCHAR(16),
    total_cost NUMERIC(38,18),
    known_cost NUMERIC(38,18),
    input_cost NUMERIC(38,18),
    output_cost NUMERIC(38,18),
    cache_read_cost NUMERIC(38,18),
    cache_write_cost NUMERIC(38,18),
    request_cost NUMERIC(38,18),
    provider_input_cost NUMERIC(38,18),
    provider_pair_cost NUMERIC(38,18),
    pricing_rule_id VARCHAR(36) REFERENCES model_pricing(id) ON DELETE SET NULL,
    pricing_version VARCHAR(128),
    pricing_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    calculator_version VARCHAR(32) NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (status <> 'priced' OR total_cost IS NOT NULL),
    CHECK (status = 'priced' OR total_cost IS NULL)
);
