# VULN-024 — Sin endurecimiento de contenedores en compose

| | |
|---|---|
| **Severidad** | no informada por ningún escáner |
| **Estado** | abierto |
| **Detectado por** | ningún gate lo detecta hoy |
| **Componente** | `deploy/docker-compose.yml` (sin línea reportada por el escáner) |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-024/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Ningún gate lo detecta hoy: `trivy config` solo analiza los Dockerfiles. Sus 6 hallazgos son de backend/Dockerfile y frontend/Dockerfile; ninguno corresponde a docker-compose.yml.
```

## Por qué importa en esta aplicación

El compose no impone defensas de ejecución que limiten el impacto de un proceso comprometido. El comentario actual lo denomina VULN-020, pero ese id pertenece a chi; esta equivalencia se corrige exclusivamente en T30.

## Remediación

Añadir endurecimiento de contenedores y corregir el comentario de compose en T30.
