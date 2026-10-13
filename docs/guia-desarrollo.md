# Guía de desarrollo local

Complementa el README (arquitectura y `make up`). Esto es lo operativo: cómo
correr el backend fuera de Docker, qué hace `make gen` por dentro, cómo correr
las pruebas de Go a mano y cómo configurar GoLand.

## 1. Dos formas de correr el proyecto

**Todo en Docker (`make up`)** — la forma "de verdad" (CI usa `make up-app`, la misma aplicación sin observabilidad). Primero
ejecuta `make certs` (CA y certificado SMTP locales en `deploy/certs/`, idempotente;
Mailpit exige STARTTLS y autenticación) y luego levanta `db`, `broker`, `mailpit`,
`migrate`, `api`, `worker` y `web`. También levanta la observabilidad
(Prometheus, Loki, Alloy, Grafana y Falco); `make up-app` levanta solo la aplicación, como CI. Un `.env` generado antes de F2
necesita `SMTP_PASSWORD` (`openssl rand -hex 24`), o el compose no interpola. Útil para probar
el sistema completo, no para iterar rápido en Go: cada cambio exige
`make restart S=api`, que reconstruye la imagen.

**Solo Postgres en Docker, Go en el host** — la forma rápida para trabajar en
`backend/`, y la que usa Claude para verificar tareas contra una base real:

```bash
# desde la raíz del repositorio, con el .env generado por `make setup`
docker compose --env-file .env -f deploy/docker-compose.yml up -d db
# espera a que quede "healthy":
docker inspect --format='{{.State.Health.Status}}' identity-hub-db-1

# aplica las migraciones (usa el servicio `migrate` del compose, no un binario local):
docker compose --env-file .env -f deploy/docker-compose.yml run --rm migrate

cd backend && go build ./...
```

Hay que pasar `--env-file .env` desde la raíz: el compose interpola **todas** las variables
obligatorias del archivo (incluida `SMTP_PASSWORD`) aunque solo se levante `db`, y desde
`deploy/` no encuentra el `.env` de la raíz y falla con «must be set in the repository root .env».

Con la base así levantada, `go test`, `go run` y el depurador de GoLand
hablan directo con PostgreSQL en `localhost:5432` sin pasar por Nginx ni por
el contenedor de la API.

### Primer arranque y `.env` anterior a F2

1. Clon nuevo: `make setup` crea `.env` desde `.env.example` con secretos aleatorios (incluida `SMTP_PASSWORD`; `scripts/setup_env.py`) y `make up` levanta el stack. `make up` ejecuta `make certs` primero, así que no hay que correrlo a mano. `make up` incluye la observabilidad y Falco; `make up-app` solo la aplicación.
2. `.env` creado antes de F2: `make setup` no modifica un `.env` existente, por lo que nunca agrega `SMTP_PASSWORD`. Agregue solo esa línea:

   ```bash
   grep -q '^SMTP_PASSWORD=' .env || printf '\nSMTP_PASSWORD=%s\n' "$(openssl rand -hex 24)" >> .env
   ```

3. No use `python3 scripts/setup_env.py --force` sobre un entorno existente: regenera todos los secretos, pero PostgreSQL y RabbitMQ conservan las contraseñas antiguas en sus volúmenes y el stack deja de autenticarse (habría que usar `make clean`, que borra los datos).

Problema frecuente: `make certs` o `make up` terminan con `error: falta SMTP_PASSWORD en .env`. Es el caso 2: agregue la línea de arriba y repita `make up`.

### Levantar el stack

El flujo completo son dos comandos: `make setup` (solo en un clon nuevo; no sobrescribe un `.env` existente) y `make up`. No hay un segundo comando para la observabilidad.

`make up` ejecuta antes `make certs` (CA y certificado SMTP locales, idempotente), construye las imágenes y levanta **todo** el stack:

| Grupo | Servicios | Puertos publicados en el host |
|---|---|---|
| Aplicación | `db` (PostgreSQL), `broker` (RabbitMQ), `mailpit`, `migrate` (corre y termina), `api`, `worker`, `web` (Nginx) | `web` 8080 · `api` 8081 · `worker` 9091 (métricas) · `db` 5432 · `broker` 5672 y 15672 · `mailpit` 1025 y 8025 |
| Observabilidad | `prometheus`, `loki`, `alloy`, `grafana`, `falco` | `prometheus` 9090 · `loki` 3100 · `grafana` 3000 · `alloy` y `falco` no publican puertos |

Al terminar imprime estas direcciones:

| Servicio | Dirección |
|---|---|
| Hub | <http://identityhub.localhost:8080> (<http://localhost:8080> también sirve el Hub) |
| Contabilidad | <http://contabilidad.localhost:8080> |
| API | <http://localhost:8081/healthz> |
| RabbitMQ | <http://localhost:15672> |
| Mailpit | <http://localhost:8025> |
| Grafana | <http://localhost:3000> |
| Prometheus | <http://localhost:9090> |

Las credenciales de RabbitMQ y Grafana salen de `.env` (`RABBITMQ_DEFAULT_USER`/`RABBITMQ_DEFAULT_PASS` y `GRAFANA_ADMIN_USER`/`GRAFANA_ADMIN_PASSWORD`, generadas por `make setup`); no se imprimen ni se versionan.

Grafana (3000), Prometheus (9090) y Loki (3100) solo escuchan en `127.0.0.1`: Prometheus y Loki no tienen autenticación, así que no se exponen a la red local.

En un equipo donde `localhost` resuelve a `::1` (IPv6) esos tres puertos no responden, porque solo escuchan en IPv4: use `127.0.0.1` en el navegador, en `curl` y en los scripts (por ejemplo `curl http://127.0.0.1:9090/-/ready`). Una prueba (`scripts/compose_ports_test.py`, parte de `make spec-drift` y del job 1 de CI) falla si alguno de los tres publica un puerto sin `127.0.0.1`.

Tiempos de espera: la aplicación queda lista cuando `api` y `web` aparecen `healthy` en `make ps` (unos 30 a 60 s tras construir). Grafana responde en <http://localhost:3000/api/health> a los pocos segundos; sus paneles con datos tardan 1 a 2 minutos porque Prometheus raspa cada 15 s y Alloy descubre contenedores nuevos cada ~60 s. Una alerta de Falco tarda también 1 a 2 minutos en verse en Loki y Grafana (`make falco-test` la dispara).

- **Solo la aplicación:** `make up-app` levanta únicamente los servicios del grupo Aplicación, sin Prometheus, Loki, Alloy, Grafana ni Falco. Es lo que usa CI (acción compuesta `stack-up`, jobs E2E y DAST) y sirve en un equipo con pocos recursos.
- **Detener:** `make down` detiene todos los contenedores, observabilidad incluida, y **conserva los volúmenes** (`pgdata`, `rabbitdata`, `grafanadata`). `make clean` hace lo mismo y además **borra los volúmenes**: se pierden los datos y las contraseñas guardadas de Postgres y RabbitMQ.
- **Estado y logs:** `make ps` lista todos los servicios, observabilidad incluida; `make logs S=<servicio>` sigue el log de uno (por ejemplo `make logs S=falco`).
- **Falco y el kernel:** Falco usa el driver modern eBPF y necesita un kernel Linux con BTF (Linux moderno o Docker Desktop). Sin él, `make up` termina bien, la aplicación y el resto de la observabilidad funcionan y solo `falco` falla y se reinicia (`restart: unless-stopped`; nada depende de él). Se ve en `make ps` (estado `Restarting` o `Exited`) y la causa en `make logs S=falco`. Falco no corre en CI.

Problemas frecuentes:

1. `error: falta SMTP_PASSWORD en .env` (o `falta .env`): el `.env` es anterior a F2 o no existe. Ejecute `make setup` en un clon nuevo; en uno existente agregue solo la línea `SMTP_PASSWORD` como se explica arriba, y repita `make up`.
2. Puerto ocupado (`port is already allocated`, típico de 8080, 5432 o 3000): detenga el proceso que lo usa (`lsof -i :8080`) o un stack anterior (`make down`) y repita `make up`.
3. `falco` en `Restarting`: revise `make logs S=falco`. Si el mensaje habla de BTF, eBPF o del driver, el kernel no lo soporta: el resto del stack funciona y puede ignorarlo, o usar `make up-app` para no verlo.

## 2. `make gen`, paso a paso

```makefile
gen:
	cd backend && go generate ./internal/api   # 1
	sqlc generate                              # 2
	cd frontend && npm run gen:api             # 3
	python3 scripts/traceability.py            # 4
```

`specs/` es la fuente de verdad (ver README, "Metodología"); estos cuatro
pasos son lo que la hace cumplible en vez de aspiracional:

1. **`go generate ./internal/api`** — dispara una directiva `//go:generate`
   dentro de `backend/internal/api` que corre `oapi-codegen` sobre
   `specs/03-api/openapi.yaml` y escribe `gen.go`: la interfaz `ServerInterface`
   (un método Go por `operationId` del OpenAPI) y todos los tipos de
   request/response (`ListUsersParams`, `TokenPair`, etc.). **Nunca se edita
   `gen.go` a mano** — cualquier cambio se pierde en el siguiente `make gen`.
2. **`sqlc generate`** (usa `sqlc.yaml` en la raíz, versión fijada en el
   `Makefile` — hoy `v1.31.1`) — lee cada archivo de `db/queries/*.sql` y
   genera su código Go equivalente en `backend/internal/store/internal/sqlc/`:
   un struct de parámetros y una función por cada `-- name: Algo :one|:many|:exec`.
   Tampoco se edita a mano. Los adaptadores que sí se escriben a mano viven un
   nivel arriba, en `backend/internal/store/*.go` (por ejemplo
   `admin_users.go`), y llaman a las funciones generadas.
3. **`npm run gen:api`** (dentro de `frontend/`) — corre `openapi-typescript`
   sobre el mismo `openapi.yaml` y escribe `frontend/src/api/schema.d.ts`: los
   tipos TypeScript del cliente, generados del mismo contrato que usa el
   backend, para que ninguno de los dos se desalinee del spec por separado.
4. **`scripts/traceability.py`** — recorre `specs/06-acceptance/*.feature` y
   los archivos de prueba de Go (`backend/**/*_test.go`) buscando funciones
   `Test(RF|RNF)NNN_...`, y regenera `specs/07-traceability.md`: para cada
   requisito, cuántos escenarios Gherkin y cuántas pruebas Go existen, y si
   eso alcanza para marcarlo "completo", "parcial" o "sin cubrir". Por eso
   **el nombre de la prueba importa**: `TestListaUsuarios` no cuenta para
   nada; `TestRF010_ListaUsuarios` sí.

`python3 scripts/traceability.py --check` (sin `make gen` completo) solo
valida que el archivo ya commiteado siga coincidiendo con el código —
es lo que corre en CI en el job `spec-drift`, y falla la build si alguien
edita `gen.go`, `schema.d.ts` o el código de sqlc a mano y no vuelve a generar.

**Cuándo correrlo:** después de tocar `specs/03-api/openapi.yaml` (solo
autorizado en tareas puntuales, ver `AGENTS.md`), después de agregar o
cambiar una consulta en `db/queries/*.sql`, o después de agregar una prueba
nueva con el patrón `TestRFnnn_`/`TestRNFnnn_`. Verificación estándar:
`make gen && git diff --exit-code` — si hay diferencia, algo quedó sin
regenerar antes del commit.

## 3. Pruebas de Go a mano

Todo esto corre desde `backend/`.

```bash
# suite completa, con detector de carreras (lo mismo que make test-go, sin cobertura)
go test -race ./...

# un paquete puntual
go test -race ./internal/auth/admin/...

# una prueba puntual, con salida detallada
go test -race -run TestRF010_NoSeDejaElSistemaSinAdmin ./internal/auth/admin/... -v

# forzar que no reuse resultados cacheados (importante después de tocar SQL
# generado o de tocar el reloj inyectable de un test)
go test -race -count=1 ./...

# cobertura, igual que `make test-go`
go test -race -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -func=coverage.out | tail -1     # resumen
go tool cover -html=coverage.out               # abre el detalle en el navegador
```

### Pruebas de integración (build tag `integration`)

Los archivos con `//go:build integration` en la primera línea (por ejemplo
`internal/auth/auditlog/audit_log_integration_test.go`) **no compilan** en
una corrida normal — hace falta el tag, y una base real:

```bash
export TEST_DATABASE_URL="postgres://identity:postgres_admin_2024@localhost:5432/identity?sslmode=disable"
go test -race -tags=integration ./...
```

Sin `TEST_DATABASE_URL`, `backend/internal/testdb.New(t)` llama a `t.Skip(...)`
y esas pruebas quedan como `ok` **por saltadas, no por haber pasado** — un
`go test` en verde con la base caída no significa nada. `make test-integration`
corre exactamente ese mismo comando, con la advertencia de que hoy no exporta
`TEST_DATABASE_URL` por sí solo: hay que levantar la base primero (sección 1)
y exportar la variable en la misma shell.

`testdb.New` crea una base nueva por prueba (nombre aleatorio) contra el
servidor que apunte `TEST_DATABASE_URL`, aplica todas las migraciones de
`db/migrations/` y la borra al terminar — por eso alcanza con un solo
Postgres corriendo para toda la suite.

### Convención de nombres

`scripts/traceability.py` cuenta exactamente `func Test(RF|RNF)NNN_Descripcion`,
ASCII, sin acentos ni ñ. Una prueba real sin ese prefijo compila y pasa igual,
pero **no cuenta** en `specs/07-traceability.md` — es el error más común al
delegar una tarea y el que más veces corrigió el hook de revisión antes del
commit (ver `odd/tasks/idp-semana-2.md`, notas de T19).

### Pruebas E2E

Las pruebas de extremo a extremo (Playwright, solo Chromium por ahora) viven en `e2e/` y corren
contra el stack ya levantado.

```bash
make up      # el stack debe estar sano antes
make e2e     # npm ci + npx playwright test
```

- Requieren Node.js y los navegadores de Playwright (`npx playwright install chromium`, una vez).
- Las pruebas crean sus propias cuentas `e2e-*@example.test` en la base local (un administrador
  sembrado por SQL con `docker exec` y las invitaciones que cada escenario necesita); no tocan
  las demás cuentas y tampoco las borran. Al terminar, `e2e/support/global-teardown.ts` pasa a
  `disabled` todas las cuentas `e2e-%@example.test` (solo ese patrón, con parámetros, contra el
  contenedor de `E2E_DB_CONTAINER`), para que los administradores sembrados con la contraseña
  del repositorio no queden activos. Las filas no se eliminan (`audit_log` las referencia).
- Se ejecutan en serie porque Nginx limita `/api/v1/auth/` a 5 peticiones por segundo.
- Todas las peticiones salen de la misma IP y varios escenarios fallan logins a propósito. Por eso
  `make e2e` recrea la API con `LOGIN_IP_MAX_FAILURES=1000` (variable `E2E_LOGIN_IP_MAX_FAILURES`)
  mientras corre la suite y la restaura al terminar, pase o falle (variable
  `LOCAL_TEST_LOGIN_IP_MAX_FAILURES`; `E2E_LOGIN_IP_MAX_FAILURES` sigue valiendo como alias). El
  límite por cuenta no se relaja. La lógica compartida con `make scan-dast` vive en
  `scripts/with-raised-login-limit.sh`. Si se corre `npx playwright test` directamente, la segunda corrida dentro de
  `LOGIN_FAILURE_WINDOW` (15 min) choca con el límite por IP; el chequeo previo lo avisa.
- Al terminar, `scripts/ip-lockout-warning.sh` consulta `audit_log` (solo lectura) y, si alguna IP
  tiene 20 o más `login_failed`/`mfa_code_rejected` en los últimos 15 min, imprime un aviso con la
  IP, el conteo y la hora local aproximada en que bajará del límite. Esos fallos no se pueden borrar
  (`audit_log` es append-only), así que hasta entonces tu navegador puede recibir 423. El aviso
  nunca cambia el código de salida.
- Variables opcionales: `E2E_HUB_URL`, `E2E_MAILPIT_URL`, `E2E_DB_CONTAINER` (por defecto
  `identity-hub-db-1`).
- CI corre esta misma suite en el job `7 · E2E (Playwright)`: genera un `.env` desechable, levanta el
  stack con `make up`, instala Chromium y ejecuta `make e2e`; si falla, sube `e2e/playwright-report`
  y `e2e/test-results` como artefactos.

### Prueba de carga (RNF-010)

```bash
make up-app      # o make up; el stack debe estar sano antes
make load-test   # k6 en un contenedor (imagen fijada por digest); falla si el p95 supera 300 ms
```

- Mide `POST /api/v1/auth/refresh` (una sesión que rota su cookie, ~3 r/s), `GET /api/v1/me` y
  `GET /.well-known/jwks.json` (50 r/s cada uno, 20 s). El login con Argon2id queda fuera de la
  medición: se hace una sola vez en `setup()` y el requisito lo excluye.
- `scripts/load-test.sh` siembra una cuenta `load-*@example.test` desechable con una invitación
  pendiente (como las pruebas E2E) y k6 la acepta, inicia sesión y lee el código MFA en Mailpit. La
  contraseña y el token de invitación se pasan a k6 por nombre de variable (no en la línea de comandos).
  `make load-test` corre bajo `scripts/with-raised-login-limit.sh` (como `make e2e`): tras la suite E2E los
  logins fallidos pueden seguir dentro de la ventana de RF-017 y bloquear la IP del login de `setup()`.
  Al terminar, la cuenta queda `disabled` (no se borra: `audit_log` es append-only y sus filas
  referencian a `users`). El script se niega a correr contra `docker-compose.prod.yml`.
- Los umbrales viven en `tests/load/auth-latency.js` (`P95_MS`, por defecto 300; `DURATION`, por defecto
  20s) y hacen que k6 termine con código distinto de cero. La imagen se cambia con `K6_IMAGE`.
- CI lo corre en el job `7 · E2E (Playwright)`, después de `make e2e` y sobre el mismo stack.

### Verificar los requisitos no funcionales

Tres comandos, de menor a mayor costo; los tres corren también en CI:

```bash
make spec-drift   # sin stack ni red salvo docker compose config; trazabilidad y controles del pipeline
make e2e          # con el stack levantado; incluye las pruebas RNF-007 y RNF-012
make load-test    # con el stack levantado; RNF-010
```

- `make spec-drift` ejecuta `scripts/traceability_test.py`, `scripts/pipeline_controls_test.py` (RNF-001,
  RNF-002, RNF-006 y RNF-008 contra el compose renderizado, los Dockerfile y los workflows) y
  `scripts/traceability.py --check`, que además comprueba que cada escenario `@RNF` apunta con
  `# Verificación:` a una prueba que existe.
- `make e2e` cubre RF-008 (perfil propio), RNF-007 (métricas Prometheus, logs JSON y tablero) y RNF-012 (ningún
  secreto en los logs de api ni worker).
- `make load-test` cubre RNF-010 y falla si el p95 de refresh, `/me` o JWKS supera 300 ms.
- Qué prueba verifica qué, y cómo se comprobó que falla, está en la tabla «Evidencia de los requisitos no
  funcionales» de la fase de pruebas del informe técnico.

### DAST con ZAP

`make scan-dast` corre OWASP ZAP (imagen fijada en `ZAP_IMAGE`) contra el stack ya levantado: baseline
del Hub (`identityhub.localhost:8080`), baseline de Contabilidad y escaneo de la API con
`specs/03-api/openapi.yaml`. Requiere Docker y Python 3.

- `scripts/zap-gate.py` es el único punto de decisión: rompe la build (código de salida 1) con
  cualquier alerta de riesgo **medio o alto** de los informes JSON. Los bajos e informativos se
  imprimen pero no rompen. Solo una entrada `IGNORE` con justificación en `.zap/rules.tsv` suprime
  una alerta media o alta; `WARN` documenta bajos aceptados.
- Debe terminar en verde. La CSP sin `form-action` (10055) lo rompía hasta VULN-030; si vuelve a
  aparecer un hallazgo medio o alto, el gate rompe la build.
- El escaneo de API golpea `/api/v1/auth/` desde una sola IP, así que `make scan-dast` sube
  `LOGIN_IP_MAX_FAILURES` en la API mientras corre y la restaura al terminar (como `make e2e`,
  con el mismo script y el mismo aviso de bloqueo por IP al final).
- Los informes (JSON y HTML) quedan en `security/zap-reports/` (ignorado por git). CI corre lo
  mismo en el job `11 · DAST (OWASP ZAP)` y sube esa carpeta como artefacto.
- Pruebas del gate: `python3 scripts/zap_gate_test.py` (también corre en `make spec-drift` y en el
  job `1 · Deriva entre specs y código`).

## 4. GoLand

El repo no trae configuraciones de ejecución compartidas (`.idea/` está fuera
de control de versiones); esto es lo mínimo para armar las propias.

**Abrir el proyecto:** abrir la carpeta `backend/` como raíz del módulo Go
(no la raíz del repo) para que GoLand resuelva `go.mod` sin configuración
extra. Si se abre la raíz del repo completo, marcar `backend/` como
"Go Modules content root" en *Settings → Go → GOPATH*.

**Run/Debug Configuration para la API** (*Run → Edit Configurations → + → Go
Build*):
- Run kind: `Directory`, Directory: `backend/cmd/api`.
- Environment: pegar como mínimo `DATABASE_URL`, `RABBITMQ_URL`,
  `JWT_SIGNING_KEY` (ver tabla abajo) — GoLand no lee `.env` solo; instalar el
  plugin **EnvFile** para apuntarlo a un `.env` local, o pegar las variables
  a mano en el campo "Environment variables".
- Con esto, breakpoints en `internal/api/*.go` o `internal/auth/**` se
  detienen de verdad al pegarle una petición con `curl` o desde el frontend.

**Run/Debug Configuration para un paquete de pruebas** (*+ → Go Test →
Directory* o *Package*):
- Test kind: `Directory`, apuntando a por ejemplo `internal/auth/login`.
- Program arguments: `-race` (y `-tags=integration` si es una prueba de
  integración — GoLand no conoce el tag por defecto, hay que agregarlo).
- Environment: `TEST_DATABASE_URL` para las de integración.
- El ícono ▶️ junto a cada `func TestXxx` en el editor genera esta misma
  configuración al vuelo para esa prueba puntual, sin crearla a mano.

**Antes de cada commit:** *Settings → Tools → File Watchers* con `gofmt -l`
sobre `backend/` avisa si algo quedó sin formatear — el hook de revisión del
repo ya lo rechaza, pero verlo en el editor ahorra una vuelta.

## 5. Variables de entorno que la API espera hoy

Reflejan `backend/internal/config/config.go` (fuente real; `.env.example`
en la raíz del repo es la plantilla; `make setup` genera el `.env` con secretos aleatorios, incluida `SMTP_PASSWORD`). Obligatorias = el proceso no arranca sin ellas.

| Variable | Obligatoria | Default | Qué es |
|---|---|---|---|
| `DATABASE_URL` | sí (no para el worker) | — | cadena de conexión a PostgreSQL; el worker arranca con `config.LoadWorker`, que no la exige |
| `RABBITMQ_URL` | sí | — | cadena de conexión al broker |
| `JWT_SIGNING_KEY` | sí (no para el worker) | — | semilla Ed25519 de 32 bytes, en base64 (`openssl rand -base64 32`); el worker no la recibe ni la lee |
| `API_PORT` | no | `8081` | puerto HTTP de la API |
| `LOG_LEVEL` | no | `info` | |
| `APP_VERSION` | no | `dev` | versión que reporta el servicio |
| `JWT_ISSUER` | no | `http://identityhub.localhost:8080` | claim `iss` del token |
| `JWT_AUDIENCE` | no | `identity-hub` | claim `aud` |
| `JWT_ACCESS_TTL` | no | `15m` | vigencia del access token |
| `JWT_REFRESH_TTL` | no | `720h` | vigencia del refresh token (cookie) |
| `HUB_SESSION_TTL` | no | `720h` | vigencia de la sesión del Hub (debe ser positiva) |
| `LOGIN_ACCOUNT_MAX_FAILURES` | no | `5` | fallos antes de bloquear la cuenta (RF-017) |
| `LOGIN_IP_MAX_FAILURES` | no | `20` | fallos por IP antes de bloquear (RF-017 / VULN-020) |
| `LOGIN_FAILURE_WINDOW` | no | `15m` | ventana deslizante de los dos contadores |
| `LOGIN_LOCKOUT_DURATION` | no | `15m` | duración del bloqueo de cuenta |
| `TRUSTED_PROXIES` | no | vacío | CIDR separados por coma; sin esto, `X-Forwarded-For` se ignora siempre (T6); solo la API la recibe |
| `ARGON2_MEMORY_KIB` / `_ITERATIONS` / `_PARALLELISM` / `_CONCURRENCY` | no | `65536` / `3` / `2` / `4` | parámetros de Argon2id |
| `SMTP_HOST` / `_PORT` / `_FROM` | no | `mailpit` / `1025` / `no-reply@identity.local` | |
| `SMTP_TLS` | no | `starttls` | `starttls` (exige STARTTLS y falla si el servidor no lo ofrece), `tls` (TLS implícito) o `none` (solo desarrollo; se rechaza con credenciales) |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | no (juntas) | vacío | autenticación SMTP PLAIN, solo tras TLS; la contraseña es `Secret` y nunca se registra |
| `SMTP_CA_FILE` | no | vacío | PEM con la CA que se confía además de las raíces del sistema (en local, la CA de `make certs`) |
| `OAUTH_CLIENT_ID` / `OAUTH_REDIRECT_URI` / `OAUTH_CLIENT_ORIGIN` | no | `contabilidad` / `http://contabilidad.localhost:8080/oauth/callback` / `http://contabilidad.localhost:8080` | cliente OAuth registrado; la redirect URI debe ser absoluta http(s) sin fragmento y el origen, exactamente su esquema y host. Debe coincidir con la fila de `oauth_clients` (migración 000007) |
| `PUBLIC_BASE_URL` | no | `http://identityhub.localhost:8080` | usado para armar enlaces en los correos |
| `BOOTSTRAP_ADMIN_EMAIL` | no | vacío | crea el primer administrador con su invitación y la reemite al arrancar si venció sin aceptarse; debe ser un correo válido si se define |

### Administrador inicial opcional

En una instalación nueva, define `BOOTSTRAP_ADMIN_EMAIL` solo para crear la
primera cuenta administradora pendiente. La API la crea con los roles `user` y
`admin`, envía la invitación habitual y espera que la persona elija su propia
contraseña al aceptarla. No existe una variable de contraseña para este flujo.

El arranque no cambia nada si la variable está vacía, ya hay un
administrador (activo, bloqueado o deshabilitado; solo uno pendiente no cuenta) o el correo
pertenece a otra cuenta. Si el administrador inicial sigue
pendiente y su invitación (24 h) venció sin aceptarse, el arranque siguiente
invalida el enlace anterior y envía uno nuevo (auditoría con actor
`system/bootstrap`); mientras la invitación siga vigente no reenvía nada. Todos
estos casos se registran sin mostrar el correo ni el token. La invitación se publica
después de confirmar la transacción; si la publicación falla, se anula ese enlace, se avisa en
el log y el arranque continúa (la cuenta queda pendiente, sin contraseña): el arranque siguiente
la reemite. Si no se puede comprobar o crear de forma segura,
la API no arranca.

`Load()` acumula **todos** los errores de configuración antes de fallar
(ver el comentario en `config.go`): un solo arranque fallido lista todo lo
que falta, en vez de una variable a la vez.

## 6. Hooks de pre-commit (RNF-003)

`.pre-commit-config.yaml` declara los controles locales que corren antes de cada commit:
gitleaks, `detect-private-key`, gofmt, `go build`, la matriz de trazabilidad y los chequeos
básicos de `pre-commit-hooks`. **No corren solos**: hay que instalarlos una vez por clon.

```bash
# instalar la herramienta (cualquiera de las dos)
uv tool install pre-commit      # o: brew install pre-commit / pipx install pre-commit

# instalar el hook en este clon
pre-commit install
```

- Si ya existe un `.git/hooks/pre-commit` (por ejemplo el de Gentleman Guardian Angel),
  `pre-commit install` lo conserva como `pre-commit.legacy` y lo ejecuta primero (modo
  migración). No usar `-f`, que lo descarta.
- La versión de gitleaks del hook es la misma que usa `make scan-secrets` (v8.24.3), con la misma
  configuración (`.gitleaks.toml`, `.gitleaksignore`).
- Si `.pre-commit-config.yaml` tiene cambios sin agregar al stage, pre-commit rechaza el commit
  hasta que se agreguen.
- Comprobar que funciona: poner en stage un archivo con una clave falsa con forma de AWS
  (`AKIA` + 16 caracteres) e intentar commitear. El hook `Detect hardcoded secrets` debe fallar y
  el commit no se crea. Después, sacar el archivo del stage y borrarlo.
- Ejecutar todos los hooks sobre el repo sin commitear: `pre-commit run --all-files`
  (`trailing-whitespace`, `end-of-file-fixer` y gofmt **modifican** archivos).

## 7. Contribución

Trabaje en una rama de funcionalidad; `main` está protegida. Cada unidad de trabajo debe usar un commit Conventional Commit con código, pruebas y documentación que expliquen el mismo cambio. No agregue atribución de IA ni líneas `Co-Authored-By`.

1. Cree una rama para el cambio y mantenga una unidad revisable por commit.
2. Ejecute las comprobaciones aplicables antes de solicitar revisión. Los gates de CI incluyen deriva de specs, lint/tipos, secretos, SAST, dependencias, pruebas, E2E, Trivy, DAST y SonarCloud; SonarCloud es requerido para los cambios que analiza.
3. Instale los hooks una vez con `pre-commit install`; un clon nuevo no los hereda. El hook puede modificar formato o rechazar secretos y cambios sin stage.
4. Abra un PR por corte de fase. La política registrada exige PR, checks requeridos y **merge commit**; squash, rebase, force push y bypass no están permitidos.
5. Antes de integrar, pase la revisión nativa configurada para el repositorio cuando aplique; su resultado no sustituye los checks obligatorios de CI.

Los commits de unidad de trabajo mantienen pruebas y documentación junto al comportamiento y permiten revisar o revertir un cambio sin arrastrar trabajo no relacionado.
