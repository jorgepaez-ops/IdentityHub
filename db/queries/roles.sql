-- name: ListRolesWithApplication :many
SELECT roles.id, roles.name, roles.description, roles.application_id, roles.system,
       applications.client_id
FROM roles
LEFT JOIN applications ON applications.id = roles.application_id
ORDER BY roles.name;

-- name: ValidateRoleNames :many
SELECT name
FROM roles
WHERE name = ANY(sqlc.arg(role_names)::text[])
ORDER BY name;

-- name: ListPermissionKeysForRolesAndApplication :many
SELECT DISTINCT permissions.key
FROM role_permissions
JOIN roles ON roles.id = role_permissions.role_id
JOIN permissions ON permissions.id = role_permissions.permission_id
JOIN applications ON applications.id = roles.application_id
WHERE roles.name = ANY(sqlc.arg(role_names)::text[])
  AND applications.client_id = sqlc.arg(client_id)
  AND permissions.application_id = roles.application_id
ORDER BY permissions.key;

-- name: ListRoleGridApplications :many
SELECT id, client_id, name FROM applications ORDER BY client_id;

-- name: GetRoleGridApplication :one
SELECT id, client_id, name FROM applications WHERE id = $1;

-- name: ListApplicationPermissions :many
SELECT key, description FROM permissions WHERE application_id = $1 ORDER BY key;

-- name: ListApplicationRoles :many
SELECT r.id, r.name, r.description, r.application_id, r.system,
       COALESCE(array_agg(p.key ORDER BY p.key) FILTER (WHERE p.key IS NOT NULL), '{}')::text[] AS permission_keys,
       (SELECT count(*) FROM user_roles ur WHERE ur.role_id = r.id)::bigint AS assigned_count
FROM roles r
LEFT JOIN role_permissions rp ON rp.role_id = r.id
LEFT JOIN permissions p ON p.id = rp.permission_id
WHERE r.application_id = $1
GROUP BY r.id
ORDER BY r.name;

-- name: GetApplicationRole :one
SELECT r.id, r.name, r.description, r.application_id, r.system,
       COALESCE(array_agg(p.key ORDER BY p.key) FILTER (WHERE p.key IS NOT NULL), '{}')::text[] AS permission_keys,
       (SELECT count(*) FROM user_roles ur WHERE ur.role_id = r.id)::bigint AS assigned_count
FROM roles r
LEFT JOIN role_permissions rp ON rp.role_id = r.id
LEFT JOIN permissions p ON p.id = rp.permission_id
WHERE r.id = $1 AND r.application_id = $2
GROUP BY r.id;

-- name: ValidateApplicationPermissionKeys :many
SELECT key FROM permissions
WHERE application_id = $1 AND key = ANY(sqlc.arg(keys)::text[])
ORDER BY key;

-- name: CreateApplicationRole :one
INSERT INTO roles (name, description, application_id, system)
VALUES ($2, $3, $1, false)
RETURNING id, name, description, application_id, system;

-- name: DeleteApplicationRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: AddApplicationRolePermission :execrows
INSERT INTO role_permissions (role_id, permission_id)
SELECT $1, id FROM permissions WHERE application_id = $2 AND key = $3;

-- name: UpdateApplicationRoleDescription :exec
UPDATE roles SET description = $2 WHERE id = $1;

-- name: DeleteApplicationRole :exec
DELETE FROM roles WHERE id = $1;

-- name: ActorHoldsApplicationRole :one
SELECT EXISTS(SELECT 1 FROM user_roles WHERE user_id = $1 AND role_id = $2);
