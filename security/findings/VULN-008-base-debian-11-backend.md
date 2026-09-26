# VULN-008 — Base Debian 11 (backend)

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH |
| **Estado** | remediado |
| **Detectado por** | docker build (baseline-scan) · Trivy image sobre imágenes base (baseline-scan, run 35534898422) |
| **Componente** | `backend/Dockerfile` (base `debian:11-slim`) |
| **Amenaza** | AM-020 (escape de contenedor desde un proceso comprometido) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-008/evidencia.json` |
| **Commit de remediación** | `a0c64d6` (T27) |
| **Evidencia después** | `docs/evidencia/VULN-008/evidencia.json` (job "9-10", Trivy image: 0 vulnerabilidades) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36259385478 |

## Evidencia

```
docker build de backend (target api) sobre debian:11-slim falla en apt-get install: 404 en bullseye-security (Debian 11 sin soporte); api y worker terminan con exit code 1

Trivy sobre debian:11-slim: 58 vulnerabilidades HIGH/CRITICAL; sobre golang:1.22-bullseye: 1625 (run 35534898422, 2026-09-20)
```

## Por qué importa en esta aplicación

Una base sin soporte impide reconstrucciones fiables y conserva vulnerabilidades conocidas en el contenedor del IdP.

## Remediación

Base final cambiada a `gcr.io/distroless/static-debian12:nonroot`, fijada por digest real
(`sha256:afa5c872...`); builder a `golang:1.25-bookworm` (`sha256:3b4a1151...`), también por
digest. Trivy image (`--severity HIGH,CRITICAL --ignore-unfixed`) sobre `identity-hub-api` e
`identity-hub-worker`: `Total: 0 (HIGH: 0, CRITICAL: 0)`.
