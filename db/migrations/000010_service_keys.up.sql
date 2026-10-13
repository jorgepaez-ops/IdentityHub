-- 000010 — Service keys for machine-to-machine access (RF-018, ADR 0014).
--
-- Only the SHA-256 of the secret is stored: the secret has 256 bits of entropy, so a fast
-- hash is enough, and the database alone never reveals a usable key. public_id locates the row.
BEGIN;

CREATE TABLE service_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    text NOT NULL UNIQUE CHECK (public_id ~ '^[0-9a-f]{16}$'),
    secret_hash  bytea NOT NULL CHECK (octet_length(secret_hash) = 32),
    name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    scopes       text[] NOT NULL CHECK (cardinality(scopes) >= 1),
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at   timestamptz,
    CONSTRAINT service_keys_expires_after_creation CHECK (expires_at > created_at)
);

CREATE INDEX service_keys_created_idx ON service_keys (created_at DESC);

COMMENT ON COLUMN service_keys.secret_hash IS
    'SHA-256 of the secret part of ihk_<public_id>_<secret>; the secret itself is never stored.';

GRANT SELECT, INSERT, UPDATE ON service_keys TO identity_app;

COMMIT;
