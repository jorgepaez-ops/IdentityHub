-- 000002 — Application role and append-only audit-log permissions (RF-011).
BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'identity_app') THEN
        CREATE ROLE identity_app NOLOGIN;
    END IF;
END;
$$;

GRANT USAGE ON SCHEMA public TO identity_app;
GRANT SELECT, INSERT, UPDATE ON users TO identity_app;
GRANT SELECT ON roles TO identity_app;
GRANT SELECT, INSERT, DELETE ON user_roles TO identity_app;
GRANT SELECT, INSERT, UPDATE ON refresh_tokens TO identity_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON verification_tokens TO identity_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON recovery_codes TO identity_app;
GRANT SELECT, INSERT ON audit_log TO identity_app;
GRANT USAGE, SELECT ON SEQUENCE audit_log_id_seq TO identity_app;

-- Defense in depth: the trigger from 000001 protects every role; these revocations
-- make the application role incapable of changing or removing audit history.
REVOKE UPDATE, DELETE ON audit_log FROM identity_app;

COMMIT;
