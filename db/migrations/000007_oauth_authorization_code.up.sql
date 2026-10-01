CREATE TABLE applications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id text NOT NULL UNIQUE,
    redirect_uri text NOT NULL,
    allowed_origin text NOT NULL,
    name text NOT NULL
);

INSERT INTO applications (client_id, redirect_uri, allowed_origin, name)
VALUES ('contabilidad', 'http://contabilidad.localhost:8080/oauth/callback', 'http://contabilidad.localhost:8080', 'Contabilidad');

CREATE TABLE hub_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX hub_sessions_active_user_idx ON hub_sessions (user_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE authorization_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE RESTRICT,
    code_hash bytea NOT NULL UNIQUE CHECK (octet_length(code_hash) = 32),
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);
CREATE INDEX authorization_codes_active_idx ON authorization_codes (expires_at) WHERE used_at IS NULL;

GRANT SELECT ON applications TO identity_app;
GRANT SELECT, INSERT, UPDATE ON hub_sessions, authorization_codes TO identity_app;
