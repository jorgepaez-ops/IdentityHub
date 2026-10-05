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
| Rol de aplicación y migraciones | `IDENTITY_APP_PASSWORD`, `MIGRATE_DATABASE_URL`, `DATABASE_URL` | Credencial de mínimo privilegio, conexión del migrador y conexión de API/worker. |
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

No despliegue este Compose de desarrollo como producción. La arquitectura de referencia en nube, IaC y `docker-compose.prod.yml` son entregables pendientes de T7, T8 y T9; Checkov se incorpora en T10.
