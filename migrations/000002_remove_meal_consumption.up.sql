ALTER TABLE meal_claims
    DROP CONSTRAINT IF EXISTS meal_claims_consumption_consistent;

DROP INDEX IF EXISTS meal_claims_report_idx;

ALTER TABLE meal_claims
    DROP COLUMN IF EXISTS consumed,
    DROP COLUMN IF EXISTS consumed_at,
    DROP COLUMN IF EXISTS consumption_registered_by;

CREATE INDEX meal_claims_report_idx
    ON meal_claims (service_date, meal_type);
