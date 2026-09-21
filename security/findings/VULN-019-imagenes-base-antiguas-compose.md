# VULN-019 — Imágenes base antiguas en compose

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH |
| **Estado** | abierto |
| **Detectado por** | Trivy image sobre las imágenes del compose (baseline-scan) |
| **Componente** | `deploy/docker-compose.yml` (imágenes declaradas) |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-019/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35534898422 / — |

## Evidencia

```
Trivy HIGH/CRITICAL: postgres:14-bullseye 197; rabbitmq:3.11-management 3; axllent/mailpit:v1.20 69; migrate/migrate:v4.17.0 65; prom/prometheus:v2.51.0 112; grafana/loki:2.9.4 59; grafana/alloy:v1.0.0 79; grafana/grafana:10.4.0 126
```

## Por qué importa en esta aplicación

Las imágenes de soporte vulnerables exponen datos, mensajería y observabilidad del entorno de despliegue.

## Remediación

Actualizar y fijar las imágenes declaradas en compose en T30.
