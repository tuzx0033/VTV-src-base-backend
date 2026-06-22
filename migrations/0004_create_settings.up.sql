-- 0004_create_settings — key/value config store (from 0011).
-- Primary key is the setting key; no surrogate id. Repository reads/writes
-- columns: key, value, description, updated_at, updated_by.

CREATE TABLE settings (
    key         varchar(100) PRIMARY KEY,
    value       text         NOT NULL DEFAULT '',
    description text         NOT NULL DEFAULT '',
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    updated_by  bigint
);

-- seed defaults (foundation only)
INSERT INTO settings (key, value, description) VALUES
    ('site.name',     'App',  'Display name of the platform'),
    ('site.logo_url', '',     'URL to the platform logo')
ON CONFLICT (key) DO NOTHING;
