# VULN-019 — Imágenes base antiguas en compose

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH |
| **Estado** | en remediación |
| **Detectado por** | Trivy image sobre las imágenes del compose (baseline-scan) |
| **Componente** | `deploy/docker-compose.yml` (imágenes declaradas) |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-019/evidencia.json` |
| **Commit de remediación** | `824be7d` (T30, solo `postgres` y `rabbitmq`; el resto sigue abierto) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35534898422 / — |

## Evidencia

```
Trivy HIGH/CRITICAL: postgres:14-bullseye 197; rabbitmq:3.11-management 3; axllent/mailpit:v1.20 69; migrate/migrate:v4.17.0 65; prom/prometheus:v2.51.0 112; grafana/loki:2.9.4 59; grafana/alloy:v1.0.0 79; grafana/grafana:10.4.0 126
```

## Por qué importa en esta aplicación

Las imágenes de soporte vulnerables exponen datos, mensajería y observabilidad del entorno de despliegue.

## Remediación

T30 solo tenía en su alcance `postgres` y `rabbitmq` (así lo dice la propia tarea): `postgres:14-bullseye`
→ `postgres:16-bookworm` y `rabbitmq:3.11-management` → `rabbitmq:4-management`, ambos fijados por
digest real. Trivy image confirma `rabbitmq`: `Total: 0 (HIGH: 0, CRITICAL: 0)`; `postgres`: `0`
en el sistema operativo, con un hallazgo HIGH aislado en `gosu` (binario Go empaquetado por la
imagen oficial, fuera de nuestro control) y otro en un certificado `ssl-cert-snakeoil` de relleno
que trae Debian — ninguno de los dos lo puede corregir este proyecto.
`axllent/mailpit`, `migrate/migrate` y las cuatro imágenes de observabilidad (`prometheus`, `loki`,
`alloy`, `grafana`) **siguen sin actualizar**: no estaban en el alcance de T30 y quedan abiertas
para una tarea futura.
