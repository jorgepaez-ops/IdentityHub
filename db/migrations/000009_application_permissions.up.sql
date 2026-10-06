-- 000009 — Configurable application roles and permissions (RF-009, RF-021).
BEGIN;

CREATE TABLE permissions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    key            text NOT NULL,
    description    text NOT NULL,
    UNIQUE (application_id, key)
);

ALTER TABLE roles
    ADD COLUMN application_id uuid REFERENCES applications(id) ON DELETE CASCADE,
    ADD COLUMN system boolean NOT NULL DEFAULT false;

CREATE TABLE role_permissions (
    role_id       uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

COMMENT ON COLUMN roles.application_id IS
    'NULL identifies a directory role; otherwise the owning application. Application role names (<client_id>.<slug>) are validated by the rolegrid service.';
COMMENT ON COLUMN roles.system IS
    'System directory roles (admin, user) are protected by the roles_reject_system_mutation trigger and refused by the rolegrid service.';

UPDATE roles
SET system = true
WHERE name IN ('admin', 'user');

UPDATE roles
SET application_id = applications.id
FROM applications
WHERE applications.client_id = 'contabilidad'
  AND roles.name IN ('contabilidad.senior', 'contabilidad.analista');

INSERT INTO permissions (application_id, key, description)
SELECT id, seed.key, seed.description
FROM applications
CROSS JOIN (VALUES
    ('movimientos.registrar', 'Registrar movimientos contables'),
    ('movimientos.ver_todos', 'Ver todos los movimientos contables'),
    ('movimientos.aprobar', 'Aprobar o rechazar movimientos pendientes'),
    ('cierre.ejecutar', 'Ejecutar el cierre contable'),
    ('reportes.ver', 'Ver reportes y resumen contable')
) AS seed(key, description)
WHERE applications.client_id = 'contabilidad';

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles
JOIN permissions ON permissions.application_id = roles.application_id
WHERE roles.name = 'contabilidad.senior';

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles
JOIN permissions ON permissions.application_id = roles.application_id
WHERE roles.name = 'contabilidad.analista'
  AND permissions.key IN ('movimientos.registrar', 'reportes.ver');

-- Guards system roles (control 1 of ADR 0013) below the API: they cannot be changed or deleted,
-- no new system role can be created and an application role cannot be promoted to one. A BEFORE
-- UPDATE trigger must return NEW, otherwise PostgreSQL silently drops the update.
CREATE FUNCTION reject_system_role_mutation() RETURNS trigger AS $$
BEGIN
    IF TG_OP IN ('UPDATE', 'DELETE') AND OLD.system THEN
        RAISE EXCEPTION 'system roles cannot be updated or deleted';
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') AND NEW.system THEN
        RAISE EXCEPTION 'system roles cannot be created';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER roles_reject_system_mutation
BEFORE INSERT OR UPDATE OR DELETE ON roles
FOR EACH ROW EXECUTE FUNCTION reject_system_role_mutation();

GRANT SELECT ON permissions, role_permissions TO identity_app;
GRANT INSERT, UPDATE, DELETE ON roles, role_permissions TO identity_app;

COMMIT;
