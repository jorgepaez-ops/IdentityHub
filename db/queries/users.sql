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

-- name: CreateInvitationToken :exec
INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at)
VALUES ($1, $2, 'invitation', $3);

-- name: InvitationTokenIsUsable :one
-- This cheap read prevents password hashing for invalid public invitation
-- tokens. ConsumeInvitationToken remains the atomic source of truth.
SELECT EXISTS (
    SELECT 1
    FROM verification_tokens
    WHERE token_hash = $1
      AND purpose = 'invitation'
      AND used_at IS NULL
      AND expires_at > now()
) AS token_is_usable;

-- name: ConsumeInvitationToken :one
-- Consumes a one-time invitation token and, in the same statement, sets the
-- password the invitee just chose and activates the account (T5, RF-002).
-- Shares the verification_tokens table with other token purposes but never
-- accepts a token whose purpose is not invitation (invariant 7).
WITH consumed AS (
    UPDATE verification_tokens
    SET used_at = now()
    WHERE token_hash = $1
      AND purpose = 'invitation'
      AND used_at IS NULL
      AND expires_at > now()
    RETURNING user_id
)
UPDATE users
SET password_hash = $2, status = 'active', updated_at = now()
WHERE id = (SELECT user_id FROM consumed)
  AND status = 'pending_verification'
RETURNING *;

-- name: GetLoginUserByEmail :one
SELECT id, email, display_name, password_hash, status, locked_until, mfa_enabled
FROM users
WHERE email = $1
FOR UPDATE;

-- name: LockLoginUser :exec
UPDATE users
SET status = 'locked', locked_until = $2
WHERE id = $1 AND status = 'active';

-- name: UnlockLoginUser :exec
UPDATE users
SET status = 'active', locked_until = NULL
WHERE id = $1 AND status = 'locked';

-- name: ListRolesForUser :many
SELECT roles.name
FROM user_roles
JOIN roles ON roles.id = user_roles.role_id
WHERE user_roles.user_id = $1
ORDER BY roles.name;

-- name: UpdatePasswordHash :exec
-- Transparent Argon2id rehash after a verified password (RF-003).
UPDATE users
SET password_hash = $2
WHERE id = $1;

-- name: UpdateLastLogin :exec
-- Recorded only when the MFA code is accepted (D11), not at password check.
UPDATE users
SET last_login_at = now()
WHERE id = $1;

-- name: LockMfaChallengeIssuance :exec
-- Serializes concurrent issuance for one account (transaction-scoped advisory
-- lock, so it never contends with the row locks Verify takes).
SELECT pg_advisory_xact_lock(hashtextextended($1::uuid::text, 0));

-- name: CountMfaChallengesSince :one
SELECT count(*) FROM mfa_challenges WHERE user_id = $1 AND created_at >= $2;

-- name: SupersedeOpenMfaChallenges :exec
-- A new challenge replaces the account's open ones with the service clock,
-- keeping deterministic tests and all MFA timestamps on one source of time.
UPDATE mfa_challenges SET used_at = $2
WHERE user_id = $1 AND used_at IS NULL;

-- name: CreateMfaChallenge :exec
-- $7 is the service clock, stored as both created_at and last_sent_at so the
-- issuance window and the resend window use the same time source as the code.
INSERT INTO mfa_challenges (id, user_id, token_hash, code_hash, expires_at, attempts_left, created_at, last_sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7);

-- name: DeleteMfaChallenge :exec
-- An initial MFA code that could not be published must not consume the
-- issuance window. The audit preserves the delivery attempt for operators.
DELETE FROM mfa_challenges WHERE id = $1;

-- name: GetMfaChallengeForUpdate :one
SELECT c.id, c.user_id, c.code_hash, c.expires_at, c.attempts_left, c.last_sent_at, c.used_at,
       u.email, u.display_name, u.status
FROM mfa_challenges c
JOIN users u ON u.id = c.user_id
WHERE c.token_hash = $1
FOR UPDATE OF c, u;

-- name: ConsumeMfaChallenge :exec
UPDATE mfa_challenges SET used_at = now()
WHERE id = $1 AND used_at IS NULL;

-- name: RejectMfaChallenge :one
UPDATE mfa_challenges
SET attempts_left = attempts_left - 1,
    used_at = CASE WHEN attempts_left = 1 THEN now() ELSE used_at END
WHERE id = $1 AND used_at IS NULL AND attempts_left > 0
RETURNING attempts_left;

-- name: ResendMfaChallenge :exec
UPDATE mfa_challenges
SET code_hash = $2, last_sent_at = $3
WHERE id = $1 AND used_at IS NULL;

-- name: RestoreMfaChallengeAfterFailedResend :execrows
-- Publish happens after commit (D16). If it fails, restore this exact change
-- so last_sent_at does not impose a retry delay. The expected new values avoid
-- overwriting a later successful resend.
UPDATE mfa_challenges
SET code_hash = $3, last_sent_at = $5
WHERE id = $1 AND code_hash = $2 AND last_sent_at = $4 AND used_at IS NULL;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, family_id, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CreateHubSession :exec
INSERT INTO hub_sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3);

-- name: GetHubSessionUser :one
SELECT s.user_id
FROM hub_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.status = 'active';

-- name: RevokeHubSessionsForUser :exec
UPDATE hub_sessions SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: GetUsedAuthorizationCodeOwner :one
SELECT user_id FROM authorization_codes
WHERE code_hash = $1 AND used_at IS NOT NULL;

-- name: CreateAuthorizationCode :execrows
INSERT INTO authorization_codes (id, user_id, application_id, code_hash, redirect_uri, code_challenge, expires_at)
SELECT $1, $2, id, $3, $4, $5, $6
FROM applications
WHERE client_id = $7 AND redirect_uri = $4;

-- name: GetAuthorizationCodeForUpdate :one
SELECT c.id, c.user_id, a.client_id, c.redirect_uri, c.code_challenge, c.expires_at
FROM authorization_codes c
JOIN applications a ON a.id = c.application_id
WHERE c.code_hash = $1 AND c.used_at IS NULL AND c.expires_at > $2
FOR UPDATE OF c;

-- name: MarkAuthorizationCodeUsed :execrows
UPDATE authorization_codes SET used_at = $2
WHERE id = $1 AND used_at IS NULL;

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


-- name: ListActiveRefreshSessions :many
-- A session is a refresh-token family. The single active token supplies the
-- latest device metadata; the family history supplies its first creation and
-- last use timestamps.
SELECT active.family_id, active.ip, active.user_agent,
       min(history.created_at)::timestamptz AS created_at,
       max(COALESCE(history.last_used_at, history.created_at))::timestamptz AS last_used_at
FROM refresh_tokens AS active
JOIN refresh_tokens AS history ON history.family_id = active.family_id
WHERE active.user_id = $1
  AND active.status = 'active'
  AND active.expires_at > now()
GROUP BY active.family_id, active.ip, active.user_agent
ORDER BY max(COALESCE(history.last_used_at, history.created_at)) DESC;

-- name: RevokeActiveRefreshSession :one
-- Ownership and activeness are both predicates: foreign, expired and already
-- revoked families all return false to the HTTP layer as the same 404.
WITH revoked AS (
    UPDATE refresh_tokens
    SET status = 'revoked'
    WHERE user_id = $1
      AND family_id = $2
      AND status = 'active'
      AND expires_at > now()
    RETURNING 1
)
SELECT EXISTS (SELECT 1 FROM revoked) AS revoked;

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

-- name: CreatePasswordResetTokenForEmail :one
-- The same INSERT ... SELECT statement runs for every request. It stores a
-- hash-only one-hour token only when the normalized email has a user row;
-- callers pass the boolean only to the broker event, never to HTTP responses.
WITH inserted AS (
    INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at)
    SELECT id, $2, 'password_reset', $3
    FROM users
    WHERE email = $1
    RETURNING user_id
)
SELECT EXISTS (SELECT 1 FROM inserted) AS token_created;

-- name: PasswordResetTokenIsUsable :one
-- Cheap precheck before Argon2id. The consuming statement below is authoritative.
SELECT EXISTS (
    SELECT 1
    FROM verification_tokens
    WHERE token_hash = $1
      AND purpose = 'password_reset'
      AND used_at IS NULL
      AND expires_at > now()
) AS token_is_usable;

-- name: ConsumePasswordResetTokenAndRevokeSessions :one
-- The password update, one-time token consumption, and all-session revocation
-- share one statement so no transaction can expose the new password with an
-- old active refresh token (RF-015 / AM-005). D13: a locked account (RF-017)
-- is reactivated in the same statement, because controlling the mailbox is a
-- different factor than the password being guessed, so unlocking here adds
-- no exposure; disabled and pending_verification accounts are left as-is.
-- `unlocked` reports whether this reset just cleared a lockout, for the
-- caller's audit metadata.
WITH consumed AS (
    UPDATE verification_tokens
    SET used_at = now()
    WHERE verification_tokens.token_hash = $1
      AND verification_tokens.purpose = 'password_reset'
      AND verification_tokens.used_at IS NULL
      AND verification_tokens.expires_at > now()
    RETURNING verification_tokens.id, verification_tokens.user_id
), invalidated_reset_tokens AS (
    UPDATE verification_tokens
    SET used_at = now()
    WHERE user_id = (SELECT user_id FROM consumed)
      AND purpose = 'password_reset'
      AND used_at IS NULL
      AND id <> (SELECT id FROM consumed)
    RETURNING id
), previous_user AS (
    SELECT id, status FROM users WHERE id = (SELECT user_id FROM consumed) FOR UPDATE
), updated_user AS (
    UPDATE users
    SET password_hash = $2,
        updated_at = now(),
        status = CASE WHEN status = 'locked' THEN 'active' ELSE status END,
        locked_until = CASE WHEN status = 'locked' THEN NULL ELSE locked_until END
    WHERE id = (SELECT id FROM previous_user)
    RETURNING *
), revoked AS (
    UPDATE refresh_tokens
    SET status = 'revoked'
    WHERE user_id = (SELECT id FROM updated_user)
      AND status <> 'revoked'
    RETURNING id
), revoked_hub_sessions AS (
    UPDATE hub_sessions
    SET revoked_at = now()
    WHERE user_id = (SELECT id FROM updated_user)
      AND revoked_at IS NULL
    RETURNING id
)
SELECT updated_user.*, (previous_user.status = 'locked') AS unlocked
FROM updated_user
JOIN previous_user ON previous_user.id = updated_user.id;

-- name: InvalidateInvitationTokens :exec
UPDATE verification_tokens
SET used_at = now()
WHERE user_id = $1
  AND purpose = 'invitation'
  AND used_at IS NULL;
