# Manual de despliegue y operación

Este manual describe el entorno local actual con Docker Compose. No afirma soporte de producción: la referencia cloud y `docker-compose.prod.yml` se incorporan en las tareas T7–T9 de la semana 4.

## Inicio rápido

### Prerrequisitos

- Docker y Docker Compose para levantar los contenedores.
- `make` para usar los objetivos del repositorio.
- Un `.env` local; `make setup` lo genera con secretos aleatorios a partir de `.env.example` y no se versiona.

```bash
make setup
docker compose up -d   # equivalente: make up (construye las imágenes)
make ps
```

`make setup` no sobrescribe un `.env` existente. Regenerarlo (`python3 scripts/setup_env.py --force`) cambia las contraseñas, pero Postgres y RabbitMQ conservan las de sus volúmenes: después hay que ejecutar `make clean`, que **borra los datos**, o el stack no podrá autenticarse. `docker compose up -d` (raíz) y `make up` construyen y levantan el perfil ordinario. Use `make down` para detenerlo conservando volúmenes y `make clean` únicamente si desea detenerlo **y borrar los volúmenes**. Para reconstruir un servicio concreto, use `make restart S=api`; para seguir su salida, `make logs S=api`.

## Configuración por entorno

Complete `.env` sin guardar secretos en Git. Las variables que se muestran a continuación provienen de [`.env.example`](../../.env.example); la API también recibe algunos valores fijados por el Compose local.

| Grupo | Variables | Uso |
|---|---|---|
| PostgreSQL | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | Usuario, contraseña y base del contenedor PostgreSQL. |
| Rol de aplicación y migraciones | `IDENTITY_APP_PASSWORD`, `MIGRATE_DATABASE_URL`, `DATABASE_URL` | Credencial de mínimo privilegio, conexión del migrador y conexión de la API. El worker no recibe `DATABASE_URL`: solo consume la cola y envía SMTP. Al desplegar en AWS, publique primero la imagen del worker que ya usa `LoadWorker` y después aplique el Terraform que le quita el secreto y el acceso a la base; una imagen anterior exige `DATABASE_URL` y no arrancaría. |
| RabbitMQ | `RABBITMQ_DEFAULT_USER`, `RABBITMQ_DEFAULT_PASS`, `RABBITMQ_URL` | Usuario, contraseña y URL AMQP del broker. |
| API y JWT | `API_PORT`, `API_BASE_URL`, `LOG_LEVEL`, `JWT_SIGNING_KEY`, `JWT_ISSUER`, `TRUSTED_PROXIES`, `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL` | Puerto, URL local, nivel de log, firma y parámetros de token; `JWT_SIGNING_KEY` no tiene valor por defecto. |
| Correo local | `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` | Destino SMTP para el worker; por defecto apunta a Mailpit. |
| Observabilidad | `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD` | Acceso inicial de Grafana cuando se habilita su perfil. |
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
make up-obs
make logs S=api
make logs S=worker
```

`make up-obs` activa Prometheus (`:9090`), Loki (`:3100`), Alloy y Grafana (`:3000`). Consulte métricas en Prometheus, logs de contenedores en Loki y ambas fuentes desde Grafana. `make logs S=<servicio>` sigue el log del servicio seleccionado.

Grafana carga por aprovisionamiento la alerta **DLQ de notificaciones con mensajes** (`deploy/observability/grafana/alerting/dlq.yml`); su procedimiento de atención está en [`../runbooks/dlq-notificaciones.md`](../runbooks/dlq-notificaciones.md).

### Backups, restauración y rotación

No existe un procedimiento versionado de backup/restauración ni de rotación de claves de firma. No los ejecute por suposición. El compose de producción y sus secretos gestionados no existen todavía; corresponden a T7–T9. La publicación y firma de imágenes con Docker Hub llegan en T17.

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
| `go build` informa símbolos duplicados con nombres terminados en ` 2` | iCloud puede crear copias sin seguimiento, por ejemplo `gen 2.go`. | Liste con `find . -path ./.git -prune -o -path '*/node_modules' -prune -o -name "* 2*" -print`; compare cada copia con `cmp` y elimine solo las idénticas, informando siempre qué se retiró. |

## Producción

No despliegue el Compose de desarrollo como producción. La arquitectura de referencia en nube e IaC son entregables de T7 y T8; Checkov se incorpora en T10. Para ensayar localmente una configuración parecida a producción existe la **producción simulada** (T9): `deploy/docker-compose.prod.yml`, un override del Compose base.

### Producción simulada

```bash
make up-prod     # docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml up -d --no-build
make down-prod   # detiene el stack conservando los volúmenes
```

Variables obligatorias en `.env`, además de las del desarrollo (el comando falla con un mensaje en español si falta alguna):

| Variable | Contenido |
|---|---|
| `IDENTITY_HUB_API_IMAGE`, `IDENTITY_HUB_WORKER_IMAGE`, `IDENTITY_HUB_WEB_IMAGE` | Digest de cada imagen publicada en Docker Hub (`nombre@sha256:...`). Las publica T17; hasta entonces no hay imágenes que referenciar. |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` | Relay SMTP y remitente de producción. |

Qué cambia frente a desarrollo:

- Solo `web` publica un puerto (8080). `db`, `broker`, `api`, `worker` y, con el perfil `observability`, Prometheus, Loki y Grafana quedan solo en la red interna de Compose (AM-014).
- `api`, `worker` y `web` usan las imágenes por digest, sin `build`; `LOG_LEVEL` pasa a `info`.
- Mailpit no arranca (perfil `correo-local`); el worker ya no depende de él.
- `restart: unless-stopped`, rotación de logs (`json-file`, 10 MB x 3) y límites de memoria y CPU.
- Se conserva el endurecimiento del base: `read_only`, `cap_drop: ALL`, `no-new-privileges` y `tmpfs`.

Límites conocidos:

- Los secretos siguen llegando como variables de entorno desde `.env`: la API no soporta variables `*_FILE`, así que no hay secretos por archivo.
- El worker envía SMTP sin autenticación ni TLS (`backend/cmd/worker/main.go:198-199`); un relay real exige un cambio de código (misma brecha que el ADR 0012).
- Hasta T17 no hay imágenes publicadas, de modo que la validación sin arrancar se limita a `docker compose ... config`.
