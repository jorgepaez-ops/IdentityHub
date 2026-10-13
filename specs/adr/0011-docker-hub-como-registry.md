# 0011 — Docker Hub como registry de las imágenes publicadas

Estado: aceptada, implementada el 2026-10-12 (C3) · 2026-09-27 · Sustituye la parte de registry de la ADR 0008

## Contexto

La ADR 0008 eligió GHCR para publicar las imágenes. Los entregables del curso exigen imágenes en
**Docker Hub**, versionadas con `vX.Y.Z` y `latest`. El resto de la ADR 0008 (GitHub Actions como
orquestador, permisos mínimos por job, SARIF en code scanning, Cosign keyless) sigue vigente.

## Decisión

- Las imágenes `api`, `worker` y `web` se publican en **Docker Hub** desde un job de GitHub
  Actions que corre solo al crear un tag `vX.Y.Z`, con las etiquetas `vX.Y.Z` y `latest` (esta última solo después de firmar y atestar; ver «Estado de implementación»).
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

## Estado de implementación

Implementada el 2026-10-12 en `.github/workflows/release.yml` (tarea C3). El workflow se dispara solo con tags `vX.Y.Z`; el job `gate` construye y escanea `api`, `worker` y `web`, y el job `publish` (único con el *environment* protegido `dockerhub`, restringido a tags `v*`) sube las mismas imágenes escaneadas a `docker.io/jorgeivp/identity-hub-api`, `-worker` y `-web` solo con `vX.Y.Z`, adjunta el SBOM CycloneDX (Syft) como atestación y firma el digest con Cosign keyless. Después ejecuta `cosign verify` y `cosign verify-attestation --type cyclonedx` sobre el digest (identidad del workflow y emisor OIDC de Actions; tarea S1): si fallan, `latest` no se mueve. Solo después de firmar, atestar y verificar mueve `latest` a ese mismo digest, y únicamente si el tag es el `vX.Y.Z` más alto del repositorio (tarea F7); los pasos de red se reintentan hasta tres veces y los artefactos de imagen se retienen 7 días (tarea F8). Validada con el tag `v0.9.0` (ejecución 38319904991) y, para el orden nuevo, con `v0.9.1` (ejecución 38405885275). Cosign v3 guarda firma y atestación como referrers OCI, por lo que la verificación exige Cosign v3.
