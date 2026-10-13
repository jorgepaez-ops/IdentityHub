#!/usr/bin/env bash
# RNF-010: prueba de carga de la latencia de autenticación con k6 (imagen fijada por digest).
#
# Usa el stack ya levantado (make up o make up-app): no arranca ni derriba nada. Siembra una cuenta
# desechable con invitación pendiente (el mismo recurso que usan las pruebas E2E) y k6 hace el login
# completo una sola vez; las mediciones no incluyen Argon2id. Sale con código distinto de cero si
# se supera algún umbral de tests/load/auth-latency.js.
#
# Uso: COMPOSE="docker compose --env-file .env -f deploy/docker-compose.yml" scripts/load-test.sh
set -euo pipefail

: "${COMPOSE:?COMPOSE es obligatorio}"
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

# shellcheck disable=SC2086 # COMPOSE es un comando con argumentos, se parte a propósito.
$COMPOSE exec -T db psql -U "$db_user" -d "$db_name" -X -q -v ON_ERROR_STOP=1 \
  -v "email=$email" -v "hash=$token_hash" >/dev/null <<'SQL'
WITH u AS (
  INSERT INTO users (email, password_hash, display_name)
  VALUES (:'email', '$argon2id$load-placeholder', 'Load Test') RETURNING id
), r AS (
  INSERT INTO user_roles (user_id, role_id)
  SELECT u.id, roles.id FROM u, roles WHERE roles.name = 'user' RETURNING 1
)
INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at)
SELECT id, decode(:'hash', 'hex'), 'invitation', now() + interval '1 hour' FROM u;
SQL

# host.docker.internal alcanza los puertos publicados del host tanto en Docker Desktop como en Linux.
docker run --rm -i --add-host host.docker.internal:host-gateway \
  -e LOAD_EMAIL="$email" -e LOAD_PW="$password" -e LOAD_INVITATION_TOKEN="$token" \
  -e HUB_URL -e MAILPIT_URL -e P95_MS -e DURATION \
  "$K6_IMAGE" run - < tests/load/auth-latency.js
