-- 0002_create_audit_log — immutable change log (from 0001_init).
-- One row per mutation, written inside the same transaction as the change.
-- NEVER store raw request bodies / passwords / tokens.

CREATE TABLE audit_log (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id bigint,
    actor_name    text,                            -- denormalized snapshot
    actor_role    text,
    action        text        NOT NULL,            -- taxonomy: <entity>.<verb>
    entity_type   text        NOT NULL,
    entity_id     bigint,
    entity_name   text,
    summary       text,                            -- human-readable description
    changes       jsonb,                           -- selective before/after diff (no secrets)
    ip_address    text,
    request_id    text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_entity     ON audit_log (entity_type, entity_id);
CREATE INDEX idx_audit_log_actor      ON audit_log (actor_user_id);
CREATE INDEX idx_audit_log_action     ON audit_log (action);
CREATE INDEX idx_audit_log_created_at ON audit_log (created_at);
