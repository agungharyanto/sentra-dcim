-- name: ListAllowed :many
SELECT id, cidr::text AS cidr, label, created_at, created_by, updated_at FROM allowed_ips ORDER BY cidr;

-- name: GetAllowed :one
SELECT id, cidr::text AS cidr, label, created_at, created_by, updated_at FROM allowed_ips WHERE id = @id;

-- name: CountAllowed :one
SELECT count(*) FROM allowed_ips;

-- name: InsertAllowed :one
INSERT INTO allowed_ips (cidr, label, created_by) VALUES (@cidr::cidr, @label, @created_by)
ON CONFLICT (cidr) DO NOTHING
RETURNING id, cidr::text AS cidr, label, created_at, created_by, updated_at;

-- name: UpdateAllowedLabel :one
UPDATE allowed_ips SET label = @label, updated_at = now() WHERE id = @id
RETURNING id, cidr::text AS cidr, label, created_at, created_by, updated_at;

-- name: DeleteAllowed :one
DELETE FROM allowed_ips WHERE id = @id RETURNING cidr::text AS cidr, label;

-- name: DeleteAllowedByCIDR :one
DELETE FROM allowed_ips WHERE cidr = @cidr::cidr RETURNING id, label;

-- One writer at a time (mode change, list change, rollback): the lockout guard sees a consistent list.
-- name: LockAllowlist :exec
SELECT pg_advisory_xact_lock(hashtext('sentra.allowlist'));
