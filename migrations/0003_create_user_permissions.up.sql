-- 0003_create_user_permissions — per-user permission grants (JSONB tree, from 0018).
-- Permissions stored as { "<resource>": { "<action>": true } }. One row per user
-- (user_id UNIQUE). Admin role bypasses this table entirely. Hard-cascade on user
-- delete is fine here — the row only holds derived grants, not audit-worthy data.

CREATE TABLE user_permissions (
    id                bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id           bigint      NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    permission_count  integer     NOT NULL DEFAULT 0,
    permissions       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    created_by        bigint      NOT NULL DEFAULT 0,
    updated_by        bigint      NOT NULL DEFAULT 0
);

CREATE INDEX idx_user_permissions_user ON user_permissions(user_id);
