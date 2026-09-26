# VULN-016 — `node:18-bullseye`

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH |
| **Estado** | remediado |
| **Detectado por** | Trivy image sobre node:18-bullseye (baseline-scan) |
| **Componente** | `frontend/Dockerfile` (builder `node:18-bullseye`) |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-016/evidencia.json` |
| **Commit de remediación** | `8f3d461` (T28) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35534898422 / — |

## Evidencia

```
Trivy sobre node:18-bullseye (builder del frontend): 2103 vulnerabilidades HIGH/CRITICAL (218 CRITICAL, 1885 HIGH)
Trivy sobre baseline/web (imagen final construida): 64 (1 CRITICAL, 63 HIGH), base debian 13.7 porque nginx:latest se resuelve hoy
```

## Por qué importa en esta aplicación

La imagen de construcción contiene una base obsoleta con un volumen muy alto de vulnerabilidades conocidas.

## Remediación

Builder cambiado a `node:24-bookworm` (LTS activa desde octubre de 2025), fijado por digest real
(`sha256:64af3819...`). Trivy image sobre `identity-hub-web` (`--severity HIGH,CRITICAL
--ignore-unfixed`): `Total: 0 (HIGH: 0, CRITICAL: 0)`.
