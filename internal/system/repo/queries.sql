-- name: GetSchemaVersion :one
SELECT version, dirty FROM schema_migrations LIMIT 1;

-- name: GetSystemInfo :one
SELECT key, value, updated_at FROM system_info WHERE key = $1;
