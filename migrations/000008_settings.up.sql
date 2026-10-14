-- 000008_settings: generic application settings (v0.3.5, D-076). The schema of every key — type, default, allowed
-- values, min/max, validator, whether users may override it — lives in code (internal/settings), reviewed and
-- tested like permissions; the database stores only values that differ from the default. A missing row means "use
-- the default" (for regional.* the default comes from .env: DISPLAY_TIMEZONE, DEFAULT_LOCALE).
CREATE TABLE settings (
    key        text        PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$' AND length(key) <= 64),
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid        NULL REFERENCES users (id)
);
COMMENT ON TABLE settings IS 'System settings that differ from their default (schema in internal/settings, v0.3.5).';

-- Per-user overrides of user-overridable keys (e.g. regional.*). No row = follow the system value.
CREATE TABLE user_preferences (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key        text        NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$' AND length(key) <= 64),
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);
COMMENT ON TABLE user_preferences IS 'Per-user overrides of user-overridable settings (v0.3.5).';

-- Resetting to the default (or to "follow the system") deletes the row.
GRANT SELECT, INSERT, UPDATE, DELETE ON settings, user_preferences TO sentra_app;
