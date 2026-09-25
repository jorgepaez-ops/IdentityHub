-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: ListAdminUsers :many
SELECT *
FROM users
WHERE (sqlc.arg(query)::text = ''
       OR email::text ILIKE '%' || sqlc.arg(query)::text || '%'
       OR display_name ILIKE '%' || sqlc.arg(query)::text || '%')
  AND (sqlc.narg(status)::user_status IS NULL OR status = sqlc.narg(status)::user_status)
  AND (sqlc.narg(cursor)::uuid IS NULL OR id > sqlc.narg(cursor)::uuid)
ORDER BY id
LIMIT sqlc.arg(limit_count)::bigint;

-- name: GetUserByIDForUpdate :one
SELECT *
FROM users
WHERE id = $1
FOR UPDATE;

-- name: LockActiveAdminUsers :many
SELECT u.id
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE u.status = 'active' AND r.name = 'admin'
ORDER BY u.id
FOR UPDATE OF u;

-- name: UpdateAdminUserStatus :one
UPDATE users
SET status = $2
WHERE id = $1
RETURNING *;

-- name: DeleteUserRoles :exec
DELETE FROM user_roles
WHERE user_id = $1;

-- name: AddUserRole :exec
INSERT INTO user_roles (user_id, role_id, granted_by)
SELECT $1, id, $3
FROM roles
WHERE name = $2;

-- name: CreateVerificationToken :exec
INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at)
VALUES ($1, $2, 'email_verification', $3);

-- name: ConsumeEmailVerificationToken :one
WITH consumed AS (
    UPDATE verification_tokens
    SET used_at = now()
    WHERE token_hash = $1
      AND purpose = 'email_verification'
      AND used_at IS NULL
      AND expires_at > now()
    RETURNING user_id
)
UPDATE users
SET status = 'active', updated_at = now()
WHERE id = (SELECT user_id FROM consumed)
  AND status = 'pending_verification'
RETURNING *;

-- name: GetLoginUserByEmail :one
SELECT id, email, password_hash, status, mfa_enabled
FROM users
WHERE email = $1;

-- name: ListRolesForUser :many
SELECT roles.name
FROM user_roles
JOIN roles ON roles.id = user_roles.role_id
WHERE user_roles.user_id = $1
ORDER BY roles.name;

-- name: UpdateLoginSuccess :exec
UPDATE users
SET password_hash = $2,
    last_login_at = now()
WHERE id = $1;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, family_id, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: RotateRefreshToken :one
WITH candidate AS (
    SELECT refresh_tokens.id, refresh_tokens.user_id, refresh_tokens.family_id, refresh_tokens.status, refresh_tokens.expires_at
    FROM refresh_tokens
    JOIN users ON users.id = refresh_tokens.user_id
    WHERE refresh_tokens.token_hash = $1
      AND users.status = 'active'
    FOR UPDATE
), rotated AS (
    UPDATE refresh_tokens
    SET status = 'rotated', last_used_at = now()
    WHERE id = (SELECT id FROM candidate)
      AND status = 'active'
      AND expires_at > now()
    RETURNING id
), created AS (
    INSERT INTO refresh_tokens (user_id, token_hash, family_id, parent_id, ip, user_agent, expires_at)
    SELECT candidate.user_id, $2, candidate.family_id, candidate.id, $3, $4, $5
    FROM candidate
    JOIN rotated ON true
    RETURNING id
)
SELECT candidate.user_id, candidate.family_id, candidate.id AS parent_id,
       candidate.status, EXISTS (SELECT 1 FROM created) AS rotated
FROM candidate;

-- name: RevokeRefreshFamily :one
WITH revoked AS (
    UPDATE refresh_tokens
    SET status = 'revoked'
    WHERE family_id = $1
      AND status <> 'revoked'
    RETURNING 1
)
SELECT count(*)::bigint FROM revoked;

-- name: RevokeRefreshToken :one
UPDATE refresh_tokens
SET status = 'revoked', last_used_at = now()
WHERE token_hash = $1
  AND status = 'active'
  AND expires_at > now()
RETURNING user_id;

-- name: UpdateDisplayName :one
UPDATE users
SET display_name = $2
WHERE id = $1
RETURNING *;
