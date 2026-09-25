-- name: InsertAuditEvent :one
INSERT INTO audit_log (
    actor_user_id,
    action,
    resource_type,
    resource_id,
    ip,
    user_agent,
    metadata
)
VALUES (
    sqlc.narg(actor_user_id),
    sqlc.arg(action),
    sqlc.narg(resource_type),
    sqlc.narg(resource_id),
    sqlc.narg(ip),
    sqlc.narg(user_agent),
    sqlc.arg(metadata)::jsonb
)
RETURNING *;

-- name: ListAuditLog :many
SELECT id, actor_user_id, action, resource_type, resource_id, ip, user_agent, metadata, created_at
FROM audit_log
WHERE (sqlc.narg(action)::text IS NULL OR action = sqlc.narg(action)::text)
  AND (sqlc.narg(actor_user_id)::uuid IS NULL OR actor_user_id = sqlc.narg(actor_user_id)::uuid)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(cursor_id)::bigint IS NULL OR id < sqlc.narg(cursor_id)::bigint)
ORDER BY id DESC
LIMIT sqlc.arg(limit_count);
