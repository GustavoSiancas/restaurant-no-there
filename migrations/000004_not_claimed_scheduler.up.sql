ALTER TABLE meal_claims
    DROP CONSTRAINT meal_claims_validation_consistent;

ALTER TABLE meal_claims
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN status TYPE TEXT USING status::TEXT;

DROP TYPE meal_claim_status;
CREATE TYPE meal_claim_status AS ENUM ('PENDING', 'VALIDATED', 'NOT_CLAIMED');

ALTER TABLE meal_claims
    ALTER COLUMN status TYPE meal_claim_status USING status::meal_claim_status,
    ALTER COLUMN status SET DEFAULT 'PENDING',
    ALTER COLUMN claimed_at DROP NOT NULL;

ALTER TABLE meal_claims
    ADD CONSTRAINT meal_claims_status_consistent CHECK (
        (status = 'PENDING' AND claimed_at IS NOT NULL AND validated_at IS NULL AND validated_by IS NULL)
        OR (status = 'VALIDATED' AND claimed_at IS NOT NULL AND validated_at IS NOT NULL AND validated_by IS NOT NULL)
        OR (status = 'NOT_CLAIMED' AND claimed_at IS NULL AND validated_at IS NULL AND validated_by IS NULL)
    );
