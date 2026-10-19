-- 000011 — Organizations and per-company role isolation (RF-022, ADR 0015, invariants 12-14).
--
-- Every application and every user belongs to exactly one organization. The database itself refuses
-- to hand a user a role of an application owned by another organization, so no code path or direct
-- query can bypass the rule. Fixed UUIDs let later seeds, tests and the transitional defaults in the
-- backend refer to the seeded organizations.
BEGIN;

CREATE TABLE organizations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO organizations (id, slug, name) VALUES
    ('00000000-0000-4000-8000-0000000000a1', 'hub',             'Identity Hub (operador)'),
    ('00000000-0000-4000-8000-0000000000a2', 'contabilidad-sa', 'Contabilidad S.A.'),
    ('00000000-0000-4000-8000-0000000000a3', 'empresa-b',       'Empresa B');

COMMENT ON TABLE organizations IS
    'Companies served by the Hub. "hub" is the platform operator and home of the global administrators.';

-- Applications: the existing Contabilidad belongs to Contabilidad S.A.
ALTER TABLE applications ADD COLUMN organization_id uuid REFERENCES organizations(id);
UPDATE applications SET organization_id = '00000000-0000-4000-8000-0000000000a2';
ALTER TABLE applications ALTER COLUMN organization_id SET NOT NULL;

-- Users: holders of the directory role admin are the operator's; everyone else is Contabilidad's.
-- An admin who already holds application roles stays in Contabilidad S.A., so the backfill never
-- leaves a user with roles of another company (the triggers below would reject that state later).
ALTER TABLE users ADD COLUMN organization_id uuid REFERENCES organizations(id);
UPDATE users
SET organization_id = '00000000-0000-4000-8000-0000000000a2';
UPDATE users
SET organization_id = '00000000-0000-4000-8000-0000000000a1'
WHERE id IN (
    SELECT ur.user_id
    FROM user_roles ur
    JOIN roles r ON r.id = ur.role_id
    WHERE r.name = 'admin'
)
AND NOT EXISTS (
    SELECT 1
    FROM user_roles ur
    JOIN roles r ON r.id = ur.role_id
    WHERE ur.user_id = users.id
      AND r.application_id IS NOT NULL
);
ALTER TABLE users ALTER COLUMN organization_id SET NOT NULL;
CREATE INDEX users_organization_idx ON users (organization_id);

-- Second company demo: Inventario of Empresa B, mirroring how Contabilidad is declared (000007, 000009).
INSERT INTO applications (client_id, redirect_uri, allowed_origin, name, organization_id)
VALUES ('inventario', 'http://inventario.localhost:8080/oauth/callback', 'http://inventario.localhost:8080',
        'Inventario', '00000000-0000-4000-8000-0000000000a3');

INSERT INTO permissions (application_id, key, description)
SELECT id, seed.key, seed.description
FROM applications
CROSS JOIN (VALUES
    ('productos.ver', 'Ver el catálogo de productos'),
    ('productos.editar', 'Crear y editar productos'),
    ('movimientos.registrar', 'Registrar entradas y salidas de bodega')
) AS seed(key, description)
WHERE applications.client_id = 'inventario';

INSERT INTO roles (name, description, application_id, system)
SELECT seed.name, seed.description, applications.id, false
FROM applications
CROSS JOIN (VALUES
    ('inventario.consulta', 'Inventario: solo consulta del catálogo'),
    ('inventario.bodeguero', 'Inventario: consulta, edición y movimientos de bodega')
) AS seed(name, description)
WHERE applications.client_id = 'inventario';

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles
JOIN permissions ON permissions.application_id = roles.application_id
WHERE roles.name = 'inventario.consulta'
  AND permissions.key = 'productos.ver';

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles
JOIN permissions ON permissions.application_id = roles.application_id
WHERE roles.name = 'inventario.bodeguero';

-- ADR 0015 point 2. SQLSTATE IH001 is a custom code the store maps to ErrCrossOrganizationRole.
-- Directory roles (application_id NULL) are exempt. BEFORE row triggers must return NEW.
CREATE FUNCTION enforce_user_role_organization() RETURNS trigger AS $$
DECLARE
    role_organization uuid;
    user_organization uuid;
BEGIN
    SELECT a.organization_id INTO role_organization
    FROM roles r
    JOIN applications a ON a.id = r.application_id
    WHERE r.id = NEW.role_id;
    IF role_organization IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT organization_id INTO user_organization FROM users WHERE id = NEW.user_id;
    IF user_organization IS DISTINCT FROM role_organization THEN
        RAISE EXCEPTION 'role_organization_mismatch: the role belongs to an application of another organization'
            USING ERRCODE = 'IH001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER user_roles_enforce_organization
BEFORE INSERT OR UPDATE ON user_roles
FOR EACH ROW EXECUTE FUNCTION enforce_user_role_organization();

CREATE FUNCTION reject_user_organization_change_with_roles() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS DISTINCT FROM OLD.organization_id AND EXISTS (
        SELECT 1
        FROM user_roles ur
        JOIN roles r ON r.id = ur.role_id
        JOIN applications a ON a.id = r.application_id
        WHERE ur.user_id = NEW.id
          AND a.organization_id <> NEW.organization_id
    ) THEN
        RAISE EXCEPTION 'role_organization_mismatch: the user still holds roles of applications of another organization'
            USING ERRCODE = 'IH001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_reject_organization_change_with_roles
BEFORE UPDATE OF organization_id ON users
FOR EACH ROW EXECUTE FUNCTION reject_user_organization_change_with_roles();

GRANT SELECT ON organizations TO identity_app;

COMMIT;
