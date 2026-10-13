-- 000010 down — removes service keys (RF-018).
BEGIN;

DROP TABLE service_keys;

COMMIT;
