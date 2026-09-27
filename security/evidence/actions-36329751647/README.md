# Evidencia "después" de VULN-019: escaneo del estado corregido (run 36329751647)

- Workflow: `Escaneo de la línea base`, `workflow_dispatch` lanzado el 2026-09-27 desde `main` con
  `ref=v0.1.0-hardened`; conclusión `success`. Job "Inventario de vulnerabilidades sembradas" (108649306649).
- Run: https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36329751647
- Código escaneado: tag `v0.1.0-hardened` (`2ee59e2565a68b05be74b46a2c498f69d5eb17ce`, merge del PR #3 en `main`).
- Archivos: `trivy-base-images.txt` (salida del paso "Trivy sobre las imágenes base") y los JSON de
  `postgres` y `rabbitmq`, copiados del artefacto `evidencia-linea-base` (retención 90 días).

## Imágenes del compose: Trivy 0.56.2, HIGH/CRITICAL, sin `--ignore-unfixed` (misma metodología que el run 35534898422)

| Imagen (antes → después) | Antes (run 35534898422) | Después | Con parche |
|---|---|---|---|
| `postgres:14-bullseye` → `postgres@sha256:efedf359…` (16-bookworm, Debian 12.15) | 197 | 115 | 22 |
| `rabbitmq:3.11-management` → `rabbitmq@sha256:ddc75301…` (4-management, Ubuntu 24.04) | 3 | 0 | 0 |
| `axllent/mailpit:v1.20` (sin cambios) | 69 | 69 | — |
| `migrate/migrate:v4.17.0` (sin cambios) | 65 | 65 | — |
| `prom/prometheus:v2.51.0` (sin cambios) | 112 | 112 | — |
| `grafana/loki:2.9.4` (sin cambios) | 59 | 59 | — |
| `grafana/alloy:v1.0.0` (sin cambios) | 79 | 79 | — |
| `grafana/grafana:10.4.0` (sin cambios) | 126 | 126 | — |

Las cuatro primeras imágenes del paso (`debian:11-slim`, `golang:1.22-bullseye`, `node:18-bullseye`,
`nginx:latest`) están fijadas en el propio workflow para la evidencia de la línea base; no son del compose.

## Lectura

- `postgres`: 93 de los 115 son paquetes del sistema operativo sin parche publicado (libxml2, util-linux,
  perl…). Los 22 con parche son todos `stdlib` de Go, dentro del binario `gosu` que empaqueta la imagen
  oficial: fuera de nuestro control hasta que el proveedor publique una imagen nueva. La medición local de
  T30 ("0 en el sistema operativo; 1 HIGH en gosu") contaba solo hallazgos con parche; el recuento de gosu
  pasó de 1 a 22 entre el 2026-09-26 y el 2026-09-27 sin cambio de imagen (mismo digest), lo que apunta a
  avisos nuevos en la base de datos de vulnerabilidades.
- VULN-019 sigue **remediado parcialmente**: `mailpit`, `migrate` y las cuatro imágenes de observabilidad
  no se actualizaron (fuera del alcance de T30).
