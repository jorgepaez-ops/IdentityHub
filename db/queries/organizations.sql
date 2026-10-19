-- name: ListOrganizations :many
SELECT id, slug, name, created_at
FROM organizations
ORDER BY name ASC, id ASC;

-- name: GetUserOrganization :one
SELECT o.id, o.slug, o.name, o.created_at
FROM users u
JOIN organizations o ON o.id = u.organization_id
WHERE u.id = $1;

-- name: ListOrganizationApplications :many
SELECT id, client_id, name, organization_id
FROM applications
WHERE organization_id = $1
ORDER BY client_id ASC;

-- name: ListOrganizationUsers :many
-- Same filters and keyset paging as ListAdminUsers, restricted to one organization (RF-022).
SELECT *
FROM users
WHERE organization_id = sqlc.arg(organization_id)::uuid
  AND (sqlc.arg(query)::text = ''
       OR email::text ILIKE '%' || sqlc.arg(query)::text || '%'
       OR display_name ILIKE '%' || sqlc.arg(query)::text || '%')
  AND (sqlc.narg(status)::user_status IS NULL OR status = sqlc.narg(status)::user_status)
  AND (sqlc.narg(cursor)::uuid IS NULL OR id > sqlc.narg(cursor)::uuid)
ORDER BY id ASC
LIMIT sqlc.arg(limit_count)::bigint;

-- name: ListUserApplications :many
-- Applications of the user's own company where the user holds at least one application role (RF-022).
SELECT a.client_id, a.name, a.allowed_origin
FROM applications a
JOIN users u ON u.organization_id = a.organization_id
WHERE u.id = $1
  AND EXISTS (
    SELECT 1
    FROM user_roles ur
    JOIN roles r ON r.id = ur.role_id
    WHERE ur.user_id = u.id AND r.application_id = a.id
  )
ORDER BY a.name ASC, a.client_id ASC;
