-- 000003_auth (down): removes accounts, sessions and the core audit log. Destroys their data — take a backup first.
DROP TABLE IF EXISTS audit_events;
DROP FUNCTION IF EXISTS audit_events_append_only();
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS citext;
