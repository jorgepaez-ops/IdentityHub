# Evidencia "antes" (pasada 2): escaneo de la línea base con el workflow corregido (run 35534898422)

- Workflow: `Escaneo de la línea base`, `workflow_dispatch` lanzado el 2026-09-20 desde la rama `feat/idp-semana-2`
  (workflow en `70f7d88`) con `ref=v0.0.0-vuln-baseline`; conclusión `success`.
- Run: https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35534898422
- Código escaneado: tag `v0.0.0-vuln-baseline` (`053e15fb27b1aaf974c7c818108eee64628973e9`).
- Datos tomados del log del job y del artefacto `evidencia-linea-base` (retención 90 días), descargado con
  autorización del usuario y guardado fuera del repo. Resumen por gravedad en `trivy-resumen.txt`; registro de
  construcción (con la contraseña sembrada redactada) en `docker-build.txt`.

## Gitleaks (D1 corregido)
`leaks found: 12`: ya no hay hallazgos autorreferenciales (en el run 35476102444 eran 28).

## Construcción de las imágenes de la línea base (D2)
| Imagen | Resultado |
|---|---|
| `baseline/api` (target api) | exit code 1 (apt-get sobre Debian 11, 404 en bullseye-security) |
| `baseline/worker` (target worker) | exit code 1 (mismo motivo) |
| `baseline/web` (target web) | exit code 0 |

Trivy no se ejecuta sobre `api` ni `worker` (no existen); sobre `baseline/web` da 64 (1 CRITICAL, 63 HIGH, 0 con corrección), base debian 13.7 (`nginx:latest` de hoy).

## Trivy sobre imágenes base (HIGH + CRITICAL, Trivy 0.56.2, resueltas el 2026-09-20)
Cuentas brutas de vulnerabilidades HIGH y CRITICAL, incluidas las sin corrección. Las etiquetas móviles
(`nginx:latest`) apuntan a lo que había el día del escaneo, no a lo que había al crear la línea base.

| Imagen | Origen | Hallazgos | VULN |
|---|---|---|---|
| `debian:11-slim` | base de `api` y `worker` | 58 | VULN-008 |
| `golang:1.22-bullseye` | builder de `api` y `worker` | 1625 | VULN-008 |
| `node:18-bullseye` | builder del frontend | 2103 | VULN-016 |
| `nginx:latest` | imagen final del frontend | 64 | VULN-017, VULN-018 |
| `postgres:14-bullseye` | compose | 197 | VULN-019 |
| `rabbitmq:3.11-management` | compose | 3 | VULN-019 |
| `axllent/mailpit:v1.20` | compose | 69 | VULN-019 |
| `migrate/migrate:v4.17.0` | compose | 65 | VULN-019 |
| `prom/prometheus:v2.51.0` | compose | 112 | VULN-019 |
| `grafana/loki:2.9.4` | compose | 59 | VULN-019 |
| `grafana/alloy:v1.0.0` | compose | 79 | VULN-019 |
| `grafana/grafana:10.4.0` | compose | 126 | VULN-019 |
