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
    SELECT id, user_id, family_id, status, expires_at
    FROM refresh_tokens
    WHERE refresh_tokens.token_hash = $1
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
