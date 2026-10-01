BEGIN;

-- user_roles.role_id REFERENCES roles ON DELETE RESTRICT: an account that
-- already holds a Contabilidad role would block the DELETE below. Revoking
-- the grant is what "roll back this migration" means here, so it happens in
-- the same transaction as removing the role from the catalog.
DELETE FROM user_roles
WHERE role_id IN (SELECT id FROM roles WHERE name IN ('contabilidad.senior', 'contabilidad.analista'));

DELETE FROM roles WHERE name IN ('contabilidad.senior', 'contabilidad.analista');

COMMIT;
