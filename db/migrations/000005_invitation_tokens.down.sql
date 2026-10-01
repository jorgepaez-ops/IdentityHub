-- PostgreSQL has no ALTER TYPE ... DROP VALUE, so removing the enum value
-- means rebuilding token_purpose without it. Any verification_tokens row
-- already using 'invitation' would make the column USING cast below fail,
-- so those rows are discarded first — rolling back this migration means
-- discarding the invitation-purpose feature, same as T4's down migration
-- discarding the Contabilidad role grants it introduced.
BEGIN;

DELETE FROM verification_tokens WHERE purpose = 'invitation';

CREATE TYPE token_purpose_pre_invitation AS ENUM ('email_verification', 'password_reset');

ALTER TABLE verification_tokens
    ALTER COLUMN purpose TYPE token_purpose_pre_invitation
    USING purpose::text::token_purpose_pre_invitation;

DROP TYPE token_purpose;

ALTER TYPE token_purpose_pre_invitation RENAME TO token_purpose;

COMMIT;
