DELETE FROM meal_claims WHERE status = 'NOT_CLAIMED';

ALTER TABLE meal_claims
    DROP CONSTRAINT IF EXISTS meal_claims_status_consistent;

ALTER TABLE meal_claims
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN claimed_at SET NOT NULL,
    ALTER COLUMN status TYPE TEXT USING status::TEXT;

DROP TYPE meal_claim_status;
CREATE TYPE meal_claim_status AS ENUM ('PENDING', 'VALIDATED');
ALTER TABLE meal_claims
    ALTER COLUMN status TYPE meal_claim_status USING status::meal_claim_status,
    ALTER COLUMN status SET DEFAULT 'PENDING';

ALTER TABLE meal_claims
    ADD CONSTRAINT meal_claims_validation_consistent CHECK (
        (status = 'PENDING' AND validated_at IS NULL AND validated_by IS NULL)
        OR (status = 'VALIDATED' AND validated_at IS NOT NULL AND validated_by IS NOT NULL)
    );
