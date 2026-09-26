#!/usr/bin/env bash
# Ejecutado automáticamente por la imagen oficial de Postgres, una sola vez,
# cuando /var/lib/postgresql/data está vacío (primer arranque del volumen).
# Da a identity_app (creado NOLOGIN por la migración 000002, T9) login y
# contraseña desde IDENTITY_APP_PASSWORD, sin escribir el secreto en ningún
# archivo versionado. Si el volumen ya existe de antes de T26, hay que
# recrearlo una vez (docker compose down -v) para que este script corra.
set -euo pipefail

if [ -z "${IDENTITY_APP_PASSWORD:-}" ]; then
    echo "01-identity-app-role.sh: IDENTITY_APP_PASSWORD no está definida, abortando" >&2
    exit 1
fi

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'identity_app') THEN
            CREATE ROLE identity_app LOGIN PASSWORD '${IDENTITY_APP_PASSWORD}';
        ELSE
            ALTER ROLE identity_app WITH LOGIN PASSWORD '${IDENTITY_APP_PASSWORD}';
        END IF;
    END
    \$\$;
EOSQL
