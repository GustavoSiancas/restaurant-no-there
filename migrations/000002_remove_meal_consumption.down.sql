DROP INDEX IF EXISTS meal_claims_report_idx;

ALTER TABLE meal_claims
    ADD COLUMN consumed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN consumed_at TIMESTAMPTZ,
    ADD COLUMN consumption_registered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT meal_claims_consumption_consistent CHECK (
        (consumed = FALSE AND consumed_at IS NULL AND consumption_registered_by IS NULL)
        OR (consumed = TRUE AND consumed_at IS NOT NULL AND consumption_registered_by IS NOT NULL)
    );

CREATE INDEX meal_claims_report_idx
    ON meal_claims (service_date, meal_type, consumed);
