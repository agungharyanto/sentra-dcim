-- 000003_auth: local accounts, server-side sessions and the core (append-only) audit log. v0.3.1, ADR-003.
-- Backward compatible: only new objects; v0.2.x binaries ignore them.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Login name (may be a NIP); case-insensitive, unique.
    username             citext      NOT NULL UNIQUE CHECK (username ~ '^[A-Za-z0-9._-]{3,64}$'),
    display_name         text        NOT NULL CHECK (length(display_name) BETWEEN 1 AND 128),
    email                citext      NULL CHECK (email IS NULL OR (length(email) <= 254 AND email ~ '^[^@\s]+@[^@\s]+$')),
    -- argon2id PHC string ($argon2id$v=19$m=…,t=…,p=…$salt$hash). Never the password.
    password_hash        text        NOT NULL CHECK (password_hash LIKE '$argon2id$%'),
    status               text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    must_change_password boolean     NOT NULL DEFAULT false,
    failed_attempts      integer     NOT NULL DEFAULT 0 CHECK (failed_attempts >= 0),
    locked_until         timestamptz NULL,
    password_changed_at  timestamptz NOT NULL DEFAULT now(),
    last_login_at        timestamptz NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE users IS 'Local accounts (v0.3.1). Passwords only as argon2id hashes. Timestamps UTC.';

CREATE TABLE sessions (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- SHA-256 of the opaque session token; the raw token is never stored.
    token_hash      bytea       NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    user_id         uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,          -- absolute limit (12 h)
    idle_expires_at timestamptz NOT NULL,          -- sliding idle limit (30 min)
    ip              inet        NULL,
    user_agent      text        NOT NULL DEFAULT '' CHECK (length(user_agent) <= 512),
    revoked_at      timestamptz NULL
);
CREATE INDEX sessions_user_active_idx ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
COMMENT ON TABLE sessions IS 'Server-side sessions (opaque token, ADR-003). Only token hashes are stored.';

CREATE TABLE audit_events (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at    timestamptz NOT NULL DEFAULT now(),
    actor_type     text        NOT NULL CHECK (actor_type IN ('user', 'system', 'anonymous')),
    actor_id       uuid        NULL,
    actor_username text        NOT NULL DEFAULT '' CHECK (length(actor_username) <= 128),
    action         text        NOT NULL CHECK (action ~ '^[a-z][a-z0-9_.]{2,63}$'),
    target_type    text        NOT NULL DEFAULT '' CHECK (length(target_type) <= 64),
    target_id      text        NOT NULL DEFAULT '' CHECK (length(target_id) <= 128),
    result         text        NOT NULL CHECK (result IN ('success', 'failure', 'denied')),
    ip             inet        NULL,
    user_agent     text        NOT NULL DEFAULT '' CHECK (length(user_agent) <= 512),
    request_id     text        NOT NULL DEFAULT '' CHECK (length(request_id) <= 128),
    metadata       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    -- Tamper-evident hash chain, filled from v0.3.3 (D-051); nullable until then.
    prev_hash      bytea       NULL,
    hash           bytea       NULL
);
CREATE INDEX audit_events_occurred_at_idx ON audit_events (occurred_at DESC);
CREATE INDEX audit_events_actor_idx ON audit_events (actor_id, occurred_at DESC);
CREATE INDEX audit_events_action_idx ON audit_events (action, occurred_at DESC);
COMMENT ON TABLE audit_events IS 'Append-only audit log (ADR-003). UPDATE/DELETE/TRUNCATE are rejected by triggers.';

CREATE FUNCTION audit_events_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only (% rejected)', TG_OP USING ERRCODE = 'insufficient_privilege';
END;
$$;
CREATE TRIGGER audit_events_no_update_delete BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_append_only();
CREATE TRIGGER audit_events_no_truncate BEFORE TRUNCATE ON audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION audit_events_append_only();
