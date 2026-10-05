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
