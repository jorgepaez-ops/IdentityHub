# VULN-017 — `nginx:latest`

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | abierto |
| **Detectado por** | Hadolint DL3007 (baseline-scan) |
| **Componente** | `frontend/Dockerfile:31` |
| **Amenaza** | AM-008 (imagen manipulada entre la construcción y el despliegue) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-017/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Hadolint DL3007 en frontend/Dockerfile:31 (imagen con etiqueta latest)
```

## Por qué importa en esta aplicación

Una etiqueta mutable hace que la misma construcción pueda incorporar una imagen distinta con el tiempo.

## Remediación

Fijar una etiqueta de versión o digest en T28.
