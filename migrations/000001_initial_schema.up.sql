CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('ADMIN', 'OWNER', 'RRHH', 'WORKER', 'COLLABORATOR');
CREATE TYPE credential_type AS ENUM ('PASSWORD', 'DNI', 'FACE_SCAN');
CREATE TYPE shift_type AS ENUM ('DIA', 'NOCHE');
CREATE TYPE meal_type AS ENUM ('DESAYUNO', 'TARDE', 'NOCHE');
CREATE TYPE meal_claim_status AS ENUM ('REQUESTED', 'VALIDATED', 'NOT_CONSUMED', 'REQUESTED_BUT_NOT_VALIDATED');

-- Cuenta del sistema. No contiene información personal ni credenciales.
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role user_role NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX users_role_idx ON users (role);
CREATE UNIQUE INDEX users_single_admin_idx ON users (role) WHERE role = 'ADMIN';

-- Datos personales separados de la cuenta.
CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(320),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_profiles_first_name_not_blank CHECK (BTRIM(first_name) <> ''),
    CONSTRAINT user_profiles_last_name_not_blank CHECK (BTRIM(last_name) <> '')
);

CREATE UNIQUE INDEX user_profiles_email_unique_idx
    ON user_profiles (LOWER(email)) WHERE email IS NOT NULL;

-- Métodos de autenticación. FACE_SCAN queda listo para asociar en el futuro
-- un identificador externo, sin almacenar imágenes o plantillas biométricas.
CREATE TABLE user_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type credential_type NOT NULL,
    identifier VARCHAR(255) NOT NULL,
    secret_hash TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_credentials_identifier_not_blank CHECK (BTRIM(identifier) <> ''),
    CONSTRAINT user_credentials_secret_by_type CHECK (
        (type = 'PASSWORD' AND secret_hash IS NOT NULL)
        OR (type IN ('DNI', 'FACE_SCAN') AND secret_hash IS NULL)
    ),
    UNIQUE (type, identifier)
);

CREATE INDEX user_credentials_user_id_idx ON user_credentials (user_id);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_active_idx ON refresh_tokens (expires_at) WHERE revoked_at IS NULL;

-- Información laboral exclusiva de usuarios WORKER.
CREATE TABLE worker_information (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    employee_code VARCHAR(50) NOT NULL UNIQUE,
    job_title VARCHAR(120),
    department VARCHAR(120),
    phone VARCHAR(30),
    address TEXT,
    hire_date DATE,
    emergency_contact_name VARCHAR(200),
    emergency_contact_phone VARCHAR(30),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT worker_information_employee_code_not_blank CHECK (BTRIM(employee_code) <> '')
);

-- Calendario rotativo. Los únicos turnos posibles son DIA y NOCHE.
CREATE TABLE worker_shift_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    worker_id UUID NOT NULL REFERENCES worker_information(user_id) ON DELETE CASCADE,
    shift_type shift_type NOT NULL,
    work_date DATE NOT NULL,
    assigned_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT worker_shift_assignments_one_shift_per_day UNIQUE (worker_id, work_date)
);

CREATE INDEX worker_shift_assignments_date_idx ON worker_shift_assignments (work_date);
CREATE INDEX worker_shift_assignments_type_date_idx ON worker_shift_assignments (shift_type, work_date);

-- Horarios configurables, siempre evaluados en hora de Perú.
CREATE TABLE meal_service_rules (
    meal_type meal_type PRIMARY KEY,
    claim_start TIME NOT NULL,
    claim_end TIME NOT NULL,
    timezone VARCHAR(100) NOT NULL DEFAULT 'America/Lima',
    description TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT meal_service_rules_valid_window CHECK (claim_start < claim_end)
);

INSERT INTO meal_service_rules (meal_type, claim_start, claim_end, description)
VALUES
    ('DESAYUNO', '06:00', '10:00', 'Disponible de 06:00 a 10:00, hora de Perú.'),
    ('TARDE', '12:00', '15:00', 'Disponible de 12:00 a 15:00, hora de Perú.'),
    ('NOCHE', '18:00', '22:00', 'Disponible de 18:00 a 22:00, hora de Perú.');

-- Registro único por trabajador, comida y fecha de servicio.
CREATE TABLE meal_claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    worker_id UUID NOT NULL REFERENCES worker_information(user_id) ON DELETE RESTRICT,
    shift_assignment_id UUID NOT NULL REFERENCES worker_shift_assignments(id) ON DELETE RESTRICT,
    meal_type meal_type NOT NULL REFERENCES meal_service_rules(meal_type) ON DELETE RESTRICT,
    service_date DATE NOT NULL,
    claimed_at TIMESTAMPTZ,
    status meal_claim_status NOT NULL DEFAULT 'REQUESTED',
    validated_at TIMESTAMPTZ,
    validated_by UUID REFERENCES users(id) ON DELETE RESTRICT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT meal_claims_status_consistent CHECK (
        (status = 'REQUESTED' AND claimed_at IS NOT NULL AND validated_at IS NULL AND validated_by IS NULL)
        OR (status = 'VALIDATED' AND claimed_at IS NOT NULL AND validated_at IS NOT NULL AND validated_by IS NOT NULL)
        OR (status = 'NOT_CONSUMED' AND claimed_at IS NULL AND validated_at IS NULL AND validated_by IS NULL)
        OR (status = 'REQUESTED_BUT_NOT_VALIDATED' AND claimed_at IS NOT NULL AND validated_at IS NULL AND validated_by IS NULL)
    ),
    CONSTRAINT meal_claims_once_per_day UNIQUE (worker_id, meal_type, service_date)
);

CREATE INDEX meal_claims_service_date_idx ON meal_claims (service_date);
CREATE INDEX meal_claims_worker_date_idx ON meal_claims (worker_id, service_date);
CREATE INDEX meal_claims_report_idx ON meal_claims (service_date, meal_type);
CREATE INDEX meal_claims_status_created_idx ON meal_claims (status, created_at DESC);
