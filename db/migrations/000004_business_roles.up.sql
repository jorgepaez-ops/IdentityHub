-- 000004 — Business (application) roles for Contabilidad (D8, RF-009).
--
-- Adds the Contabilidad application roles to the existing role catalog
-- (specs/02-domain-model.md). The "contabilidad." prefix is the application
-- boundary the Go role catalog (internal/auth/roles) filters on when it
-- decides which roles an application-scoped access token may carry; no
-- schema change is needed since roles.name is already free text.
BEGIN;

INSERT INTO roles (name, description) VALUES
    ('contabilidad.senior',   'Contabilidad: todas las operaciones, incluidos aprobar y cerrar el mes'),
    ('contabilidad.analista', 'Contabilidad: solo sus propias transacciones, sin cierre del mes');

COMMIT;
