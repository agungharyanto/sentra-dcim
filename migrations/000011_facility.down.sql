-- Reverts 000011_facility. Site-scoped role assignments are removed first: a v0.3 binary does not know site_id and
-- would otherwise read them as global roles. Facility data is dropped (take a backup before rolling back).
DROP TRIGGER IF EXISTS user_roles_admin_global ON user_roles;
DROP FUNCTION IF EXISTS user_roles_admin_global();
DELETE FROM user_roles WHERE site_id IS NOT NULL;
ALTER TABLE user_roles DROP CONSTRAINT IF EXISTS user_roles_uq;
ALTER TABLE user_roles DROP COLUMN IF EXISTS site_id;
ALTER TABLE user_roles ADD PRIMARY KEY (user_id, role_id);
DROP TABLE IF EXISTS racks;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS floors;
DROP TABLE IF EXISTS buildings;
DROP TABLE IF EXISTS sites;
