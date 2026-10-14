-- 000010_session_password_policy (v0.3.7, D-068..D-070, D-079): kiosk/monitoring accounts and sessions, and the
-- password history used to refuse re-use. Timeouts and expiry are settings (session.*, password.*; schema in code).
ALTER TABLE users ADD COLUMN kiosk boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN users.kiosk IS 'Monitoring/kiosk account (Viewer only, set by an Admin): no idle/absolute limit, valid only from Allowed IP networks.';

ALTER TABLE sessions ADD COLUMN kiosk boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN sessions.kiosk IS 'Session of a kiosk account: never idles out, revoked when used from outside the Allowed IP list.';

CREATE TABLE password_history (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- argon2id PHC string of a previous password (never the password).
    password_hash text        NOT NULL CHECK (password_hash LIKE '$argon2id$%'),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_history_user_idx ON password_history (user_id, id DESC);
COMMENT ON TABLE password_history IS 'Previous password hashes per user (newest first), trimmed to password.history entries (v0.3.7).';

GRANT SELECT, INSERT, DELETE ON password_history TO sentra_app;
GRANT USAGE, SELECT ON SEQUENCE password_history_id_seq TO sentra_app;
