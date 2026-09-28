CREATE TABLE mfa_challenges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    code_hash bytea NOT NULL CHECK (octet_length(code_hash) = 32),
    expires_at timestamptz NOT NULL,
    attempts_left integer NOT NULL CHECK (attempts_left BETWEEN 0 AND 5),
    last_sent_at timestamptz NOT NULL DEFAULT now(),
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX mfa_challenges_active_user_idx ON mfa_challenges (user_id, expires_at)
WHERE used_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON mfa_challenges TO identity_app;
