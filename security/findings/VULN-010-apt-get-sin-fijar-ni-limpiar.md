# VULN-010 — `apt-get` sin fijar ni limpiar

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | abierto |
| **Detectado por** | Hadolint DL3008/DL3009 (baseline-scan) · Trivy config DS029 |
| **Componente** | `backend/Dockerfile:24,51,66` |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-010/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Hadolint DL3008 en backend/Dockerfile líneas 24, 51 y 66
Hadolint DL3009 en línea 66
Hadolint DL3015 en líneas 24, 51 y 66
Trivy config DS029 HIGH x3 (apt-get sin --no-install-recommends)
```

## Por qué importa en esta aplicación

Dependencias no fijadas y capas con paquetes sobrantes reducen reproducibilidad y aumentan la superficie de ataque.

## Remediación

Fijar dependencias, usar `--no-install-recommends` y limpiar listas en T27.
