# Identity Hub: web

**identity-hub-web** es la capa web de [Identity Hub](https://github.com/jorgepaez-ops/IdentityHub), un proveedor de identidad didáctico desarrollado como proyecto final universitario. Es un Nginx (`nginx-unprivileged`) que sirve dos aplicaciones de una sola página ya compiladas: la consola del Hub (`identityhub.localhost`) y la aplicación de ejemplo Contabilidad (`contabilidad.localhost`). También actúa como proxy inverso hacia la API (`/api/`, `/oauth/`, `/.well-known/`, `/healthz`, `/readyz`), aplica cabeceras de seguridad y limita la tasa de `/api/v1/auth/`.

Identity Hub se compone de **tres imágenes que trabajan juntas**: [`api`](https://hub.docker.com/r/jorgeivp/identity-hub-api), [`worker`](https://hub.docker.com/r/jorgeivp/identity-hub-worker) y `web`. Esta imagen **no está pensada para ejecutarse sola**: espera encontrar un servicio llamado `api` en el puerto 8081 de su red.

## Etiquetas

| Etiqueta | Contenido |
|---|---|
| `v1.0.0` | Entrega final. Digest: `sha256:d5e812497770fd484fae66cfb2235b2c17b163fe664b4a1f9f4bf35137609887` |
| `latest` | Igual a `v1.0.0` hoy. Solo se mueve a un digest ya firmado y con SBOM, y solo para el `vX.Y.Z` más alto. |
| `v0.9.x` | Versiones previas del proyecto, conservadas por trazabilidad. |

Recomendación: descargue **por digest**, que no cambia aunque se mueva una etiqueta.

```bash
docker pull docker.io/jorgeivp/identity-hub-web@sha256:d5e812497770fd484fae66cfb2235b2c17b163fe664b4a1f9f4bf35137609887
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

Puerto: **8080** (HTTP). No tiene variables de entorno propias: la configuración de Nginx va dentro de la imagen.

| Aspecto | Valor |
|---|---|
| Upstream de la API | Host `api`, puerto `8081` (nombre fijo en `default.conf`; en Compose es el nombre del servicio). |
| Hosts virtuales | `identityhub.localhost` (consola del Hub) y `contabilidad.localhost` (Contabilidad). |
| Salud | `HEALTHCHECK` con `wget` a `http://127.0.0.1:8080/`; `/healthz` y `/readyz` se reenvían a la API. |
| Directorio temporal | Nginx escribe sus temporales en `/tmp`; con `read_only` hay que montar un `tmpfs` ahí. |

## Seguridad

- Base `nginxinc/nginx-unprivileged` (Alpine): usuario **101** (no root) y puerto 8080 sin privilegios.
- El Compose del proyecto la ejecuta con `read_only: true`, `cap_drop: [ALL]`, `no-new-privileges` y `tmpfs` en `/tmp`.
- Cabeceras de seguridad y Content-Security-Policy estrictas, `server_tokens off` y límite de tasa en `/api/v1/auth/`.
- Antes de publicar, el release construye la imagen y la escanea con Trivy: no se publica con CVE HIGH o CRITICAL corregibles.
- Cada digest va firmado con Cosign (sin llaves, identidad del workflow de release) y lleva adjunto su SBOM CycloneDX generado con Syft.

## Verificar la firma y el SBOM

Requiere [Cosign](https://docs.sigstore.dev/cosign/system_config/installation/). Si los comandos terminan sin error, la imagen la publicó el workflow `release.yml` del repositorio en un tag `vX.Y.Z`.

```bash
cosign verify docker.io/jorgeivp/identity-hub-web@sha256:d5e812497770fd484fae66cfb2235b2c17b163fe664b4a1f9f4bf35137609887 \
  --certificate-identity-regexp '^https://github.com/jorgepaez-ops/IdentityHub/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

cosign verify-attestation --type cyclonedx docker.io/jorgeivp/identity-hub-web@sha256:d5e812497770fd484fae66cfb2235b2c17b163fe664b4a1f9f4bf35137609887 \
  --certificate-identity-regexp '^https://github.com/jorgepaez-ops/IdentityHub/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

La segunda orden imprime la atestación con el SBOM completo; añada `> /dev/null` si solo quiere el resultado.

## Más información

- [README del proyecto](https://github.com/jorgepaez-ops/IdentityHub/blob/main/README.md)
- [Manual de despliegue y operación](https://github.com/jorgepaez-ops/IdentityHub/blob/main/docs/manuales/despliegue-y-operacion.md)
- [Índice de la documentación](https://github.com/jorgepaez-ops/IdentityHub/blob/main/docs/README.md)
- Imágenes hermanas: [api](https://hub.docker.com/r/jorgeivp/identity-hub-api), [worker](https://hub.docker.com/r/jorgeivp/identity-hub-worker), [web](https://hub.docker.com/r/jorgeivp/identity-hub-web)
