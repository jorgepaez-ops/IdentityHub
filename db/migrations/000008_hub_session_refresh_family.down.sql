DROP INDEX IF EXISTS hub_sessions_active_family_idx;

ALTER TABLE hub_sessions
    DROP COLUMN family_id;
