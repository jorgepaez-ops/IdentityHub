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
