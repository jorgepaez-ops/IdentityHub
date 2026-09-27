# VULN-019 — Imágenes base antiguas en compose

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH |
| **Estado** | remediado parcialmente: solo `postgres` y `rabbitmq`; el resto sigue abierto |
| **Detectado por** | Trivy image sobre las imágenes del compose (baseline-scan) |
| **Componente** | `deploy/docker-compose.yml` (imágenes declaradas) |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-019/evidencia.json` — informe Desktop §3 VULN-019 |
| **Commit de remediación** | `824be7d` (T30, solo `postgres` y `rabbitmq`; el resto sigue abierto) |
| **Evidencia después** | `docs/evidencia/VULN-019/evidencia.json` y `security/evidence/actions-36329751647/README.md` (T38, run sobre `v0.1.0-hardened`, sin `--ignore-unfixed` como el antes: `postgres` 197 → 115, de ellos 22 con parche, todos `stdlib` de Go en gosu, fuera de nuestro control; `rabbitmq` 3 → 0; Mailpit, migrate y las 4 imágenes de observabilidad sin cambios, siguen abiertas/fuera de alcance T30. La medición local de T30 —`postgres:16-bookworm` 0 en SO, 1 HIGH en gosu— contaba solo hallazgos con parche) — captura pendiente |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35534898422 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36329751647 (`baseline-scan.yml` por `workflow_dispatch` con `ref=v0.1.0-hardened`, T38); `ci.yml` no escanea las imágenes de infraestructura del compose |

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
