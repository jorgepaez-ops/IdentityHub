# VULN-009 — `USER root` (backend)

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | abierto |
| **Detectado por** | Trivy config DS002 (baseline-scan) |
| **Componente** | `backend/Dockerfile` |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-009/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Trivy config DS002 HIGH en backend/Dockerfile: Image user should not be 'root'
```

## Por qué importa en esta aplicación

Un proceso comprometido ejecutado como root amplía el impacto dentro del contenedor.

## Remediación

Crear y seleccionar un usuario no privilegiado en T27.
