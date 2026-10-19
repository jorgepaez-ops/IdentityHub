# Identity Hub: api

**identity-hub-api** es el servicio HTTP de [Identity Hub](https://github.com/jorgepaez-ops/IdentityHub), un proveedor de identidad didáctico (comparable en alcance a un Entra ID pequeño) desarrollado como proyecto final universitario. Es un binario Go estático (`backend/cmd/api`) que expone la API REST `/api/v1` (autenticación con MFA por correo, usuarios, aplicaciones y roles, sesiones, claves de servicio, registro de auditoría), los endpoints OAuth 2.0 con PKCE (`/oauth/`; no implementa OpenID Connect) y las claves públicas de verificación (`/.well-known/jwks.json`), además de `/healthz` y `/readyz`.

Identity Hub se compone de **tres imágenes que trabajan juntas**: `api`, [`worker`](https://hub.docker.com/r/jorgeivp/identity-hub-worker) y [`web`](https://hub.docker.com/r/jorgeivp/identity-hub-web). Esta imagen **no está pensada para ejecutarse sola**: necesita PostgreSQL con las migraciones aplicadas y un broker RabbitMQ, y la consola web la usa como proxy inverso.

## Etiquetas

| Etiqueta | Contenido |
|---|---|
| `v1.0.0` | Entrega final. Digest: `sha256:fa53d22a313b32b261b0a75478fb68ab6f2ab014a6194a6c1e2bb922603845c8` |
| `latest` | Igual a `v1.0.0` hoy. Solo se mueve a un digest ya firmado y con SBOM, y solo para el `vX.Y.Z` más alto. |
| `v0.9.x` | Versiones previas del proyecto, conservadas por trazabilidad. |

Recomendación: descargue **por digest**, que no cambia aunque se mueva una etiqueta.

```bash
docker pull docker.io/jorgeivp/identity-hub-api@sha256:fa53d22a313b32b261b0a75478fb68ab6f2ab014a6194a6c1e2bb922603845c8
```

## Cómo ejecutarla

La forma soportada es el stack completo del repositorio ([`deploy/docker-compose.prod.yml`](https://github.com/jorgepaez-ops/IdentityHub/blob/main/deploy/docker-compose.prod.yml)), que arranca las tres imágenes junto con PostgreSQL, RabbitMQ y las migraciones:

```bash
git clone https://github.com/jorgepaez-ops/IdentityHub.git && cd IdentityHub
make setup      # genera el .env con secretos aleatorios
# en .env: IDENTITY_HUB_API_IMAGE, IDENTITY_HUB_WORKER_IMAGE e IDENTITY_HUB_WEB_IMAGE
# con el digest de cada imagen, más SMTP_HOST, SMTP_PORT, SMTP_FROM, SMTP_USERNAME y SMTP_PASSWORD
make up-prod    # producción simulada local: solo la web publica un puerto (8080)
```

Esta es una simulación local, no un despliegue de producción real. El primer administrador y el resto de pasos están en el [manual de despliegue y operación](https://github.com/jorgepaez-ops/IdentityHub/blob/main/docs/manuales/despliegue-y-operacion.md).

## Configuración

Puerto de escucha: **8081** (`API_PORT`). Las variables obligatorias hacen que el contenedor falle al arrancar con la lista completa de lo que falta.

| Variable | Obligatoria | Para qué sirve |
|---|---|---|
| `DATABASE_URL` | Sí | Conexión a PostgreSQL. |
| `RABBITMQ_URL` | Sí | Conexión al broker RabbitMQ. |
| `JWT_SIGNING_KEY` | Sí | Semilla Ed25519 de 32 bytes en base64 para firmar los tokens. |
| `TRUSTED_PROXIES` | No | CIDR de los proxies de confianza, separados por coma. Solo de ellos se acepta `X-Forwarded-For` para obtener la IP real del cliente; vacía, se usa la dirección de la conexión. |
| `JWT_ISSUER`, `PUBLIC_BASE_URL` | No | URL pública del Hub (por defecto `http://identityhub.localhost:8080`). |
| `BOOTSTRAP_ADMIN_EMAIL` | No | Si no hay ningún administrador, crea una cuenta `admin` pendiente y envía su invitación por correo. |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` | No | Datos de correo; el envío real lo hace el worker. |
| `LOG_LEVEL`, `APP_VERSION` | No | Nivel de log (`info` por defecto) y versión informada. |

Subcomando de salud (la imagen no tiene shell ni `curl`): `api healthcheck`. El esquema de la base lo aplican las migraciones de `db/migrations` del repositorio, con la imagen `migrate/migrate`; la imagen `api` no las incluye.

## Seguridad

- Base `gcr.io/distroless/static-debian12:nonroot`: sin shell ni gestor de paquetes, usuario **65532** (no root).
- El Compose del proyecto la ejecuta con `read_only: true`, `cap_drop: [ALL]`, `no-new-privileges` y `tmpfs` en `/tmp`.
- Antes de publicar, el release construye la imagen y la escanea con Trivy: no se publica con CVE HIGH o CRITICAL corregibles.
- Cada digest va firmado con Cosign (sin llaves, identidad del workflow de release) y lleva adjunto su SBOM CycloneDX generado con Syft.

## Verificar la firma y el SBOM

Requiere [Cosign](https://docs.sigstore.dev/cosign/system_config/installation/). Si los comandos terminan sin error, la imagen la publicó el workflow `release.yml` del repositorio en un tag `vX.Y.Z`.

```bash
cosign verify docker.io/jorgeivp/identity-hub-api@sha256:fa53d22a313b32b261b0a75478fb68ab6f2ab014a6194a6c1e2bb922603845c8 \
  --certificate-identity-regexp '^https://github.com/jorgepaez-ops/IdentityHub/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

cosign verify-attestation --type cyclonedx docker.io/jorgeivp/identity-hub-api@sha256:fa53d22a313b32b261b0a75478fb68ab6f2ab014a6194a6c1e2bb922603845c8 \
  --certificate-identity-regexp '^https://github.com/jorgepaez-ops/IdentityHub/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

La segunda orden imprime la atestación con el SBOM completo; añada `> /dev/null` si solo quiere el resultado.

## Más información

- [README del proyecto](https://github.com/jorgepaez-ops/IdentityHub/blob/main/README.md)
- [Manual de despliegue y operación](https://github.com/jorgepaez-ops/IdentityHub/blob/main/docs/manuales/despliegue-y-operacion.md)
- [Índice de la documentación](https://github.com/jorgepaez-ops/IdentityHub/blob/main/docs/README.md)
- Imágenes hermanas: [api](https://hub.docker.com/r/jorgeivp/identity-hub-api), [worker](https://hub.docker.com/r/jorgeivp/identity-hub-worker), [web](https://hub.docker.com/r/jorgeivp/identity-hub-web)
