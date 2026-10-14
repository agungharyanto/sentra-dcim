-- 000006_db_roles: the application runs as a DML-only role (ISS-039, v0.3.4). The migration owner (POSTGRES_USER)
-- keeps DDL; the app's login role (DB_APP_USER, created with its password by `sentra db sync-app-role`, never here)
-- is a member of the group role sentra_app below. Every table must have an explicit decision here or in a later
-- migration (test/integration TestAppRoleGrants fails otherwise).
DO $$
BEGIN
    CREATE ROLE sentra_app NOLOGIN;
EXCEPTION WHEN duplicate_object THEN
    NULL; -- roles are cluster-wide: another database of the cluster created it already
END
$$;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO sentra_app;

-- Accounts are never deleted (disabled instead): no DELETE on users.
GRANT SELECT, INSERT, UPDATE ON users TO sentra_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON sessions, user_roles, user_mfa, mfa_recovery_codes TO sentra_app;
GRANT SELECT ON roles, schema_migrations, system_info, ops_runs TO sentra_app;
-- Audit log: append and read only, enforced by privileges (not only by the trigger).
GRANT SELECT, INSERT ON audit_events TO sentra_app;
GRANT USAGE, SELECT ON SEQUENCE audit_events_id_seq, mfa_recovery_codes_id_seq TO sentra_app;
