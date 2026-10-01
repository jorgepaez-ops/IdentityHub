# 0011 — Docker Hub como registry de las imágenes publicadas

Estado: aceptada · 2026-09-27 · Sustituye la parte de registry de la ADR 0008

## Contexto

La ADR 0008 eligió GHCR para publicar las imágenes. Los entregables del curso exigen imágenes en
**Docker Hub**, versionadas con `vX.Y.Z` y `latest`. El resto de la ADR 0008 (GitHub Actions como
orquestador, permisos mínimos por job, SARIF en code scanning, Cosign keyless) sigue vigente.

## Decisión

- Las imágenes `api`, `worker` y `web` se publican en **Docker Hub** desde un job de GitHub
  Actions que corre solo al crear un tag `vX.Y.Z`, con las etiquetas `vX.Y.Z` y `latest`.
- La publicación ocurre al final del proyecto (semana 4), después de que las imágenes pasen los
  mismos gates que hoy (Trivy sin HIGH/CRITICAL corregibles, Hadolint, Trivy config).
- **Firma con Cosign keyless** (OIDC del workflow) y **SBOM CycloneDX con Syft** adjunto como
  atestación: Docker Hub acepta los artefactos OCI de firma y atestación igual que GHCR.
- El tag `v0.0.0-vuln-baseline` nunca se publica (ADR 0007).

## Consecuencias

- **Aparece una credencial de larga vida.** GHCR se autentica con el `GITHUB_TOKEN` efímero del
  workflow; Docker Hub necesita un usuario y un *access token* guardados como secretos del
  repositorio. Se mitiga con un token con permiso solo de lectura y escritura sobre los
  repositorios del proyecto, disponible únicamente para el job de publicación (un *environment*
  de GitHub con protección) y rotable. Es la misma clase de riesgo que se documenta en
  AM-009/AM-022 del modelo de amenazas.
- Docker Hub impone límites de descarga a usuarios anónimos; para quien clone el repositorio y
  levante el stack no cambia nada, porque el compose construye las imágenes localmente.
- La ADR 0008 queda en estado "sustituida parcialmente por 0011" en lo que toca al registry.
