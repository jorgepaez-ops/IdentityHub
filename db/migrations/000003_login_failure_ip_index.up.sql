BEGIN;

CREATE INDEX audit_log_ip_created_idx ON audit_log (ip, created_at DESC);

COMMIT;
