-- 0005_create_notifications — announcements + per-user notifications (final shape).
-- Consolidated from:
--   0009_create_notifications  — announcements + notifications base
--   0063_notifications_cats    — notifications.category / is_important / link
--
-- No announcement_reads / announcement_replies tables exist: the matching HTTP
-- handlers are stubs with no backing storage.

-- ── announcements: system-wide messages posted by admin/manager ─────────────────
CREATE TABLE announcements (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title        text        NOT NULL DEFAULT '',
    body         text        NOT NULL DEFAULT '',
    priority     text        NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high')),
    published_at timestamptz,
    expires_at   timestamptz,
    deleted_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    created_by   bigint      NOT NULL REFERENCES users(id),
    updated_by   bigint      NOT NULL REFERENCES users(id)
);

CREATE INDEX idx_announcements_active ON announcements (published_at, expires_at) WHERE deleted_at IS NULL;

-- ── notifications: per-user inbox items (system events + announcement fan-out) ──
CREATE TABLE notifications (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      bigint      NOT NULL REFERENCES users(id),
    type         text        NOT NULL DEFAULT 'system',
    category     text        NOT NULL DEFAULT 'task',
    title        text        NOT NULL DEFAULT '',
    body         text        NOT NULL DEFAULT '',
    link         text,
    ref_type     text,
    ref_id       bigint,
    is_read      boolean     NOT NULL DEFAULT false,
    is_important boolean     NOT NULL DEFAULT false,
    read_at      timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user      ON notifications (user_id, is_read, created_at DESC);
CREATE INDEX idx_notifications_ref       ON notifications (ref_type, ref_id) WHERE ref_id IS NOT NULL;
CREATE INDEX idx_notifications_category  ON notifications (user_id, category, created_at DESC);
CREATE INDEX idx_notifications_important ON notifications (user_id, is_important, created_at DESC) WHERE is_important = true;
