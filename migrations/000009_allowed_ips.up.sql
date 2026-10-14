-- 000009_allowed_ips: Allowed IP for the web panel (v0.3.6, D-078). The mode (off/monitor/enforce) and the pending
-- auto-rollback deadline live in `settings` (keys allowlist.mode, allowlist.pending_until, schema in code); this
-- table holds the allowed networks. Single addresses are stored as /32 or /128.
CREATE TABLE allowed_ips (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    cidr       cidr        NOT NULL UNIQUE,
    label      text        NOT NULL DEFAULT '' CHECK (length(label) <= 100),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid        NULL REFERENCES users (id),
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE allowed_ips IS 'Networks allowed to use the web panel when allowlist.mode = enforce (v0.3.6).';

GRANT SELECT, INSERT, UPDATE, DELETE ON allowed_ips TO sentra_app;
