DROP TABLE IF EXISTS model_usage_cost;
DROP TRIGGER IF EXISTS trg_model_pricing_no_overlap ON model_pricing;
DROP FUNCTION IF EXISTS reject_overlapping_model_pricing();
DROP TABLE IF EXISTS model_pricing;
ALTER TABLE model_usage DROP COLUMN IF EXISTS resolved_model_name;

