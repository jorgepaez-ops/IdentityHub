-- Restore the pre-organizations model exactly (applications and users without a company).
BEGIN;

REVOKE SELECT ON organizations FROM identity_app;

DROP TRIGGER users_reject_organization_change_with_roles ON users;
DROP FUNCTION reject_user_organization_change_with_roles();
DROP TRIGGER user_roles_enforce_organization ON user_roles;
DROP FUNCTION enforce_user_role_organization();

-- Remove the Inventario demo. user_roles and authorization_codes restrict deletion of what they reference.
DELETE FROM user_roles
WHERE role_id IN (SELECT r.id FROM roles r JOIN applications a ON a.id = r.application_id WHERE a.client_id = 'inventario');
DELETE FROM authorization_codes
WHERE application_id IN (SELECT id FROM applications WHERE client_id = 'inventario');
DELETE FROM applications WHERE client_id = 'inventario'; -- cascades to its roles, permissions and role_permissions

DROP INDEX users_organization_idx;
ALTER TABLE users DROP COLUMN organization_id;
ALTER TABLE applications DROP COLUMN organization_id;
DROP TABLE organizations;

COMMIT;
