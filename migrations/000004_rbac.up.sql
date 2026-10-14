-- 000004_rbac: roles in the database, permissions in code (internal/platform/rbac). v0.3.2, ADR-003, D-058.
-- Backward compatible: only new objects; v0.3.1 binaries ignore them (they grant no permissions at all).
CREATE TABLE roles (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Stable key referenced by code (rbac.Role*) and translated by the UI (users:roles.<key>).
    key        text        NOT NULL UNIQUE CHECK (key ~ '^[a-z][a-z0-9_]{1,31}$'),
    -- System roles are seeded here and cannot be renamed or deleted; their permissions are defined in code.
    system     boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE roles IS 'Roles (v0.3.2). Permissions of system roles are defined in code (internal/platform/rbac).';

CREATE TABLE user_roles (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    uuid        NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL DEFAULT now(),
    -- Who granted it (NULL: migration or CLI).
    granted_by uuid        NULL REFERENCES users (id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, role_id)
);
CREATE INDEX user_roles_role_idx ON user_roles (role_id);
COMMENT ON TABLE user_roles IS 'Role assignments (v0.3.2). A user has the union of the permissions of its roles.';

INSERT INTO roles (key, system) VALUES
    ('admin', true), ('noc', true), ('security', true), ('technician', true), ('viewer', true), ('auditor', true);

-- Every v0.3.1 account was created by `sentra user create-admin` (the only way to create accounts then), so all
-- existing accounts become Admin.
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u CROSS JOIN roles r WHERE r.key = 'admin';
