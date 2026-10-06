# Evidencia "después" de VULN-019: imágenes restantes del compose (run 37471823657)

- Workflow: `Escaneo de la línea base`, `workflow_dispatch` lanzado el 2026-10-06 desde `main` con
  `ref=14047c703b0afc869abbfe79c130541bd8f44166`; conclusión `success`. Job "Inventario de
  vulnerabilidades sembradas" (112296994694), paso "Trivy sobre las imágenes base".
- Run: https://github.com/jorgepaez-ops/IdentityHub/actions/runs/37471823657
- Código escaneado: commit `14047c7` de T16 en `feat/idp-semana-4`.
- Archivos: `trivy-base-images.txt` y los JSON de las ocho imágenes del compose, copiados del artefacto
  `evidencia-linea-base` (retención 90 días).

## Imágenes del compose: Trivy 0.56.2, HIGH/CRITICAL, sin `--ignore-unfixed` (misma metodología que los runs 35534898422 y 36329751647)

| Imagen (antes → después) | Antes (run 35534898422) | T38 (run 36329751647) | Después (este run) | Con parche |
|---|---|---|---|---|
| `postgres:14-bullseye` → `postgres@sha256:efedf359…` (16-bookworm) | 197 | 115 | 117 | 51 |
| `rabbitmq:3.11-management` → `rabbitmq@sha256:ddc75301…` (4-management) | 3 | 0 | 2 | 2 |
| `axllent/mailpit:v1.20` → `@sha256:b68349e3…` (v1.31.4) | 69 | 69 | 0 | 0 |
| `migrate/migrate:v4.17.0` → `@sha256:76cc2074…` (v4.20.1) | 65 | 65 | 6 | 5 |
| `prom/prometheus:v2.51.0` → `@sha256:efd719c9…` (v3.15.0) | 112 | 112 | 2 | 2 |
| `grafana/loki:2.9.4` → `@sha256:1107dd52…` (3.7.8) | 59 | 59 | 0 | 0 |
| `grafana/alloy:v1.0.0` → `@sha256:2aa2099a…` (v1.20.1) | 79 | 79 | 2 | 2 |
| `grafana/grafana:10.4.0` → `@sha256:b28bae15…` (13.2.3) | 126 | 126 | 8 | 8 |
| **Total** | **710** | **625** | **137** | **70** |

## Lectura

- Las seis imágenes que T30 dejó abiertas bajan de 510 a 18 hallazgos. Ninguno es corregible desde este
  proyecto: todos están en binarios o paquetes que empaqueta el proveedor de cada imagen.
  - `migrate` (6): módulos Go dentro del binario `migrate` (`pgproto3/v2` sin parche; `x/crypto`, `x/text`
    y `grpc` con versión corregida que la imagen aún no incorpora).
  - `prometheus` (2): CVE-2026-42154 sobre el módulo `github.com/prometheus/prometheus`, que Trivy 0.56.2
    lee como la seudoversión `v0.0.0-20260925072538-5241a27fe3c6+dirty` del propio binario y compara con
    `0.311.3`/`0.305.2`. Con esa seudoversión no puede saber que es la 3.15.0; el escaneo local de T16
    con Trivy 0.73.0 no lo reporta. Se trata como probable falso positivo de versión.
  - `alloy` (2) y `rabbitmq` (2): CVE-2026-84782 en `openssl`/`libssl3t64` de Ubuntu 24.04, con paquete
    corregido (`3.0.13-0ubuntu3.16`) que las imágenes oficiales todavía no incluyen. En `rabbitmq` es un
    aviso posterior a T38 (mismo digest, 0 entonces).
  - `grafana` (8): `grpc` y `grafana/tempo` dentro de los plugins empaquetados en
    `/usr/share/grafana/data/plugins-bundled/`.
- `postgres`: 115 → 117 con el mismo digest; la diferencia y el aumento de hallazgos con parche (22 → 51)
  vienen de avisos nuevos en la base de datos, no de un cambio de imagen.
- Las imágenes `debian:11-slim`, `golang:1.22-bullseye`, `node:18-bullseye` y `nginx:latest` del mismo paso
  son la línea base vulnerable fijada en el workflow (ADR 0007), no imágenes del compose.
