-- name: InsertServiceKey :one
INSERT INTO service_keys (public_id, secret_hash, name, scopes, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, public_id, name, scopes, created_by, created_at, expires_at, last_used_at, revoked_at;

-- name: ListServiceKeys :many
-- Metadata only: the hash is deliberately not selected.
SELECT id, public_id, name, scopes, created_by, created_at, expires_at, last_used_at, revoked_at
FROM service_keys
ORDER BY created_at DESC, id DESC;

-- name: GetServiceKeyByPublicID :one
SELECT id, public_id, secret_hash, name, scopes, created_by, created_at, expires_at, last_used_at, revoked_at
FROM service_keys
WHERE public_id = $1;

-- name: GetServiceKeyForUpdate :one
SELECT id, revoked_at
FROM service_keys
WHERE id = $1
FOR UPDATE;

-- name: RevokeServiceKey :execrows
UPDATE service_keys
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: TouchServiceKeyLastUsed :exec
-- Throttled: at most one write per key per minute.
UPDATE service_keys
SET last_used_at = sqlc.arg(used_at)::timestamptz
WHERE id = sqlc.arg(id)
  AND (last_used_at IS NULL OR last_used_at < sqlc.arg(used_at)::timestamptz - interval '1 minute');
