-- 000011_facility: facility hierarchy Site > Building > Floor > Room > Rack (v0.4.1, D-082..D-085) and site-scoped
-- role assignments (user_roles.site_id, D-084).
--
-- Every level below Site carries site_id (denormalised) so data can be filtered by site scope with one column; it
-- cannot drift because each parent reference is a composite foreign key (parent_id, site_id) → parent(id, site_id).
-- Rows are archived (archived_at), never deleted (the application role has no DELETE). Names are unique per parent,
-- case-insensitive, among non-archived rows (a name can be reused after archiving); rack names are unique per site.
-- Enumerations are text + CHECK (adding a value is a plain constraint change, sqlc maps them to string).
-- `version` is the optimistic lock (ETag / If-Match): every update increments it.

CREATE TABLE sites (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text        NOT NULL CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{0,15}$'),
    name        text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100 AND name = btrim(name)),
    address     text        NOT NULL DEFAULT '' CHECK (length(address) <= 500),
    status      text        NOT NULL DEFAULT 'active' CHECK (status IN ('planned', 'active', 'inactive')),
    version     integer     NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz NULL
);
-- Site codes identify a site in reports and labels: unique for ever (archived sites keep theirs).
CREATE UNIQUE INDEX sites_code_uq ON sites (code);
CREATE UNIQUE INDEX sites_name_uq ON sites (lower(name)) WHERE archived_at IS NULL;
COMMENT ON TABLE sites IS 'Data center sites (v0.4.1). Top of the facility hierarchy; scope unit of site-scoped roles.';

CREATE TABLE buildings (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id     uuid        NOT NULL REFERENCES sites (id),
    code        text        NOT NULL DEFAULT '' CHECK (code = '' OR code ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,15}$'),
    name        text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100 AND name = btrim(name)),
    notes       text        NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    version     integer     NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz NULL,
    UNIQUE (id, site_id)
);
CREATE UNIQUE INDEX buildings_name_uq ON buildings (site_id, lower(name)) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX buildings_code_uq ON buildings (site_id, lower(code)) WHERE archived_at IS NULL AND code <> '';

CREATE TABLE floors (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    building_id uuid        NOT NULL,
    site_id     uuid        NOT NULL,
    name        text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100 AND name = btrim(name)),
    level       integer     NOT NULL DEFAULT 0 CHECK (level BETWEEN -20 AND 200),
    notes       text        NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    version     integer     NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz NULL,
    UNIQUE (id, site_id),
    FOREIGN KEY (building_id, site_id) REFERENCES buildings (id, site_id)
);
CREATE UNIQUE INDEX floors_name_uq ON floors (building_id, lower(name)) WHERE archived_at IS NULL;
CREATE INDEX floors_site_idx ON floors (site_id);

CREATE TABLE rooms (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    floor_id    uuid        NOT NULL,
    site_id     uuid        NOT NULL,
    name        text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100 AND name = btrim(name)),
    room_type   text        NOT NULL DEFAULT 'whitespace'
                            CHECK (room_type IN ('whitespace', 'mmr', 'power', 'cooling', 'noc', 'storage', 'staging', 'other')),
    notes       text        NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    version     integer     NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz NULL,
    UNIQUE (id, site_id),
    FOREIGN KEY (floor_id, site_id) REFERENCES floors (id, site_id)
);
CREATE UNIQUE INDEX rooms_name_uq ON rooms (floor_id, lower(name)) WHERE archived_at IS NULL;
CREATE INDEX rooms_site_idx ON rooms (site_id);

-- Rack names are chosen by the operator (D-083): 1–64 characters, letters, digits, space and . _ - /, starting with a
-- letter or digit, no surrounding spaces. Relations always use the UUID, so a rename never breaks a reference.
CREATE TABLE racks (
    id            uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id       uuid          NOT NULL,
    site_id       uuid          NOT NULL,
    name          text          NOT NULL CHECK (name ~ '^[A-Za-z0-9][A-Za-z0-9 ._/-]{0,63}$' AND name = btrim(name)),
    asset_tag     text          NULL CHECK (asset_tag ~ '^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$'),
    row_label     text          NOT NULL DEFAULT '' CHECK (length(row_label) <= 16 AND row_label = btrim(row_label)),
    row_position  integer       NULL CHECK (row_position BETWEEN 1 AND 999),
    u_height      integer       NOT NULL DEFAULT 42 CHECK (u_height BETWEEN 1 AND 60),
    starting_unit integer       NOT NULL DEFAULT 1 CHECK (starting_unit BETWEEN 1 AND 100),
    desc_units    boolean       NOT NULL DEFAULT false,
    width_in      integer       NOT NULL DEFAULT 19 CHECK (width_in IN (19, 23)),
    depth_mm      integer       NULL CHECK (depth_mm BETWEEN 100 AND 2000),
    max_weight_kg numeric(8, 2) NULL CHECK (max_weight_kg > 0),
    max_power_kw  numeric(6, 2) NULL CHECK (max_power_kw > 0),
    status        text          NOT NULL DEFAULT 'planned'
                                CHECK (status IN ('planned', 'active', 'reserved', 'decommissioning', 'retired')),
    notes         text          NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    version       integer       NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at    timestamptz   NOT NULL DEFAULT now(),
    updated_at    timestamptz   NOT NULL DEFAULT now(),
    archived_at   timestamptz   NULL,
    FOREIGN KEY (room_id, site_id) REFERENCES rooms (id, site_id)
);
CREATE UNIQUE INDEX racks_name_uq ON racks (site_id, lower(name)) WHERE archived_at IS NULL;
-- Asset tags are physical labels: unique for ever when set (archived racks keep theirs).
CREATE UNIQUE INDEX racks_asset_tag_uq ON racks (lower(asset_tag)) WHERE asset_tag IS NOT NULL;
CREATE INDEX racks_room_idx ON racks (room_id);
COMMENT ON TABLE racks IS 'Racks (v0.4.1). site_id follows the room through the composite foreign key.';

-- Facility data is archived, never deleted (D-085): no DELETE for the application role.
GRANT SELECT, INSERT, UPDATE ON sites, buildings, floors, rooms, racks TO sentra_app;

-- Site-scoped role assignments (D-084): site_id NULL = the role applies everywhere (every assignment so far); a site
-- limits the assignment to that site and grants only the site-scoped permissions of the role (rbac.ResolveSite).
-- The primary key (user_id, role_id) becomes a unique key that treats NULL as a value: one global assignment and at
-- most one per site for each role.
ALTER TABLE user_roles ADD COLUMN site_id uuid NULL REFERENCES sites (id);
ALTER TABLE user_roles DROP CONSTRAINT user_roles_pkey;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_uq UNIQUE NULLS NOT DISTINCT (user_id, role_id, site_id);
-- Admin is global by definition (users, settings and the audit log have no site); a CHECK cannot look up the role
-- key, so a trigger refuses it (the application validates first and answers 422).
CREATE FUNCTION user_roles_admin_global() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.site_id IS NOT NULL AND EXISTS (SELECT 1 FROM roles WHERE id = NEW.role_id AND key = 'admin') THEN
        RAISE EXCEPTION 'the admin role cannot be assigned per site' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END
$$;
CREATE TRIGGER user_roles_admin_global BEFORE INSERT OR UPDATE ON user_roles
    FOR EACH ROW EXECUTE FUNCTION user_roles_admin_global();
COMMENT ON COLUMN user_roles.site_id IS 'NULL = global assignment; else the role applies to this site only (v0.4.1, D-084).';
