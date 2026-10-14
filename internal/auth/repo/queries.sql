-- CreateAdminUser creates an account with the Admin role in one statement (CLI create-admin; atomic).
-- name: CreateAdminUser :one
WITH u AS (
    INSERT INTO users (username, display_name, email, password_hash, must_change_password)
    VALUES (@username, @display_name, @email, @password_hash, true)
    RETURNING *
), granted AS (
    INSERT INTO user_roles (user_id, role_id)
    SELECT u.id, r.id FROM u CROSS JOIN roles r WHERE r.key = 'admin'
    RETURNING role_id
)
SELECT u.*, (SELECT count(*) FROM granted)::int AS roles_granted FROM u;

-- name: GetUserRoleKeys :many
SELECT r.key FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = @user_id ORDER BY r.key;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- RecordLoginFailure counts a wrong password and locks the account at the threshold. A lock that has already
-- expired starts a fresh count, so an unlocked account gets the full number of attempts again.
-- name: RecordLoginFailure :one
UPDATE users AS u
SET failed_attempts = c.attempts,
    locked_until    = CASE WHEN c.attempts >= @threshold::int THEN now() + make_interval(secs => @lock_seconds::float8)
                           WHEN u.locked_until <= now() THEN NULL
                           ELSE u.locked_until END,
    updated_at      = now()
FROM (SELECT x.id AS uid,
             CASE WHEN x.locked_until IS NOT NULL AND x.locked_until <= now() THEN 1 ELSE x.failed_attempts + 1 END AS attempts
      FROM users AS x WHERE x.id = @user_id) AS c
WHERE u.id = c.uid
RETURNING u.failed_attempts, u.locked_until;

-- name: RecordLoginSuccess :exec
UPDATE users SET failed_attempts = 0, locked_until = NULL, last_login_at = now(), updated_at = now() WHERE id = @id;

-- name: UpdatePasswordHash :exec
UPDATE users
SET password_hash = @password_hash, must_change_password = @must_change_password,
    password_changed_at = now(), updated_at = now()
WHERE id = @id;

-- name: RehashPassword :exec
UPDATE users SET password_hash = @password_hash, updated_at = now() WHERE id = @id;

-- name: CreateSession :one
INSERT INTO sessions (token_hash, user_id, expires_at, idle_expires_at, ip, user_agent)
VALUES (@token_hash, @user_id, @expires_at, @idle_expires_at, @ip, @user_agent)
RETURNING *;

-- name: GetActiveSession :one
SELECT s.id AS session_id, s.user_id, s.created_at AS session_created_at, s.last_seen_at, s.expires_at, s.idle_expires_at,
       u.username, u.display_name, u.status, u.must_change_password,
       ARRAY(SELECT r.key FROM user_roles ur JOIN roles r ON r.id = ur.role_id
             WHERE ur.user_id = u.id ORDER BY r.key)::text[] AS roles,
       (s.mfa_verified_at IS NOT NULL)::boolean AS mfa_verified,
       EXISTS (SELECT 1 FROM user_mfa m WHERE m.user_id = u.id AND m.enabled_at IS NOT NULL)::boolean AS mfa_enabled
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND s.idle_expires_at > now()
  AND u.status = 'active';

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now(), idle_expires_at = LEAST(@idle_expires_at, expires_at) WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeSession :execrows
UPDATE sessions SET revoked_at = now() WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeOtherSessions :execrows
UPDATE sessions SET revoked_at = now() WHERE user_id = @user_id AND id <> @keep_id AND revoked_at IS NULL;

-- name: ListActiveSessions :many
SELECT id, created_at, last_seen_at, expires_at, idle_expires_at, ip, user_agent
FROM sessions
WHERE user_id = @user_id AND revoked_at IS NULL AND expires_at > now() AND idle_expires_at > now()
ORDER BY last_seen_at DESC;

-- name: DeleteStaleSessions :execrows
DELETE FROM sessions
WHERE expires_at < @cutoff OR idle_expires_at < @cutoff OR revoked_at < @cutoff;

-- name: GetUserMFA :one
SELECT * FROM user_mfa WHERE user_id = @user_id;

-- UpsertPendingMFA stores a new enrolment secret unless MFA is already active (then no row is returned).
-- name: UpsertPendingMFA :one
INSERT INTO user_mfa (user_id, secret_enc) VALUES (@user_id, @secret_enc)
ON CONFLICT (user_id) DO UPDATE SET secret_enc = EXCLUDED.secret_enc, last_used_step = 0, updated_at = now()
WHERE user_mfa.enabled_at IS NULL
RETURNING user_id;

-- name: EnableMFA :execrows
UPDATE user_mfa SET enabled_at = now(), last_used_step = @step, updated_at = now()
WHERE user_id = @user_id AND enabled_at IS NULL AND last_used_step < @step;

-- UseTOTPStep records an accepted code atomically: a concurrent or later replay of the same step updates nothing.
-- name: UseTOTPStep :execrows
UPDATE user_mfa SET last_used_step = @step, updated_at = now()
WHERE user_id = @user_id AND enabled_at IS NOT NULL AND last_used_step < @step;

-- name: MarkSessionMFAVerified :exec
UPDATE sessions SET mfa_verified_at = now() WHERE id = @id AND revoked_at IS NULL;

-- name: DeleteRecoveryCodes :exec
DELETE FROM mfa_recovery_codes WHERE user_id = @user_id;

-- name: InsertRecoveryCode :exec
INSERT INTO mfa_recovery_codes (user_id, code_hash) VALUES (@user_id, @code_hash);

-- name: UseRecoveryCode :execrows
UPDATE mfa_recovery_codes SET used_at = now() WHERE user_id = @user_id AND code_hash = @code_hash AND used_at IS NULL;

-- name: CountUnusedRecoveryCodes :one
SELECT count(*) FROM mfa_recovery_codes WHERE user_id = @user_id AND used_at IS NULL;

-- name: RevokeUserSessions :execrows
UPDATE sessions SET revoked_at = now() WHERE user_id = @user_id AND revoked_at IS NULL;
