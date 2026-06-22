-- 0001_create_users — foundation users table (final consolidated shape).
-- Convention: timestamptz UTC; bigint identity PK (legacy data keeps its ids);
-- audit columns created_at/updated_at/created_by/updated_by + deleted_at (soft-delete).
--
-- Columns of note:
--   password_reset_token_hash / password_reset_expires_at — password reset flow
--   force_password_change — force a password change on next login
--   employment_type fulltime|parttime — generic HR attribute

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id                        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username                  text        NOT NULL,
    password_hash             text        NOT NULL,            -- bcrypt
    full_name                 text        NOT NULL,
    email                     text,
    phone                     text,
    role                      text        NOT NULL DEFAULT 'staff'
        CHECK (role IN ('admin', 'manager', 'staff')),
    status                    text        NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'locked')),
    staff_code                text,
    avatar_url                text,
    force_password_change     boolean     NOT NULL DEFAULT false,
    employment_type           text        NOT NULL DEFAULT 'fulltime'
        CHECK (employment_type IN ('fulltime', 'parttime')),
    password_reset_token_hash text,                            -- SHA-256 hash, never plaintext
    password_reset_expires_at timestamptz,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    created_by                bigint      NOT NULL DEFAULT 0,
    updated_by                bigint      NOT NULL DEFAULT 0,
    deleted_at                timestamptz,
    CONSTRAINT users_username_unique   UNIQUE (username),
    CONSTRAINT users_staff_code_unique UNIQUE (staff_code)
);

CREATE INDEX idx_users_role   ON users (role);
CREATE INDEX idx_users_status ON users (status);
CREATE INDEX idx_users_email  ON users (lower(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_users_password_reset_token_hash
    ON users (password_reset_token_hash)
    WHERE password_reset_token_hash IS NOT NULL;
