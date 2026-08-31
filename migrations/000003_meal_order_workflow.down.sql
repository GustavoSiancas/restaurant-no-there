DROP INDEX IF EXISTS meal_claims_status_created_idx;

ALTER TABLE meal_claims
    DROP CONSTRAINT IF EXISTS meal_claims_validation_consistent,
    DROP COLUMN IF EXISTS validated_by,
    DROP COLUMN IF EXISTS validated_at,
    DROP COLUMN IF EXISTS status;

DROP TYPE IF EXISTS meal_claim_status;

DELETE FROM users WHERE role = 'COLLABORATOR';
ALTER TABLE users ALTER COLUMN role TYPE TEXT;
DROP TYPE user_role;
CREATE TYPE user_role AS ENUM ('ADMIN', 'OWNER', 'RRHH', 'WORKER');
ALTER TABLE users ALTER COLUMN role TYPE user_role USING role::user_role;
