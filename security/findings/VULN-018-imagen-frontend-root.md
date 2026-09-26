# VULN-018 — Imagen final del frontend como root

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | remediado |
| **Detectado por** | Trivy config DS002 (baseline-scan) |
| **Componente** | `frontend/Dockerfile` |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-018/evidencia.json` |
| **Commit de remediación** | `8f3d461` (T28) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
Trivy config DS002 HIGH en frontend/Dockerfile: Image user should not be 'root'
```

## Por qué importa en esta aplicación

Un servidor web comprometido como root aumenta el impacto de una intrusión en el contenedor.

## Remediación

`nginxinc/nginx-unprivileged:stable` ya corre como uid 101 por defecto; se declaró `USER 101`
explícito además (mismo motivo que VULN-009 en T27: Trivy config no resuelve el `USER` heredado de
una base referenciada solo por digest). Verificado con `docker inspect --format
'{{.Config.User}}'` = `101`, y con Trivy config: sin hallazgo en `frontend/Dockerfile`.
