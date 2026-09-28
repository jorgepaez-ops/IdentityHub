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

-- name: CountLoginFailuresByAccount :one
-- D13: failures recorded before the account's latest completed password
-- reset no longer count, so a single wrong attempt right after a reset does
-- not immediately re-lock the account. The IP-based sibling query below is
-- unaffected on purpose: RF-017 keeps limiting guessing from one IP
-- regardless of which account last reset its password.
SELECT count(*)::bigint
FROM audit_log AS failures
WHERE failures.actor_user_id = $1
  AND failures.action = 'login_failed'
  AND failures.created_at >= $2
  AND failures.created_at > COALESCE(
        (SELECT resets.created_at
         FROM audit_log AS resets
         WHERE resets.actor_user_id = $1
           AND resets.action = 'password_reset_completed'
         ORDER BY resets.created_at DESC
         LIMIT 1),
        '-infinity'::timestamptz
      );

-- name: CountLoginFailuresByIP :one
SELECT count(*)::bigint
FROM audit_log
WHERE ip = $1
  AND action = 'login_failed'
  AND created_at >= $2;

-- name: ListAuditLog :many
SELECT id, actor_user_id, action, resource_type, resource_id, ip, user_agent, metadata, created_at
FROM audit_log
WHERE (sqlc.narg(action)::text IS NULL OR action = sqlc.narg(action)::text)
  AND (sqlc.narg(actor_user_id)::uuid IS NULL OR actor_user_id = sqlc.narg(actor_user_id)::uuid)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(cursor_id)::bigint IS NULL OR id < sqlc.narg(cursor_id)::bigint)
ORDER BY id DESC
LIMIT sqlc.arg(limit_count);
