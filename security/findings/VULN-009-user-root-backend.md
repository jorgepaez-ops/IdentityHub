# VULN-009 — `USER root` (backend)

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | remediado |
| **Detectado por** | Trivy config DS002 (baseline-scan) |
| **Componente** | `backend/Dockerfile` |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-009/evidencia.json` |
| **Commit de remediación** | `a0c64d6` (T27) |
| **Evidencia después** | `docs/evidencia/VULN-009/evidencia.json` (job "8", Trivy config: 0 misconfiguraciones) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36259385478 |

## Evidencia

```
Trivy config DS002 HIGH en backend/Dockerfile: Image user should not be 'root'
```

## Por qué importa en esta aplicación

Un proceso comprometido ejecutado como root amplía el impacto dentro del contenedor.

## Remediación

`USER 65532:65532` explícito en ambas imágenes finales (`api` y `worker`), sobre el usuario
`nonroot` que ya trae la base distroless por defecto. Explícito a propósito: Trivy config (DS002)
analiza el Dockerfile de forma estática y no resuelve el `USER` heredado de una base referenciada
solo por digest. Verificado con `docker inspect --format '{{.Config.User}}'` = `65532:65532` en
ambas imágenes, y con Trivy config: sin hallazgo en `backend/Dockerfile`.
