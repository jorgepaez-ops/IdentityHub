-- Revierte 000001. El orden importa: primero lo que depende, luego lo dependido.

BEGIN;

DROP TRIGGER IF EXISTS users_touch_updated_at ON users;
DROP TRIGGER IF EXISTS audit_log_no_delete    ON audit_log;
DROP TRIGGER IF EXISTS audit_log_no_update    ON audit_log;

DROP FUNCTION IF EXISTS touch_updated_at();
DROP FUNCTION IF EXISTS audit_log_is_append_only();

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS recovery_codes;
DROP TABLE IF EXISTS verification_tokens;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS token_purpose;
DROP TYPE IF EXISTS refresh_status;
DROP TYPE IF EXISTS user_status;

COMMIT;
