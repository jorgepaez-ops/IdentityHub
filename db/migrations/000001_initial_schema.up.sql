-- 000001 — Esquema inicial
--
-- Implementa specs/02-domain-model.md. Cada tabla lleva el requisito que la
-- justifica; si una tabla no puede señalar un requisito, sobra.

BEGIN;

-- citext resuelve la comparación de correos sin distinguir mayúsculas dentro de
-- la base de datos. Hacerlo en código de aplicación es fácil de olvidar en una
-- consulta y ese olvido abre un registro duplicado (specs/02, notas de diseño).
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ── users ────────────────────────────────────────────────────────────────
CREATE TYPE user_status AS ENUM (
    'pending_verification',
    'active',
    'locked',
    'disabled'
);

CREATE TABLE users (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email              citext      NOT NULL UNIQUE,
    password_hash      text        NOT NULL,
    display_name       text        NOT NULL,
    status             user_status NOT NULL DEFAULT 'pending_verification',
    mfa_enabled        boolean     NOT NULL DEFAULT false,
    mfa_secret_enc     bytea,
    failed_login_count integer     NOT NULL DEFAULT 0,
    locked_until       timestamptz,
    last_login_at      timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),

    -- Invariante 1: la contraseña nunca se almacena de forma recuperable.
    -- La restricción convierte en imposible por construcción el error de
    -- guardar un hash rápido, que es justo VULN-002 de la línea base.
    CONSTRAINT users_password_hash_is_argon2id
        CHECK (password_hash LIKE '$argon2id$%'),

    -- El segundo factor no puede estar activo sin secreto guardado.
    CONSTRAINT users_mfa_requires_secret
        CHECK (NOT mfa_enabled OR mfa_secret_enc IS NOT NULL)
);

CREATE INDEX users_status_idx     ON users (status);
CREATE INDEX users_created_at_idx ON users (created_at DESC);

-- ── roles y asignación ───────────────────────────────────────────────────
CREATE TABLE roles (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL UNIQUE,
    description text NOT NULL DEFAULT ''
);

INSERT INTO roles (name, description) VALUES
    ('admin', 'Administra cuentas y consulta el registro de auditoría'),
    ('user',  'Gestiona únicamente su propia cuenta');

CREATE TABLE user_roles (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    granted_by uuid REFERENCES users (id) ON DELETE SET NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_roles_role_idx ON user_roles (role_id);

-- ── refresh_tokens ───────────────────────────────────────────────────────
-- El diseño de familias es lo que hace posible RF-006: detectar el reuso de un
-- token rotado y revocar de un golpe todos sus descendientes (ver adr/0005).
CREATE TYPE refresh_status AS ENUM ('active', 'rotated', 'revoked');

CREATE TABLE refresh_tokens (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,          -- SHA-256; nunca el token en claro
    family_id    uuid  NOT NULL,
    parent_id    uuid  REFERENCES refresh_tokens (id) ON DELETE SET NULL,
    status       refresh_status NOT NULL DEFAULT 'active',
    ip           inet,
    user_agent   text,
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,

    CONSTRAINT refresh_tokens_hash_is_sha256 CHECK (octet_length(token_hash) = 32)
);

CREATE INDEX refresh_tokens_user_idx   ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_expiry_idx ON refresh_tokens (expires_at)
    WHERE status = 'active';

-- Invariante 3: como máximo un token activo por familia.
CREATE UNIQUE INDEX refresh_tokens_one_active_per_family
    ON refresh_tokens (family_id)
    WHERE status = 'active';

-- ── verification_tokens ──────────────────────────────────────────────────
CREATE TYPE token_purpose AS ENUM ('email_verification', 'password_reset');

CREATE TABLE verification_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    purpose    token_purpose NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT verification_tokens_hash_is_sha256 CHECK (octet_length(token_hash) = 32)
);

CREATE INDEX verification_tokens_user_idx ON verification_tokens (user_id, purpose);

-- ── recovery_codes ───────────────────────────────────────────────────────
CREATE TABLE recovery_codes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  text NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX recovery_codes_user_idx ON recovery_codes (user_id) WHERE used_at IS NULL;

-- ── audit_log ────────────────────────────────────────────────────────────
-- bigint secuencial en vez de uuid: deja explícito el orden de inserción y
-- abarata la consulta por rango temporal (specs/02, notas de diseño).
CREATE TABLE audit_log (
    id            bigserial PRIMARY KEY,
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    action        text NOT NULL,
    resource_type text,
    resource_id   text,
    ip            inet,
    user_agent    text,
    metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_actor_idx   ON audit_log (actor_user_id, created_at DESC);
CREATE INDEX audit_log_action_idx  ON audit_log (action, created_at DESC);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

-- Invariante 5: el registro de auditoría es append-only (RF-011, AM-010).
--
-- Se implementa con un disparador y no solo con permisos porque un disparador
-- se aplica a TODO el mundo, incluido el superusuario y quien se conecte por
-- error con credenciales de más. En la semana 2, cuando exista el rol
-- `identity_app` con permisos mínimos, se añade encima la revocación de
-- UPDATE y DELETE: defensa en profundidad, no una en lugar de la otra.
CREATE OR REPLACE FUNCTION audit_log_is_append_only()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION
        'audit_log es append-only: % no está permitido (invariante 5, RF-011)',
        TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$;

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_is_append_only();

CREATE TRIGGER audit_log_no_delete
    BEFORE DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_is_append_only();

-- ── updated_at automático ────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION touch_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER users_touch_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMIT;
