-- name: ListUsers :many
SELECT u.id, u.username::text AS username, u.display_name, COALESCE(u.email::text, '')::text AS email, u.status, u.must_change_password,
       (u.locked_until IS NOT NULL AND u.locked_until > now())::boolean AS locked,
       u.last_login_at, u.created_at, u.updated_at,
       ARRAY(SELECT r.key FROM user_roles ur JOIN roles r ON r.id = ur.role_id
             WHERE ur.user_id = u.id AND ur.site_id IS NULL ORDER BY r.key)::text[] AS roles,
       EXISTS (SELECT 1 FROM user_mfa m WHERE m.user_id = u.id AND m.enabled_at IS NOT NULL)::boolean AS mfa_enabled,
       u.kiosk,
       count(*) OVER () AS total
FROM users u
WHERE (@query::text = '' OR u.username::text ILIKE '%' || @query || '%' ESCAPE '\'
                         OR u.display_name ILIKE '%' || @query || '%' ESCAPE '\')
  AND (@status::text = '' OR u.status = @status)
  AND (@role::text = '' OR EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
                                   WHERE ur.user_id = u.id AND r.key = @role))
ORDER BY u.username
LIMIT @lim OFFSET @off;

-- name: GetUser :one
SELECT u.id, u.username::text AS username, u.display_name, COALESCE(u.email::text, '')::text AS email, u.status, u.must_change_password,
       (u.locked_until IS NOT NULL AND u.locked_until > now())::boolean AS locked,
       u.last_login_at, u.created_at, u.updated_at,
       ARRAY(SELECT r.key FROM user_roles ur JOIN roles r ON r.id = ur.role_id
             WHERE ur.user_id = u.id AND ur.site_id IS NULL ORDER BY r.key)::text[] AS roles,
       EXISTS (SELECT 1 FROM user_mfa m WHERE m.user_id = u.id AND m.enabled_at IS NOT NULL)::boolean AS mfa_enabled,
       u.kiosk
FROM users u
WHERE u.id = @id;

-- name: CreateUser :one
INSERT INTO users (username, display_name, email, password_hash, must_change_password)
VALUES (@username, @display_name, @email, @password_hash, true)
RETURNING id;

-- name: UpdateUserProfile :execrows
UPDATE users SET display_name = @display_name, email = @email, updated_at = now() WHERE id = @id;

-- name: SetUserStatus :execrows
UPDATE users SET status = @status, updated_at = now() WHERE id = @id;

-- ResetPassword stores a temporary password: it must be changed at the next login, and the lockout is cleared.
-- name: ResetPassword :execrows
UPDATE users
SET password_hash = @password_hash, must_change_password = true, failed_attempts = 0, locked_until = NULL,
    password_changed_at = now(), updated_at = now()
WHERE id = @id;

-- name: RevokeUserSessions :execrows
UPDATE sessions SET revoked_at = now() WHERE user_id = @user_id AND revoked_at IS NULL;

-- name: ListRoles :many
SELECT r.key, r.system, (SELECT count(DISTINCT ur.user_id) FROM user_roles ur WHERE ur.role_id = r.id) AS user_count
FROM roles r
ORDER BY r.key;

-- name: GetRolesByKeys :many
SELECT id, key FROM roles WHERE key = ANY(@keys::text[]);

-- DeleteUserRoles removes the global assignments (site-scoped ones are managed by SetSiteRoles).
-- name: DeleteUserRoles :exec
DELETE FROM user_roles WHERE user_id = @user_id AND site_id IS NULL;

-- name: AddUserRole :exec
INSERT INTO user_roles (user_id, role_id, granted_by) VALUES (@user_id, @role_id, @granted_by);

-- LockAdminRole serialises every change that could remove the last active Admin (role changes, disabling). An
-- advisory transaction lock: the application role has no UPDATE on roles, which SELECT … FOR UPDATE would need.
-- name: LockAdminRole :exec
SELECT pg_advisory_xact_lock(hashtext('sentra.admin_guard'));

-- name: CountOtherActiveAdmins :one
SELECT count(*) FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id AND r.key = 'admin' AND ur.site_id IS NULL
WHERE u.status = 'active' AND u.id <> @user_id;

-- name: DeleteUserMFA :execrows
DELETE FROM user_mfa WHERE user_id = @user_id;

-- name: DeleteUserRecoveryCodes :exec
DELETE FROM mfa_recovery_codes WHERE user_id = @user_id;

-- name: GetUserIDByUsername :one
SELECT id FROM users WHERE username = @username;

-- name: UnlockUser :execrows
UPDATE users SET failed_attempts = 0, locked_until = NULL, updated_at = now() WHERE id = @id;

-- name: SetKiosk :execrows
UPDATE users SET kiosk = @kiosk, updated_at = now() WHERE id = @id;

-- name: GetPasswordHash :one
SELECT password_hash FROM users WHERE id = @id;

-- name: RememberPasswordHash :exec
INSERT INTO password_history (user_id, password_hash) VALUES (@user_id, @password_hash);

-- name: TrimPasswordHistoryTo :exec
DELETE FROM password_history p
WHERE p.user_id = @user_id::uuid AND p.id NOT IN (SELECT h.id FROM password_history h WHERE h.user_id = @user_id::uuid ORDER BY h.id DESC LIMIT @keep);

-- Site-scoped role assignments (v0.4.1, D-084).
-- name: ListUserSiteRoles :many
SELECT r.key AS role, s.id AS site_id, s.code AS site_code, s.name AS site_name, (s.archived_at IS NOT NULL)::boolean AS site_archived
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
JOIN sites s ON s.id = ur.site_id
WHERE ur.user_id = @user_id
ORDER BY s.code, r.key;

-- name: DeleteUserSiteRoles :exec
DELETE FROM user_roles WHERE user_id = @user_id AND site_id IS NOT NULL;

-- name: AddUserSiteRole :exec
INSERT INTO user_roles (user_id, role_id, site_id, granted_by) VALUES (@user_id, @role_id, @site_id, @granted_by);

-- name: GetActiveSitesByIDs :many
SELECT id, code FROM sites WHERE id = ANY(@ids::uuid[]) AND archived_at IS NULL;
