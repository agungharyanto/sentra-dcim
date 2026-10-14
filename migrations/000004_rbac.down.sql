-- Reverts 000004_rbac. Role assignments are lost; v0.3.1 has no roles.
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
