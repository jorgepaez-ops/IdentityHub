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
