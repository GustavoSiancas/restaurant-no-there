ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'COLLABORATOR';

CREATE TYPE meal_claim_status AS ENUM ('PENDING', 'VALIDATED');

ALTER TABLE meal_claims
    ADD COLUMN status meal_claim_status NOT NULL DEFAULT 'PENDING',
    ADD COLUMN validated_at TIMESTAMPTZ,
    ADD COLUMN validated_by UUID REFERENCES users(id) ON DELETE RESTRICT,
    ADD CONSTRAINT meal_claims_validation_consistent CHECK (
        (status = 'PENDING' AND validated_at IS NULL AND validated_by IS NULL)
        OR (status = 'VALIDATED' AND validated_at IS NOT NULL AND validated_by IS NOT NULL)
    );

CREATE INDEX meal_claims_status_created_idx
    ON meal_claims (status, created_at DESC);
