-- name: ListSettings :many
SELECT key, value, updated_at, updated_by FROM settings WHERE starts_with(key, @prefix::text) ORDER BY key;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value, updated_at, updated_by) VALUES (@key, @value, now(), @updated_by)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now(), updated_by = EXCLUDED.updated_by;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE key = @key;

-- name: ListPreferences :many
SELECT key, value, updated_at FROM user_preferences WHERE user_id = @user_id ORDER BY key;

-- name: UpsertPreference :exec
INSERT INTO user_preferences (user_id, key, value, updated_at) VALUES (@user_id, @key, @value, now())
ON CONFLICT (user_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: DeletePreference :exec
DELETE FROM user_preferences WHERE user_id = @user_id AND key = @key;

-- Serialises concurrent writers of the same scope so "old value" in the audit trail is exact.
-- name: LockSettings :exec
SELECT pg_advisory_xact_lock(hashtext('sentra.settings'), hashtext(@scope::text));
