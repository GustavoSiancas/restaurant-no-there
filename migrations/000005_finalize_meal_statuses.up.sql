ALTER TABLE meal_claims
    DROP CONSTRAINT meal_claims_status_consistent;

ALTER TABLE meal_claims
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN status TYPE TEXT USING status::TEXT;

UPDATE meal_claims SET status = 'REQUESTED' WHERE status = 'PENDING';
UPDATE meal_claims SET status = 'NOT_CONSUMED' WHERE status = 'NOT_CLAIMED';

DROP TYPE meal_claim_status;
CREATE TYPE meal_claim_status AS ENUM (
    'REQUESTED',
    'VALIDATED',
    'NOT_CONSUMED',
    'REQUESTED_BUT_NOT_VALIDATED'
);

ALTER TABLE meal_claims
    ALTER COLUMN status TYPE meal_claim_status USING status::meal_claim_status,
    ALTER COLUMN status SET DEFAULT 'REQUESTED';

ALTER TABLE meal_claims
    ADD CONSTRAINT meal_claims_status_consistent CHECK (
        (status = 'REQUESTED' AND claimed_at IS NOT NULL AND validated_at IS NULL AND validated_by IS NULL)
        OR (status = 'VALIDATED' AND claimed_at IS NOT NULL AND validated_at IS NOT NULL AND validated_by IS NOT NULL)
        OR (status = 'NOT_CONSUMED' AND claimed_at IS NULL AND validated_at IS NULL AND validated_by IS NULL)
        OR (status = 'REQUESTED_BUT_NOT_VALIDATED' AND claimed_at IS NOT NULL AND validated_at IS NULL AND validated_by IS NULL)
    );
