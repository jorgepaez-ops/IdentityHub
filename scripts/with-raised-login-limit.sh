#!/usr/bin/env bash
# Ejecuta un comando con LOGIN_IP_MAX_FAILURES elevado en la API y lo restaura al terminar.
#
# Las suites locales (make e2e, make scan-dast) hacen peticiones y logins fallidos a propósito desde
# una sola IP. Se sube SOLO el límite por IP (RF-017); el límite por cuenta no se relaja. La
# restauración corre con trap en EXIT/INT/TERM, avisa si falla (y entonces sale con error) y conserva
# el código de salida del comando. Al final avisa si alguna IP quedó bloqueada (solo lectura).
#
# Uso: COMPOSE="docker compose ..." LOCAL_TEST_LOGIN_IP_MAX_FAILURES=1000 \
#        scripts/with-raised-login-limit.sh <comando> [args...]
set -u

: "${COMPOSE:?COMPOSE es obligatorio}"
: "${LOCAL_TEST_LOGIN_IP_MAX_FAILURES:?LOCAL_TEST_LOGIN_IP_MAX_FAILURES es obligatorio}"
[ $# -gt 0 ] || { echo "uso: $0 <comando> [args...]" >&2; exit 2; }

here="$(cd "$(dirname "$0")" && pwd)"

cleanup() {
  status=$?
  trap - EXIT INT TERM
  # shellcheck disable=SC2086 # COMPOSE es un comando con argumentos, se parte a propósito.
  if ! $COMPOSE up -d --no-deps --wait api; then
    echo "ERROR: no se pudo restaurar la API con el límite por defecto; puede seguir con LOGIN_IP_MAX_FAILURES elevado. Ejecuta 'make up' para restaurarla." >&2
    [ "$status" -ne 0 ] || status=1
  fi
  "$here/ip-lockout-warning.sh" || true
  exit "$status"
}
trap cleanup EXIT INT TERM

# shellcheck disable=SC2086
LOGIN_IP_MAX_FAILURES="$LOCAL_TEST_LOGIN_IP_MAX_FAILURES" $COMPOSE up -d --no-deps --wait api || exit $?
"$@"
