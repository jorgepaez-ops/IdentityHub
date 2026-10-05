#!/usr/bin/env bash
# Avisa si alguna IP quedó en o sobre el límite de fallos de login (RF-017) tras una suite local.
#
# make e2e y make scan-dast hacen logins fallidos a propósito desde la IP del host mientras el
# límite por IP está elevado. audit_log es append-only, así que esas filas no se pueden borrar: al
# restaurar el límite a su valor por defecto, el navegador del desarrollador puede recibir 423 hasta
# que los fallos salgan de LOGIN_FAILURE_WINDOW. Este script solo LEE (psql dentro del contenedor
# de la base) y siempre termina con código 0: nunca cambia el resultado de quien lo invoca.
#
# Variables opcionales: E2E_DB_CONTAINER, E2E_DB_USER, E2E_DB_NAME, E2E_DOCKER,
# LOGIN_IP_LIMIT (20) y LOGIN_FAILURE_WINDOW_MINUTES (15), que deben coincidir con la API.
set -u

container="${E2E_DB_CONTAINER:-identity-hub-db-1}"
db_user="${E2E_DB_USER:-identity}"
db_name="${E2E_DB_NAME:-identity}"
docker_bin="${E2E_DOCKER:-docker}"
# The effective limit is read from the (already restored) API container, so the warning follows the
# API's real configuration; LOGIN_IP_LIMIT overrides it and 20 is only the last-resort default.
api_container="${E2E_API_CONTAINER:-identity-hub-api-1}"
api_limit="$("$docker_bin" inspect "$api_container" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
  | sed -n 's/^LOGIN_IP_MAX_FAILURES=//p' | head -n 1)"
limit="${LOGIN_IP_LIMIT:-${api_limit:-20}}"
window_minutes="${LOGIN_FAILURE_WINDOW_MINUTES:-15}"

case "$limit$window_minutes" in
  '' | *[!0-9]*) exit 0 ;;
esac

# Para cada IP en o sobre el límite: cuántos fallos tiene y en cuántos segundos el fallo número
# (n - límite + 1), contando desde el más antiguo, sale de la ventana y la IP baja del límite.
rows="$(
  "$docker_bin" exec -i "$container" psql -U "$db_user" -d "$db_name" -X -q -t -A -F ' ' \
    -v ON_ERROR_STOP=1 -v "lim=$limit" -v "win=$window_minutes" 2>/dev/null <<'SQL'
WITH recent AS (
  SELECT host(ip) AS ip, created_at,
         row_number() OVER (PARTITION BY ip ORDER BY created_at) AS rn,
         count(*)     OVER (PARTITION BY ip)                      AS n
  FROM audit_log
  WHERE ip IS NOT NULL
    AND action IN ('login_failed', 'mfa_code_rejected')
    AND created_at >= now() - (:'win' || ' minutes')::interval
)
SELECT ip, n,
       greatest(0, ceil(extract(epoch FROM (created_at + (:'win' || ' minutes')::interval - now()))))::int
FROM recent
WHERE n >= :'lim'::int AND rn = n - :'lim'::int + 1
ORDER BY ip;
SQL
)" || {
  echo "AVISO: no se pudo consultar audit_log para revisar el bloqueo por IP (¿contenedor '$container' arriba?)." >&2
  exit 0
}

[ -n "$rows" ] || exit 0

local_time() { # $1 = segundos desde ahora
  date -v+"$1"S +%H:%M 2>/dev/null || date -d "+$1 seconds" +%H:%M 2>/dev/null || echo '?'
}

while read -r ip count seconds; do
  [ -n "$ip" ] || continue
  minutes=$(((seconds + 59) / 60))
  {
    echo "AVISO: la IP $ip tiene $count fallos de login en los últimos ${window_minutes} min (límite: $limit)."
    echo "       Hasta que baje del límite, la API responde 423 a esa IP (tu navegador incluido)."
    echo "       Baja del límite en ~${minutes} min (hora local aprox. $(local_time "$seconds"))."
    echo "       audit_log es append-only: no se borra; solo hay que esperar."
  } >&2
done <<<"$rows"

exit 0
