# Manual de despliegue y operación

Este manual describe el entorno local actual con Docker Compose. No afirma soporte de producción: la referencia cloud y `docker-compose.prod.yml` ya están incorporadas (referencia cloud como IaC y compose de producción simulada) y las imágenes se publican en Docker Hub con `release.yml`.

## Inicio rápido

### Prerrequisitos

- Docker y Docker Compose para levantar los contenedores.
- `make` para usar los objetivos del repositorio.
- Un `.env` local; `make setup` lo genera con secretos aleatorios a partir de `.env.example` y no se versiona.

```bash
make setup
docker compose up -d   # solo la aplicación (equivale a make up-app); make up añade la observabilidad
make ps
```

`make setup` no sobrescribe un `.env` existente. Regenerarlo (`python3 scripts/setup_env.py --force`) cambia las contraseñas, pero Postgres y RabbitMQ conservan las de sus volúmenes: después hay que ejecutar `make clean`, que **borra los datos**, o el stack no podrá autenticarse. `docker compose up -d` (raíz) y `make up` construyen y levantan el perfil ordinario. Use `make down` para detenerlo conservando volúmenes y `make clean` únicamente si desea detenerlo **y borrar los volúmenes**. Para reconstruir un servicio concreto, use `make restart S=api`; para seguir su salida, `make logs S=api`.

## Configuración por entorno

Complete `.env` sin guardar secretos en Git. Las variables que se muestran a continuación provienen de [`.env.example`](../../.env.example); la API también recibe algunos valores fijados por el Compose local.

| Grupo | Variables | Uso |
|---|---|---|
| PostgreSQL | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | Usuario, contraseña y base del contenedor PostgreSQL. |
| Rol de aplicación y migraciones | `IDENTITY_APP_PASSWORD`, `MIGRATE_DATABASE_URL`, `DATABASE_URL` | Credencial de mínimo privilegio, conexión del migrador y conexión de la API. El worker no recibe `DATABASE_URL`: solo consume la cola y envía SMTP. Al desplegar en AWS, publique primero la imagen del worker que ya usa `LoadWorker` y después aplique el Terraform que le quita el secreto y el acceso a la base y los secretos `JWT_SIGNING_KEY` y `TRUSTED_PROXIES`; una imagen anterior exige `DATABASE_URL` y `JWT_SIGNING_KEY` y no arrancaría. |
| RabbitMQ | `RABBITMQ_DEFAULT_USER`, `RABBITMQ_DEFAULT_PASS`, `RABBITMQ_URL` | Usuario, contraseña y URL AMQP del broker. |
| API y JWT | `API_PORT`, `API_BASE_URL`, `LOG_LEVEL`, `JWT_SIGNING_KEY`, `JWT_ISSUER`, `TRUSTED_PROXIES`, `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL` | Puerto, URL local, nivel de log, firma y parámetros de token; `JWT_SIGNING_KEY` no tiene valor por defecto. Solo la API los recibe: el worker no firma tokens ni usa `TRUSTED_PROXIES`. |
| Correo (SMTP) | `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_TLS`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_CA_FILE` | Destino y seguridad del SMTP del worker; por defecto apunta a Mailpit con STARTTLS y autenticación (ver "Correo SMTP"). `SMTP_PASSWORD` lo genera `make setup`. |
| Observabilidad | `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD` | Acceso inicial de Grafana (los levanta `make up`). |
| Arranque inicial | `BOOTSTRAP_ADMIN_EMAIL` | Variable opcional consumida por Compose: crea o reemite por invitación el primer administrador bajo D17. No hay contraseña en esta variable. |
| Límite local de E2E | `LOGIN_IP_MAX_FAILURES` | Compose lo admite para el límite por IP; `make e2e` lo eleva temporalmente y lo restaura. |

## Verificación y puntos de operación

| Recurso | Dónde verificar |
|---|---|
| Hub y Contabilidad | `http://identityhub.localhost:8080` y `http://contabilidad.localhost:8080` |
| API | `http://localhost:8081/healthz`; `GET /healthz` solo verifica que el proceso vive. |
| Disponibilidad de API | `http://localhost:8081/readyz`; comprueba dependencias y responde 503 si están degradadas. |
| Métricas | API en `/metrics` dentro de la red de Docker; worker en `http://localhost:9091/metrics`. |
| RabbitMQ | `http://localhost:15672` |
| Mailpit | `http://localhost:8025` |

El `healthcheck` del contenedor API consulta `/healthz`; no use esa sonda para inferir que PostgreSQL y RabbitMQ están disponibles. Use `/readyz` para ese diagnóstico.

### Observabilidad y logs

```bash
make up           # ya incluye la observabilidad
make logs S=api
make logs S=worker
```

`make up` activa, junto con la aplicación, Prometheus (`:9090`), Loki (`:3100`), Alloy y Grafana (`:3000`). Consulte métricas en Prometheus, logs de contenedores en Loki y ambas fuentes desde Grafana. `make logs S=<servicio>` sigue el log del servicio seleccionado.

Grafana carga por aprovisionamiento la alerta **DLQ de notificaciones con mensajes** (`deploy/observability/grafana/alerting/dlq.yml`); su procedimiento de atención está en [`../runbooks/dlq-notificaciones.md`](../runbooks/dlq-notificaciones.md).

### Detección en tiempo de ejecución (Falco)

`make up` también levanta **Falco** (AM-020): observa las llamadas al sistema de los contenedores con el driver modern eBPF y evalúa `deploy/observability/falco/rules.d/identity-hub.yaml` sobre `api`, `worker` y `web` (shell, escritura en directorios del sistema, gestor de paquetes o descargador, conexión saliente a una IP pública). Escribe cada alerta como JSON en su salida estándar; Alloy la envía a Loki con las etiquetas `rule` y `priority`, el tablero «Identity Hub — Seguridad» la muestra y Grafana aprovisiona la alerta **Falco detectó actividad anómala en un contenedor** (`deploy/observability/grafana/alerting/falco.yml`). Atención y triaje en [`../runbooks/falco-alerta.md`](../runbooks/falco-alerta.md).

```bash
make falco-test   # dispara eventos inofensivos y dice dónde ver la alerta
```

Falco corre sin `privileged`, solo con las capacidades `BPF`, `PERFMON`, `SYS_RESOURCE` y `SYS_PTRACE`, y monta `/proc` y el socket de Docker en solo lectura. Es un control del stack desplegado: **no forma parte del CI** (los runners de GitHub no ofrecen eBPF equivalente) y requiere un kernel Linux con BTF (Docker Desktop o Linux moderno). Sin él, `make up` termina bien y el resto del stack funciona; solo `falco` se reinicia (revise `make ps` y `make logs S=falco`). `make up-app` levanta la aplicación sin observabilidad ni Falco y es lo que usa CI. `make down` detiene todo conservando los volúmenes; `make clean` también los borra. La imagen está fijada por digest (0.45.0).

### Backups, restauración y rotación

No existe un procedimiento versionado de backup/restauración ni de rotación de claves de firma. No los ejecute por suposición. El compose de producción de referencia (`deploy/docker-compose.prod.yml`) existe, pero no hay gestión de secretos de producción. Las imágenes se publican y firman en Docker Hub con `release.yml` (tarea C3).

## Operaciones comunes

### Crear el primer administrador

Defina `BOOTSTRAP_ADMIN_EMAIL` en el archivo `.env` antes del arranque. Según D17, si no existe ningún administrador, la API crea una cuenta pendiente con rol `admin` y envía su invitación por el flujo normal a Mailpit. Si la invitación pendiente ya venció, el siguiente arranque la reemite; no se configura ni se registra una contraseña inicial.

### Desbloquear una cuenta

No hay un comando administrativo de desbloqueo documentado. D13 establece que completar el restablecimiento de contraseña lleva una cuenta `locked` a `active`, ignora los fallos previos y deja auditoría. No reactiva cuentas deshabilitadas ni pendientes.

## Troubleshooting

| Síntoma | Causa documentada | Acción segura |
|---|---|---|
| E2E o DAST local termina con 423 o bloqueo por IP | Las pruebas fallan logins desde una sola IP; los eventos de auditoría son append-only y al restaurar el límite puede durar hasta 15 minutos. | Ejecute `make e2e` o `make scan-dast`, no Playwright/ZAP directo; el wrapper eleva solo el límite IP y `scripts/ip-lockout-warning.sh` informa cuándo baja. Espere el vencimiento; no borre auditoría. |
| Nginx devuelve 502 después de recrear `api` | Al recrear el contenedor `api` (por ejemplo `docker compose up -d api`) cambia su IP, y Nginx en `web` sigue usando la que resolvió al arrancar. | Compruebe con `make ps` y `/healthz` que la API está sana y reinicie el proxy: `docker restart identity-hub-web-1`. |

## Producción

No despliegue el Compose de desarrollo como producción. La arquitectura de referencia en nube e IaC son entregables de T7 y T8; Checkov se incorpora en T10. Para ensayar localmente una configuración parecida a producción existe la **producción simulada** (T9): `deploy/docker-compose.prod.yml`, un override del Compose base.

### Correo SMTP: STARTTLS y credenciales

El worker no usa `smtp.SendMail`: abre la sesión SMTP explícitamente (EHLO, STARTTLS, AUTH, MAIL, RCPT, DATA, QUIT), con TLS 1.2 como mínimo, verificación del certificado contra `SMTP_HOST`, un plazo de 10 s para conectar y 30 s por sesión.

| Variable | Valores | Efecto |
|---|---|---|
| `SMTP_TLS` | `starttls` (por defecto), `tls`, `none` | `starttls` exige que el servidor ofrezca STARTTLS y falla sin enviar nada si no lo hace; `tls` abre TLS implícito (puerto 465); `none` es texto plano, solo para desarrollo, y la configuración lo rechaza si hay credenciales. |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | ambas o ninguna | AUTH PLAIN, únicamente sobre TLS. La contraseña es un `Secret`: no aparece en logs ni en errores. |
| `SMTP_CA_FILE` | ruta a un PEM | CA adicional de confianza (se suma a las raíces del sistema). Un archivo ilegible o sin certificados impide arrancar el worker. |

**Desarrollo local.** Mailpit corre con `MP_SMTP_REQUIRE_STARTTLS`, certificado propio y `MP_SMTP_AUTH_FILE`. Los certificados se generan con:

```bash
make certs    # sh scripts/gen-smtp-certs.sh --env-file .env
```

Crea `deploy/certs/` (ignorado por git): `ca.pem` y `ca-key.pem` (CA local), `server.pem` y `server-key.pem` (SAN `mailpit`, `localhost`, `127.0.0.1`; claves en modo 0600) y `smtp-auth.txt` (usuario y contraseña de `.env`, que se reescribe en cada ejecución). Es idempotente; `sh scripts/gen-smtp-certs.sh --force` regenera la CA y los certificados. `make up`, `make up-app` y `make restart` la ejecutan antes de levantar el stack, y la acción `stack-up` de CI también (vía `make up-app`, sin observabilidad). El worker monta solo `ca.pem` y confía en ella con `SMTP_CA_FILE`; un `.env` anterior sin `SMTP_PASSWORD` debe añadirla (`openssl rand -hex 24`).

**Producción con Amazon SES.** Use el endpoint SMTP regional de SES (`email-smtp.<región>.amazonaws.com`) en el puerto 587 con `SMTP_TLS=starttls`; no defina `SMTP_CA_FILE` (los certificados de SES los firma una CA pública). `SMTP_USERNAME` y `SMTP_PASSWORD` son las credenciales SMTP de SES, que no son la clave de acceso IAM: se derivan de ella (`aws_iam_access_key.ses_smtp_password_v4` en Terraform). La referencia de `infraestructura/terraform` las crea para un usuario IAM con solo `ses:SendRawEmail`, las guarda en Secrets Manager (cifradas con KMS) y las inyecta únicamente al worker. El remitente `SMTP_FROM` debe pertenecer a una identidad verificada en SES y, mientras la cuenta esté en el sandbox, los destinatarios también.

### Hosts configurables y fijos

Los clientes OAuth ya no se configuran con variables de entorno: la API los lee de la tabla `applications` una sola vez al arrancar (`client_id`, `redirect_uri` y `allowed_origin` de cada fila; ver `docs/manuales/integracion-terceros.md`, sección 2.3). Para registrar otra aplicación hay que agregar una migración y reiniciar la API. Al arrancar, la API se niega a iniciar si no hay ninguna aplicación o si alguna tiene una redirect URI relativa, con fragmento o que no sea http(s), un origen con ruta o un origen que no coincida con el de la redirect URI.

Siguen fijos, y hay que cambiarlos en conjunto para otro dominio, porque no son valores de ejecución sino de compilación o de configuración de Nginx:

- `frontend/nginx/default.conf`: los `server_name` `identityhub.localhost`, `contabilidad.localhost` e `inventario.localhost` y el `connect-src` de la CSP de cada aplicación.
- `contabilidad/src/config.ts` e `inventario/src/config.ts`: `HUB_ORIGIN`, `CLIENT_ID` y `AUDIENCE`, constantes de compilación acopladas a esa CSP.
- Las filas de `applications` sembradas por las migraciones 000007 (Contabilidad) y 000011 (Inventario): sus URL son las locales.
- Los valores por defecto de `PUBLIC_BASE_URL` y `JWT_ISSUER` (estos dos sí se pueden sobrescribir por entorno).

Hacerlos configurables exige plantillas de Nginx (`envsubst`) y argumentos de compilación del frontend; se dejó fuera para no cambiar la superficie de seguridad (CSP) sin una prueba de extremo a extremo con otro dominio.

### Producción simulada

```bash
make up-prod     # docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml up -d --no-build
make down-prod   # detiene el stack conservando los volúmenes
```

Variables obligatorias en `.env`, además de las del desarrollo (el comando falla con un mensaje en español si falta alguna):

| Variable | Contenido |
|---|---|
| `IDENTITY_HUB_API_IMAGE`, `IDENTITY_HUB_WORKER_IMAGE`, `IDENTITY_HUB_WEB_IMAGE` | Digest de cada imagen publicada en Docker Hub (`nombre@sha256:...`). Las publica `release.yml` en `docker.io/jorgeivp/identity-hub-api`, `identity-hub-worker` e `identity-hub-web` con cada tag `vX.Y.Z` (validado con `v0.9.0` y `v0.9.1`); solo después de firmar, atestar y verificar (`cosign verify` y `cosign verify-attestation --type cyclonedx` con la identidad del workflow) mueve `latest` al digest firmado, y únicamente para el `vX.Y.Z` más alto. Los pasos de red se reintentan hasta tres veces y los artefactos de imagen se retienen 7 días. Use el digest de la publicación, que figura en el resumen del run. |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` | Relay SMTP y remitente de producción. |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | Credenciales del relay (con SES, las credenciales SMTP derivadas de una clave IAM). El worker las presenta solo tras STARTTLS. |

Qué cambia frente a desarrollo:

- Solo `web` publica un puerto (8080). `db`, `broker`, `api`, `worker` y, con el perfil `observability`, Prometheus, Loki y Grafana quedan solo en la red interna de Compose (AM-014).
- `api`, `worker` y `web` usan las imágenes por digest, sin `build`; `LOG_LEVEL` pasa a `info`.
- Mailpit no arranca (perfil `correo-local`); el worker ya no depende de él.
- `restart: unless-stopped`, rotación de logs (`json-file`, 10 MB x 3) y límites de memoria y CPU.
- Se conserva el endurecimiento del base: `read_only`, `cap_drop: ALL`, `no-new-privileges` y `tmpfs`.

Límites conocidos:

- Los secretos siguen llegando como variables de entorno desde `.env`: la API no soporta variables `*_FILE`, así que no hay secretos por archivo.
- El worker autentica por SMTP y exige STARTTLS (`SMTP_TLS=starttls`, puerto 587). La CA local de desarrollo no se monta aquí: el relay se verifica con las raíces del sistema.
- Las imágenes ya están publicadas en Docker Hub; sin tener a mano sus digests, la validación sin arrancar se limita a `docker compose ... config`.
- El texto que Docker Hub muestra para cada imagen (etiquetas, digests y verificación de la firma) vive en [`deploy/dockerhub/`](../../deploy/dockerhub/README.md) y un workflow lo sincroniza al cambiar en `main`.
