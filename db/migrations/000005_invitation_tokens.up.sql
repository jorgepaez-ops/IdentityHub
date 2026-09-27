-- 000005 — Invitation token purpose (T5, RF-001/RF-002, D6/D9).
--
-- Reuses the existing verification_tokens table/token_purpose enum
-- (specs/02-domain-model.md "account_tokens": a single one-time,
-- hash-only, expiring token store shared by every account-token purpose)
-- instead of creating a new table. 'password_reset' already exists for
-- T6 (RF-015) to reuse independently; this migration only adds the value
-- 'invitation' that admin-created employees consume to set their password.
--
-- ALTER TYPE ... ADD VALUE cannot be used by the same transaction that adds
-- it, so this migration only adds the enum value; no other statement in
-- this file references 'invitation'.
BEGIN;

ALTER TYPE token_purpose ADD VALUE 'invitation';

COMMIT;
