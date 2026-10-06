-- Restore the pre-T12 role model exactly.
BEGIN;

REVOKE INSERT, UPDATE, DELETE ON roles, role_permissions FROM identity_app;
REVOKE SELECT ON permissions, role_permissions FROM identity_app;

DROP TRIGGER roles_reject_system_mutation ON roles;
DROP FUNCTION reject_system_role_mutation();

DROP TABLE role_permissions;
DROP TABLE permissions;

ALTER TABLE roles
    DROP COLUMN application_id,
    DROP COLUMN system;

COMMIT;
