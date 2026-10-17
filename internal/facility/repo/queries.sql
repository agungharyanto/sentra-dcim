-- Facility hierarchy (v0.4.1, D-082..D-085). Every query takes the caller's site scope (authz.Scope):
--   (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
-- There are no unscoped variants: rows outside the scope behave exactly like missing rows (404).
-- Updates and archiving require the expected version (optimistic lock); no row back = missing or stale.

-- ---------------------------------------------------------------------------------------------------------------
-- Sites
-- name: ListSites :many
SELECT s.*, count(*) OVER () AS total
FROM sites s
WHERE (@all_sites::bool OR s.id = ANY(@site_ids::uuid[]))
  AND (@include_archived::bool OR s.archived_at IS NULL)
  AND (@status::text = '' OR s.status = @status)
  AND (@query::text = '' OR s.code ILIKE '%' || @query || '%' ESCAPE '\' OR s.name ILIKE '%' || @query || '%' ESCAPE '\')
ORDER BY
  CASE WHEN @sort::text = 'name' THEN lower(s.name) END ASC,
  CASE WHEN @sort::text = '-name' THEN lower(s.name) END DESC,
  CASE WHEN @sort::text = 'updated_at' THEN s.updated_at END ASC,
  CASE WHEN @sort::text = '-updated_at' THEN s.updated_at END DESC,
  CASE WHEN @sort::text = '-code' THEN s.code END DESC,
  s.code ASC
LIMIT @lim OFFSET @off;

-- name: GetSite :one
SELECT * FROM sites s
WHERE s.id = @id AND (@all_sites::bool OR s.id = ANY(@site_ids::uuid[]));

-- name: CreateSite :one
-- Sites are created by global facility managers only (D-084): nothing is inserted otherwise.
INSERT INTO sites (code, name, address, status) SELECT @code, @name, @address, @status WHERE @all_sites::bool
RETURNING *;

-- name: UpdateSite :one
UPDATE sites SET name = @name, address = @address, status = @status, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR id = ANY(@site_ids::uuid[]))
RETURNING *;

-- name: ArchiveSite :one
UPDATE sites SET archived_at = now(), version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR id = ANY(@site_ids::uuid[]))
RETURNING *;

-- name: CountActiveBuildings :one
SELECT count(*) FROM buildings WHERE site_id = @site_id AND archived_at IS NULL AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]));

-- ---------------------------------------------------------------------------------------------------------------
-- Buildings
-- name: ListBuildings :many
SELECT b.*, s.code AS site_code, s.name AS site_name, count(*) OVER () AS total
FROM buildings b JOIN sites s ON s.id = b.site_id
WHERE (@all_sites::bool OR b.site_id = ANY(@site_ids::uuid[]))
  AND (@include_archived::bool OR b.archived_at IS NULL)
  AND (sqlc.narg(site_id)::uuid IS NULL OR b.site_id = sqlc.narg(site_id))
  AND (@query::text = '' OR b.name ILIKE '%' || @query || '%' ESCAPE '\' OR b.code ILIKE '%' || @query || '%' ESCAPE '\')
ORDER BY
  CASE WHEN @sort::text = '-name' THEN lower(b.name) END DESC,
  CASE WHEN @sort::text = 'updated_at' THEN b.updated_at END ASC,
  CASE WHEN @sort::text = '-updated_at' THEN b.updated_at END DESC,
  s.code ASC, lower(b.name) ASC
LIMIT @lim OFFSET @off;

-- name: GetBuilding :one
SELECT b.*, s.code AS site_code, s.name AS site_name
FROM buildings b JOIN sites s ON s.id = b.site_id
WHERE b.id = @id AND (@all_sites::bool OR b.site_id = ANY(@site_ids::uuid[]));

-- name: CreateBuilding :one
INSERT INTO buildings (site_id, code, name, notes)
SELECT @site_id, @code, @name, @notes WHERE (@all_sites::bool OR @site_id::uuid = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: UpdateBuilding :one
UPDATE buildings SET code = @code, name = @name, notes = @notes, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: ArchiveBuilding :one
UPDATE buildings SET archived_at = now(), version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: CountActiveFloors :one
SELECT count(*) FROM floors WHERE building_id = @building_id AND archived_at IS NULL AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]));

-- ---------------------------------------------------------------------------------------------------------------
-- Floors
-- name: ListFloors :many
SELECT f.*, b.name AS building_name, s.code AS site_code, count(*) OVER () AS total
FROM floors f JOIN buildings b ON b.id = f.building_id JOIN sites s ON s.id = f.site_id
WHERE (@all_sites::bool OR f.site_id = ANY(@site_ids::uuid[]))
  AND (@include_archived::bool OR f.archived_at IS NULL)
  AND (sqlc.narg(site_id)::uuid IS NULL OR f.site_id = sqlc.narg(site_id))
  AND (sqlc.narg(building_id)::uuid IS NULL OR f.building_id = sqlc.narg(building_id))
  AND (@query::text = '' OR f.name ILIKE '%' || @query || '%' ESCAPE '\')
ORDER BY
  CASE WHEN @sort::text = 'name' THEN lower(f.name) END ASC,
  CASE WHEN @sort::text = '-name' THEN lower(f.name) END DESC,
  CASE WHEN @sort::text = '-level' THEN f.level END DESC,
  s.code ASC, lower(b.name) ASC, f.level ASC, lower(f.name) ASC
LIMIT @lim OFFSET @off;

-- name: GetFloor :one
SELECT f.*, b.name AS building_name, s.code AS site_code, s.name AS site_name
FROM floors f JOIN buildings b ON b.id = f.building_id JOIN sites s ON s.id = f.site_id
WHERE f.id = @id AND (@all_sites::bool OR f.site_id = ANY(@site_ids::uuid[]));

-- name: CreateFloor :one
INSERT INTO floors (building_id, site_id, name, level, notes)
SELECT @building_id, @site_id, @name, @level, @notes WHERE (@all_sites::bool OR @site_id::uuid = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: UpdateFloor :one
UPDATE floors SET name = @name, level = @level, notes = @notes, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: ArchiveFloor :one
UPDATE floors SET archived_at = now(), version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: CountActiveRooms :one
SELECT count(*) FROM rooms WHERE floor_id = @floor_id AND archived_at IS NULL AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]));

-- ---------------------------------------------------------------------------------------------------------------
-- Rooms
-- name: ListRooms :many
SELECT r.*, f.name AS floor_name, b.id AS building_id, b.name AS building_name, s.code AS site_code,
       (SELECT count(*) FROM racks k WHERE k.room_id = r.id AND k.archived_at IS NULL) AS rack_count,
       count(*) OVER () AS total
FROM rooms r JOIN floors f ON f.id = r.floor_id JOIN buildings b ON b.id = f.building_id JOIN sites s ON s.id = r.site_id
WHERE (@all_sites::bool OR r.site_id = ANY(@site_ids::uuid[]))
  AND (@include_archived::bool OR r.archived_at IS NULL)
  AND (sqlc.narg(site_id)::uuid IS NULL OR r.site_id = sqlc.narg(site_id))
  AND (sqlc.narg(floor_id)::uuid IS NULL OR r.floor_id = sqlc.narg(floor_id))
  AND (@room_type::text = '' OR r.room_type = @room_type)
  AND (@query::text = '' OR r.name ILIKE '%' || @query || '%' ESCAPE '\')
ORDER BY
  CASE WHEN @sort::text = '-name' THEN lower(r.name) END DESC,
  CASE WHEN @sort::text = 'room_type' THEN r.room_type END ASC,
  s.code ASC, lower(b.name) ASC, f.level ASC, lower(r.name) ASC
LIMIT @lim OFFSET @off;

-- name: GetRoom :one
SELECT r.*, f.name AS floor_name, b.id AS building_id, b.name AS building_name, s.code AS site_code, s.name AS site_name,
       (SELECT count(*) FROM racks k WHERE k.room_id = r.id AND k.archived_at IS NULL) AS rack_count
FROM rooms r JOIN floors f ON f.id = r.floor_id JOIN buildings b ON b.id = f.building_id JOIN sites s ON s.id = r.site_id
WHERE r.id = @id AND (@all_sites::bool OR r.site_id = ANY(@site_ids::uuid[]));

-- name: CreateRoom :one
INSERT INTO rooms (floor_id, site_id, name, room_type, notes)
SELECT @floor_id, @site_id, @name, @room_type, @notes WHERE (@all_sites::bool OR @site_id::uuid = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: UpdateRoom :one
UPDATE rooms SET name = @name, room_type = @room_type, notes = @notes, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: ArchiveRoom :one
UPDATE rooms SET archived_at = now(), version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- ---------------------------------------------------------------------------------------------------------------
-- Racks (numeric limits are written as float8 and read as numeric(…,2))
-- name: ListRacks :many
SELECT k.id, k.room_id, k.site_id, k.name, k.asset_tag, k.row_label, k.row_position, k.u_height, k.starting_unit,
       k.desc_units, k.width_in, k.depth_mm, k.max_weight_kg, k.max_power_kw,
       k.status, k.notes, k.version, k.created_at, k.updated_at, k.archived_at,
       r.name AS room_name, r.room_type, f.id AS floor_id, f.name AS floor_name, b.id AS building_id, b.name AS building_name,
       s.code AS site_code, s.name AS site_name,
       count(*) OVER () AS total
FROM racks k JOIN rooms r ON r.id = k.room_id JOIN floors f ON f.id = r.floor_id
     JOIN buildings b ON b.id = f.building_id JOIN sites s ON s.id = k.site_id
WHERE (@all_sites::bool OR k.site_id = ANY(@site_ids::uuid[]))
  AND (@include_archived::bool OR k.archived_at IS NULL)
  AND (sqlc.narg(site_id)::uuid IS NULL OR k.site_id = sqlc.narg(site_id))
  AND (sqlc.narg(room_id)::uuid IS NULL OR k.room_id = sqlc.narg(room_id))
  AND (@status::text = '' OR k.status = @status)
  AND (@query::text = '' OR k.name ILIKE '%' || @query || '%' ESCAPE '\' OR k.asset_tag ILIKE '%' || @query || '%' ESCAPE '\'
                         OR k.row_label ILIKE '%' || @query || '%' ESCAPE '\')
ORDER BY
  CASE WHEN @sort::text = 'name' THEN lower(k.name) END ASC,
  CASE WHEN @sort::text = '-name' THEN lower(k.name) END DESC,
  CASE WHEN @sort::text = 'status' THEN k.status END ASC,
  CASE WHEN @sort::text = '-status' THEN k.status END DESC,
  CASE WHEN @sort::text = 'updated_at' THEN k.updated_at END ASC,
  CASE WHEN @sort::text = '-updated_at' THEN k.updated_at END DESC,
  s.code ASC, lower(r.name) ASC, k.row_label ASC, k.row_position ASC NULLS LAST, lower(k.name) ASC
LIMIT @lim OFFSET @off;

-- name: GetRack :one
SELECT k.id, k.room_id, k.site_id, k.name, k.asset_tag, k.row_label, k.row_position, k.u_height, k.starting_unit,
       k.desc_units, k.width_in, k.depth_mm, k.max_weight_kg, k.max_power_kw,
       k.status, k.notes, k.version, k.created_at, k.updated_at, k.archived_at,
       r.name AS room_name, r.room_type, f.id AS floor_id, f.name AS floor_name, b.id AS building_id, b.name AS building_name,
       s.code AS site_code, s.name AS site_name
FROM racks k JOIN rooms r ON r.id = k.room_id JOIN floors f ON f.id = r.floor_id
     JOIN buildings b ON b.id = f.building_id JOIN sites s ON s.id = k.site_id
WHERE k.id = @id AND (@all_sites::bool OR k.site_id = ANY(@site_ids::uuid[]));

-- name: CreateRack :one
INSERT INTO racks (room_id, site_id, name, asset_tag, row_label, row_position, u_height, starting_unit, desc_units,
                   width_in, depth_mm, max_weight_kg, max_power_kw, status, notes)
SELECT @room_id, @site_id, @name, sqlc.narg(asset_tag), @row_label, sqlc.narg(row_position), @u_height, @starting_unit,
       @desc_units, @width_in, sqlc.narg(depth_mm), sqlc.narg(max_weight_kg)::float8, sqlc.narg(max_power_kw)::float8,
       @status, @notes
WHERE (@all_sites::bool OR @site_id::uuid = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: UpdateRack :one
UPDATE racks SET room_id = @room_id, name = @name, asset_tag = sqlc.narg(asset_tag), row_label = @row_label,
       row_position = sqlc.narg(row_position), u_height = @u_height, starting_unit = @starting_unit, desc_units = @desc_units,
       width_in = @width_in, depth_mm = sqlc.narg(depth_mm), max_weight_kg = sqlc.narg(max_weight_kg)::float8,
       max_power_kw = sqlc.narg(max_power_kw)::float8, status = @status, notes = @notes,
       version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- name: ArchiveRack :one
UPDATE racks SET archived_at = now(), version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND archived_at IS NULL
  AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
RETURNING id;

-- ---------------------------------------------------------------------------------------------------------------
-- Tree (non-archived nodes in scope)
-- name: TreeSites :many
SELECT id, code, name, status FROM sites
WHERE archived_at IS NULL AND (@all_sites::bool OR id = ANY(@site_ids::uuid[]))
ORDER BY code;

-- name: TreeBuildings :many
SELECT id, site_id, code, name FROM buildings
WHERE archived_at IS NULL AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
ORDER BY lower(name);

-- name: TreeFloors :many
SELECT id, building_id, name, level FROM floors
WHERE archived_at IS NULL AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
ORDER BY level, lower(name);

-- name: TreeRooms :many
SELECT id, floor_id, name, room_type FROM rooms
WHERE archived_at IS NULL AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
ORDER BY lower(name);

-- name: TreeRacks :many
SELECT id, room_id, name, status FROM racks
WHERE archived_at IS NULL AND (@all_sites::bool OR site_id = ANY(@site_ids::uuid[]))
ORDER BY row_label, row_position NULLS LAST, lower(name);
