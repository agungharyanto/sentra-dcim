-- 000001_init: base extensions and system metadata.
-- TimescaleDB is preinstalled by the image; IF NOT EXISTS keeps this idempotent on plain PostgreSQL+Timescale.
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS system_info (
    key        text        PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{0,63}$'),
    value      text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE system_info IS 'Key/value metadata about this Sentra DCIM installation. All timestamps are UTC.';

INSERT INTO system_info (key, value)
VALUES ('installation.initialized_at', to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'))
ON CONFLICT (key) DO NOTHING;
