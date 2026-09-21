# VULN-008 — Base Debian 11 (backend)

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH |
| **Estado** | abierto |
| **Detectado por** | docker build (baseline-scan) · Trivy image sobre imágenes base (baseline-scan, run 35534898422) |
| **Componente** | `backend/Dockerfile` (base `debian:11-slim`) |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-008/evidencia.json` |
| **Commit de remediación** | |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
docker build de backend (target api) sobre debian:11-slim falla en apt-get install: 404 en bullseye-security (Debian 11 sin soporte); api y worker terminan con exit code 1

Trivy sobre debian:11-slim: 58 vulnerabilidades HIGH/CRITICAL; sobre golang:1.22-bullseye: 1625 (run 35534898422, 2026-09-20)
```

## Por qué importa en esta aplicación

Una base sin soporte impide reconstrucciones fiables y conserva vulnerabilidades conocidas en el contenedor del IdP.

## Remediación

Actualizar y fijar las imágenes base soportadas en T27.
