# Escaneo semanal (`scheduled-scan.yml`), run 35547924202

- Workflow: `Escaneo semanal`, `workflow_dispatch` lanzado el 2026-09-21 (00:30 UTC) sobre `main` (`acd3da8`), con
  autorización del usuario (el disparo por calendario corre los lunes a las 06:00 UTC).
- Run: https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35547924202 (conclusión `success`).
- Resultado de `govulncheck` sobre `main`: **33 vulnerabilidades** (5 módulos y la biblioteca estándar), las mismas que la
  línea base. La primera es GO-2026-6372 (`amqp091-go`).

## Defecto encontrado (D9)

El paso "Abrir una incidencia si aparecen vulnerabilidades nuevas" **no se ejecutó**, y no se abrió ninguna incidencia,
aunque `govulncheck` encontró 33. Causa: el paso `govulncheck` usa `govulncheck ./... | tee /tmp/govulncheck.txt` y
GitHub ejecuta `run` con `bash -e` (sin `pipefail`), así que la tubería devuelve el estado de `tee` (0) y el resultado
del paso es `success`; la condición `steps.govuln.outcome == 'failure'` nunca se cumple. El control continuo que este
workflow promete (vulnerabilidades nuevas en código que no cambia) estaba desactivado sin que se notara.

Corrección: `shell: bash` en ese paso (activa `pipefail`); ahora el paso falla cuando `govulncheck` encuentra algo y la
incidencia se abre. Queda pendiente relanzarlo tras el próximo push (requiere autorización: abrirá una incidencia pública
con la salida de `govulncheck`).
