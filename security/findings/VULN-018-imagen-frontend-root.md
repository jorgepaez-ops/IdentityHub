# VULN-018 — Imagen final del frontend como root

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | abierto |
| **Detectado por** | Trivy config DS002 (baseline-scan) |
| **Componente** | `frontend/Dockerfile` |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-018/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Trivy config DS002 HIGH en frontend/Dockerfile: Image user should not be 'root'
```

## Por qué importa en esta aplicación

Un servidor web comprometido como root aumenta el impacto de una intrusión en el contenedor.

## Remediación

Ejecutar la imagen final con un usuario no privilegiado en T28.
