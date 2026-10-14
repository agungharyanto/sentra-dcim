-- Reverts 000006_db_roles in this database. The group role is dropped only when no other database still uses it.
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM sentra_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM sentra_app;
REVOKE ALL ON SCHEMA public FROM sentra_app;
DO $$
BEGIN
    DROP ROLE sentra_app;
EXCEPTION WHEN dependent_objects_still_exist OR undefined_object THEN
    NULL;
END
$$;
