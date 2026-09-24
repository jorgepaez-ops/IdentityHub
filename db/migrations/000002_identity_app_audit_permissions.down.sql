-- Revert 000002 without removing the append-only trigger created in 000001.
BEGIN;

REVOKE USAGE ON SCHEMA public FROM identity_app;
REVOKE ALL PRIVILEGES ON users, roles, user_roles, refresh_tokens, verification_tokens, recovery_codes, audit_log FROM identity_app;
REVOKE ALL PRIVILEGES ON SEQUENCE audit_log_id_seq FROM identity_app;
DROP ROLE IF EXISTS identity_app;

COMMIT;
