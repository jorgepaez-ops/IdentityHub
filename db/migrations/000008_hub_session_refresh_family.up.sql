ALTER TABLE hub_sessions
    ADD COLUMN family_id uuid;

CREATE INDEX hub_sessions_active_family_idx
    ON hub_sessions (family_id, expires_at)
    WHERE revoked_at IS NULL AND family_id IS NOT NULL;
