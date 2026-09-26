# VULN-012 — Secreto en `ENV`

| | |
|---|---|
| **Severidad** | CRITICAL |
| **Estado** | remediado |
| **Detectado por** | Gitleaks (baseline-scan) · Trivy config DS031 |
| **Componente** | `backend/Dockerfile:48-49,64` |
| **Amenaza** | AM-012 (secretos en el repositorio o en la imagen) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-012/evidencia.json` |
| **Commit de remediación** | `afab4e9` (T26) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Gitleaks: 3 hallazgos contrasena-en-variable-de-entorno en backend/Dockerfile líneas 48, 49 y 64
Trivy config DS031 CRITICAL en backend/Dockerfile: Secrets passed via build-args or envs or copied secret files
```

## Por qué importa en esta aplicación

Las variables `ENV` quedan en la imagen y pueden ser leídas por quien tenga acceso a ella.

## Remediación

Retirar secretos de la imagen y suministrarlos solo en tiempo de ejecución en T26.
