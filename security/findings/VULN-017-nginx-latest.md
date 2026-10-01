# VULN-017 — `nginx:latest`

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | remediado |
| **Detectado por** | Hadolint DL3007 (baseline-scan) |
| **Componente** | `frontend/Dockerfile:31` |
| **Amenaza** | AM-008 (imagen manipulada entre la construcción y el despliegue) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-017/evidencia.json` — informe Desktop §3 VULN-017 |
| **Commit de remediación** | `8f3d461` (T28) |
| **Evidencia después** | `docs/evidencia/VULN-017/evidencia.json` (Hadolint local, T28: limpio; Hadolint en job 2 de `ci.yml`: sin hallazgos) — informe Desktop, seccion VULN-017 despues |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36283211113 (Hadolint en job 2 de `ci.yml`: sin hallazgos; conserva evidencia local de T28) |

## Evidencia

```
Hadolint DL3007 en frontend/Dockerfile:31 (imagen con etiqueta latest)
```

## Por qué importa en esta aplicación

Una etiqueta mutable hace que la misma construcción pueda incorporar una imagen distinta con el tiempo.

## Remediación

Imagen final cambiada a `nginxinc/nginx-unprivileged:stable`, fijada por digest real
(`sha256:0918d093...`) en vez de una etiqueta móvil. Hadolint sobre `frontend/Dockerfile`: sin
DL3007.

Actualización (2026-10-01, PR #6): la imagen Debian fijada acumuló 13 CVE HIGH corregibles
publicados después de fijarla (`openssl`, `libpcre2`, `libheif`; CI run 36863251783) y el digest
más reciente de `stable` (2026-09-28) todavía no los incluía. La base pasó a
`nginxinc/nginx-unprivileged:stable-alpine` (Alpine 3.24.2), fijada por digest
(`sha256:ed04ec1f...`), con 0 HIGH/CRITICAL corregibles. La remediación de esta ficha no cambia:
sigue siendo una base fijada por digest, no una etiqueta móvil.
