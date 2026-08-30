CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('ADMIN', 'OWNER', 'RRHH', 'WORKER');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100),
    dni VARCHAR(30),
    email VARCHAR(320),
    password_hash TEXT,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    role user_role NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_credentials_by_role CHECK (
        (role IN ('ADMIN', 'OWNER', 'RRHH') AND username IS NOT NULL AND password_hash IS NOT NULL AND dni IS NULL)
        OR
        (role = 'WORKER' AND dni IS NOT NULL AND username IS NULL AND password_hash IS NULL)
    )
);

CREATE UNIQUE INDEX users_username_unique_idx ON users (LOWER(username)) WHERE username IS NOT NULL;
CREATE UNIQUE INDEX users_dni_unique_idx ON users (dni) WHERE dni IS NOT NULL;
CREATE UNIQUE INDEX users_email_unique_idx ON users (LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX users_role_idx ON users (role);

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
CREATE INDEX refresh_tokens_active_idx
    ON refresh_tokens (expires_at)
    WHERE revoked_at IS NULL;
