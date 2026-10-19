# Identity Hub: worker

**identity-hub-worker** es el consumidor de eventos de [Identity Hub](https://github.com/jorgepaez-ops/IdentityHub), un proveedor de identidad didáctico desarrollado como proyecto final universitario. Es un binario Go estático (`backend/cmd/worker`) que lee los eventos que publica la API en RabbitMQ y entrega las notificaciones por correo (registro, invitaciones, códigos MFA, restablecimiento de contraseña y alertas de seguridad) a un relay SMTP. Expone `/metrics` (Prometheus) y `/healthz` en el puerto 9091.

Identity Hub se compone de **tres imágenes que trabajan juntas**: [`api`](https://hub.docker.com/r/jorgeivp/identity-hub-api), `worker` y [`web`](https://hub.docker.com/r/jorgeivp/identity-hub-web). Esta imagen **no está pensada para ejecutarse sola**: sin la API no hay eventos que consumir. No se conecta a la base de datos ni recibe la clave de firma JWT (mínimo privilegio).

## Etiquetas

| Etiqueta | Contenido |
|---|---|
| `v1.0.0` | Entrega final. Digest: `sha256:9bd06633b7321707b4b72a4f4b6b2e13905d568576e8e532c42ae950a1bd76d5` |
| `latest` | Igual a `v1.0.0` hoy. Solo se mueve a un digest ya firmado y con SBOM, y solo para el `vX.Y.Z` más alto. |
| `v0.9.x` | Versiones previas del proyecto, conservadas por trazabilidad. |

Recomendación: descargue **por digest**, que no cambia aunque se mueva una etiqueta.

```bash
docker pull docker.io/jorgeivp/identity-hub-worker@sha256:9bd06633b7321707b4b72a4f4b6b2e13905d568576e8e532c42ae950a1bd76d5
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

Puerto: **9091** (`/metrics` y `/healthz`), solo para la red interna. Las variables obligatorias hacen que el contenedor falle al arrancar con la lista completa de lo que falta.

| Variable | Obligatoria | Para qué sirve |
|---|---|---|
| `RABBITMQ_URL` | Sí | Conexión al broker RabbitMQ de donde consume los eventos. |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` | En producción | Relay SMTP y remitente (en `make up-prod` son obligatorios). |
| `SMTP_TLS` | No | `starttls` por defecto; `none` no se permite con credenciales. |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | En producción | Credenciales del relay; solo se presentan tras STARTTLS. |
| `SMTP_CA_FILE` | No | CA propia para verificar el relay (desarrollo); vacío usa las raíces del sistema. |
| `LOG_LEVEL`, `APP_VERSION` | No | Nivel de log (`info` por defecto) y versión informada. |

La imagen no define `HEALTHCHECK` porque no trae shell ni `curl`; el orquestador puede sondear `http://<contenedor>:9091/healthz`.

## Seguridad

- Base `gcr.io/distroless/static-debian12:nonroot`: sin shell ni gestor de paquetes, usuario **65532** (no root).
- El Compose del proyecto la ejecuta con `read_only: true`, `cap_drop: [ALL]`, `no-new-privileges` y `tmpfs` en `/tmp`.
- No recibe `DATABASE_URL`, `JWT_SIGNING_KEY` ni `TRUSTED_PROXIES`: solo lo que necesita para consumir la cola y enviar correo.
- Antes de publicar, el release construye la imagen y la escanea con Trivy: no se publica con CVE HIGH o CRITICAL corregibles.
- Cada digest va firmado con Cosign (sin llaves, identidad del workflow de release) y lleva adjunto su SBOM CycloneDX generado con Syft.

## Verificar la firma y el SBOM

Requiere [Cosign](https://docs.sigstore.dev/cosign/system_config/installation/). Si los comandos terminan sin error, la imagen la publicó el workflow `release.yml` del repositorio en un tag `vX.Y.Z`.

```bash
cosign verify docker.io/jorgeivp/identity-hub-worker@sha256:9bd06633b7321707b4b72a4f4b6b2e13905d568576e8e532c42ae950a1bd76d5 \
  --certificate-identity-regexp '^https://github.com/jorgepaez-ops/IdentityHub/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

cosign verify-attestation --type cyclonedx docker.io/jorgeivp/identity-hub-worker@sha256:9bd06633b7321707b4b72a4f4b6b2e13905d568576e8e532c42ae950a1bd76d5 \
  --certificate-identity-regexp '^https://github.com/jorgepaez-ops/IdentityHub/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

La segunda orden imprime la atestación con el SBOM completo; añada `> /dev/null` si solo quiere el resultado.

## Más información

- [README del proyecto](https://github.com/jorgepaez-ops/IdentityHub/blob/main/README.md)
- [Manual de despliegue y operación](https://github.com/jorgepaez-ops/IdentityHub/blob/main/docs/manuales/despliegue-y-operacion.md)
- [Índice de la documentación](https://github.com/jorgepaez-ops/IdentityHub/blob/main/docs/README.md)
- Imágenes hermanas: [api](https://hub.docker.com/r/jorgeivp/identity-hub-api), [worker](https://hub.docker.com/r/jorgeivp/identity-hub-worker), [web](https://hub.docker.com/r/jorgeivp/identity-hub-web)
