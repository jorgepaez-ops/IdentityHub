#!/usr/bin/env bash
# RNF-010: prueba de carga de la latencia de autenticación con k6 (imagen fijada por digest).
#
# Usa el stack ya levantado (make up o make up-app): no arranca ni derriba nada. Siembra una cuenta
# desechable con invitación pendiente (el mismo recurso que usan las pruebas E2E) y k6 hace el login
# completo una sola vez; las mediciones no incluyen Argon2id. Sale con código distinto de cero si
# se supera algún umbral de tests/load/auth-latency.js.
#
# Al terminar (pase o falle) la cuenta sembrada queda deshabilitada (status 'disabled'), como hace el
# teardown de E2E con e2e-*: no se borra porque audit_log es append-only y sus filas referencian a
# users con ON DELETE SET NULL, que es un UPDATE y el disparador lo rechaza. Se niega a correr contra
# la producción simulada (docker-compose.prod.yml): siembra una cuenta con SQL directo. La guarda mira
# COMPOSE y COMPOSE_FILE por nombre de archivo; no puede comprobar que el contenedor db sea el de
# desarrollo porque la producción simulada comparte el nombre de proyecto (identity-hub) y el servicio db.
#
# Señales: INT y TERM salen con 130 y 143 (nunca con 0 aunque k6 ya hubiera terminado) y la limpieza
# corre en EXIT conservando el código real de salida.
#
# Uso: COMPOSE="docker compose --env-file .env -f deploy/docker-compose.yml" scripts/load-test.sh
set -euo pipefail

: "${COMPOSE:?COMPOSE es obligatorio}"
case "$COMPOSE ${COMPOSE_FILE:-}" in
  *docker-compose.prod.yml*) echo "ERROR: make load-test no corre contra la producción simulada (siembra una cuenta con SQL directo)." >&2; exit 2 ;;
esac
cd "$(dirname "$0")/.."

# grafana/k6 2.3.0; digest del índice multi-arquitectura (docker pull + docker inspect, 2026-10-13).
K6_IMAGE="${K6_IMAGE:-grafana/k6@sha256:e66db15b860113878fa74670e31f5e274830b7b6e42c8bff28b2f2d86a257603}"
db_user="${E2E_DB_USER:-identity}"
db_name="${E2E_DB_NAME:-identity}"

# Cuenta, contraseña y token de invitación aleatorios por ejecución; ningún secreto vive en el repositorio.
read -r email password token token_hash < <(python3 - <<'PY'
import base64, hashlib, secrets, time
raw = secrets.token_bytes(32)
print(f"load-{int(time.time())}-{secrets.token_hex(3)}@example.test",
      "load-" + secrets.token_urlsafe(18),
      base64.urlsafe_b64encode(raw).rstrip(b"=").decode(),
      hashlib.sha256(raw).hexdigest())
PY
)

# La cuenta se deshabilita al salir aunque la siembra o k6 fallen (si no llegó a crearse, no toca nada).
cleanup() {
  status=$?
  trap - EXIT
  # shellcheck disable=SC2086 # COMPOSE es un comando con argumentos, se parte a propósito.
  if ! $COMPOSE exec -T db psql -U "$db_user" -d "$db_name" -X -q -v ON_ERROR_STOP=1 -v "email=$email" >/dev/null \
    <<<"UPDATE users SET status = 'disabled' WHERE email = :'email' AND status <> 'disabled';"; then
    echo "AVISO: no se pudo deshabilitar la cuenta de carga $email." >&2
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# shellcheck disable=SC2086
$COMPOSE exec -T db psql -U "$db_user" -d "$db_name" -X -q -v ON_ERROR_STOP=1 \
  -v "email=$email" -v "hash=$token_hash" >/dev/null <<'SQL'
WITH u AS (
  INSERT INTO users (email, password_hash, display_name, organization_id)
  VALUES (:'email', '$argon2id$load-placeholder', 'Load Test',
          (SELECT id FROM organizations WHERE slug = 'contabilidad-sa')) RETURNING id
), r AS (
  INSERT INTO user_roles (user_id, role_id)
  SELECT u.id, roles.id FROM u, roles WHERE roles.name = 'user' RETURNING 1
)
INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at)
SELECT id, decode(:'hash', 'hex'), 'invitation', now() + interval '1 hour' FROM u;
SQL

# host.docker.internal alcanza los puertos publicados del host tanto en Docker Desktop como en Linux.
# Los secretos se pasan por nombre (-e NAME): el valor viene del entorno de este proceso y no aparece
# en la línea de comandos de `docker run` (visible en la lista de procesos).
export LOAD_EMAIL="$email" LOAD_PW="$password" LOAD_INVITATION_TOKEN="$token"
docker run --rm -i --add-host host.docker.internal:host-gateway \
  -e LOAD_EMAIL -e LOAD_PW -e LOAD_INVITATION_TOKEN \
  -e HUB_URL -e MAILPIT_URL -e P95_MS -e DURATION \
  "$K6_IMAGE" run - < tests/load/auth-latency.js
