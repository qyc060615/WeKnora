DROP TABLE IF EXISTS model_usage_cost;
DROP TRIGGER IF EXISTS trg_model_pricing_no_overlap_update;
DROP TRIGGER IF EXISTS trg_model_pricing_no_overlap_insert;
DROP TABLE IF EXISTS model_pricing;
ALTER TABLE model_usage DROP COLUMN resolved_model_name;
