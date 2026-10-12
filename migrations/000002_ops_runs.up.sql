-- 000002_ops_runs: audit trail of operational runs (backup, restore, rollback).
CREATE TABLE IF NOT EXISTS ops_runs (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind        text        NOT NULL CHECK (kind IN ('backup', 'restore', 'rollback')),
    status      text        NOT NULL CHECK (status IN ('success', 'failed')),
    ref         text        NOT NULL DEFAULT '' CHECK (length(ref) <= 255),
    app_version text        NOT NULL DEFAULT '' CHECK (length(app_version) <= 64),
    details     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ops_runs_kind_created_at_idx ON ops_runs (kind, created_at DESC);

COMMENT ON TABLE ops_runs IS 'Operational runs recorded by ops/*.sh. Timestamps are UTC.';
