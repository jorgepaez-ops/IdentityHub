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
