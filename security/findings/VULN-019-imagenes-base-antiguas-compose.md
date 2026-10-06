# VULN-019 — Imágenes base antiguas en compose

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH |
| **Estado** | remediado: las 8 imágenes del compose actualizadas y fijadas por digest (T30 y T16); el residual depende de imágenes nuevas de cada proveedor |
| **Detectado por** | Trivy image sobre las imágenes del compose (baseline-scan) |
| **Componente** | `deploy/docker-compose.yml` (imágenes declaradas) |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-019/evidencia.json` — informe Desktop §3 VULN-019 |
| **Commit de remediación** | `824be7d` (T30: `postgres` y `rabbitmq`), `14047c7` (T16: `mailpit`, `migrate`, `prometheus`, `loki`, `alloy` y `grafana`) y `924b581` (T16: `postgres` reconstruida) |
| **Evidencia después** | `security/evidence/actions-37473616659/README.md` (T16, run sobre `924b581`: total de las 8 imágenes 710 → 108; `mailpit` y `loki` 0, `prometheus` 2, `alloy` 2, `rabbitmq` 2, `migrate` 6, `grafana` 8, `postgres` 88) — informe Desktop, sección VULN-019 después (T16), pendiente. Antes, parcial de T30: `docs/evidencia/VULN-019/evidencia.json` y `security/evidence/actions-36329751647/README.md` (T38, run sobre `v0.1.0-hardened`, sin `--ignore-unfixed` como el antes: `postgres` 197 → 115, de ellos 22 con parche, todos `stdlib` de Go en gosu, fuera de nuestro control; `rabbitmq` 3 → 0; Mailpit, migrate y las 4 imágenes de observabilidad sin cambios, siguen abiertas/fuera de alcance T30. La medición local de T30 —`postgres:16-bookworm` 0 en SO, 1 HIGH en gosu— contaba solo hallazgos con parche; total de las 8 imágenes del compose 710 → 625) — informe Desktop, seccion VULN-019 despues (T38; Discrepancia 8) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35534898422 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/37473616659 (T16, `workflow_dispatch` con `ref=924b581`); parcial de T38: https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36329751647 (`baseline-scan.yml` por `workflow_dispatch` con `ref=v0.1.0-hardened`, T38); `ci.yml` no escanea las imágenes de infraestructura del compose |

## Evidencia

```
Trivy HIGH/CRITICAL: postgres:14-bullseye 197; rabbitmq:3.11-management 3; axllent/mailpit:v1.20 69; migrate/migrate:v4.17.0 65; prom/prometheus:v2.51.0 112; grafana/loki:2.9.4 59; grafana/alloy:v1.0.0 79; grafana/grafana:10.4.0 126
```

## Por qué importa en esta aplicación

Las imágenes de soporte vulnerables exponen datos, mensajería y observabilidad del entorno de despliegue.

## Remediación

**Estado en T30 (histórico).** T30 solo tenía en su alcance `postgres` y `rabbitmq` (así lo dice la propia tarea): `postgres:14-bullseye`
→ `postgres:16-bookworm` y `rabbitmq:3.11-management` → `rabbitmq:4-management`, ambos fijados por
digest real. Trivy image confirma `rabbitmq`: `Total: 0 (HIGH: 0, CRITICAL: 0)`; `postgres`: `0`
en el sistema operativo, con un hallazgo HIGH aislado en `gosu` (binario Go empaquetado por la
imagen oficial, fuera de nuestro control) y otro en un certificado `ssl-cert-snakeoil` de relleno
que trae Debian — ninguno de los dos lo puede corregir este proyecto.
`axllent/mailpit`, `migrate/migrate` y las cuatro imágenes de observabilidad (`prometheus`, `loki`,
`alloy`, `grafana`) **siguen sin actualizar**: no estaban en el alcance de T30 y quedan abiertas
para una tarea futura.

**T16 completa la remediación:** `axllent/mailpit` v1.31.4, `migrate/migrate` v4.20.1, `prom/prometheus`
v3.15.0, `grafana/loki` 3.7.8, `grafana/alloy` v1.20.1 y `grafana/grafana` 13.2.3, cada una fijada por el
digest de su índice multiarquitectura. La configuración de observabilidad funciona sin cambios (métricas
de Prometheus, logs de Alloy a Loki, fuentes de datos de Grafana) y la suite E2E pasa contra el stack
actualizado. `postgres` se vuelve a fijar en el digest reconstruido de `16-bookworm` (misma 16.15), que
corrige perl y pcre2. El residual (108 HIGH/CRITICAL en las 8 imágenes, frente a 710) son paquetes del
sistema sin parche publicado y componentes con parche que el proveedor aún no incorporó a su imagen
(Go en `gosu`, `migrate` y plugins de Grafana; openssl de Ubuntu); se cierra fijando la siguiente imagen
que los publique. Detalle en `security/evidence/actions-37473616659/README.md`.
