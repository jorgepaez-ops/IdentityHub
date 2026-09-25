# Guía de desarrollo local

Complementa el README (arquitectura y `make up`). Esto es lo operativo: cómo
correr el backend fuera de Docker, qué hace `make gen` por dentro, cómo correr
las pruebas de Go a mano y cómo configurar GoLand.

## 1. Dos formas de correr el proyecto

**Todo en Docker (`make up`)** — la forma "de verdad", igual que en CI. Levanta
`db`, `broker`, `mailpit`, `migrate`, `api`, `worker` y `web`. Útil para probar
el sistema completo, no para iterar rápido en Go: cada cambio exige
`make restart S=api`, que reconstruye la imagen.

**Solo Postgres en Docker, Go en el host** — la forma rápida para trabajar en
`backend/`, y la que usa Claude para verificar tareas contra una base real:

```bash
cd deploy && docker compose up -d db
# espera a que quede "healthy":
docker inspect --format='{{.State.Health.Status}}' identity-hub-db-1

# aplica las migraciones (usa el servicio `migrate` del compose, no un binario local):
docker compose run --rm migrate

cd ../backend && go build ./...
```

Con la base así levantada, `go test`, `go run` y el depurador de GoLand
hablan directo con PostgreSQL en `localhost:5432` sin pasar por Nginx ni por
el contenedor de la API.

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
en la raíz del repo quedó desactualizado desde la semana 1 y T21 lo pone al
día cuando cablee `main.go`). Obligatorias = el proceso no arranca sin ellas.

| Variable | Obligatoria | Default | Qué es |
|---|---|---|---|
| `DATABASE_URL` | sí | — | cadena de conexión a PostgreSQL |
| `RABBITMQ_URL` | sí | — | cadena de conexión al broker |
| `JWT_SIGNING_KEY` | sí | — | semilla Ed25519 de 32 bytes, en base64 (`openssl rand -base64 32`) |
| `API_PORT` | no | `8081` | puerto HTTP de la API |
| `LOG_LEVEL` | no | `info` | |
| `JWT_ISSUER` | no | `http://localhost:8080` | claim `iss` del token |
| `JWT_AUDIENCE` | no | `identity-hub` | claim `aud` |
| `JWT_ACCESS_TTL` | no | `15m` | vigencia del access token |
| `JWT_REFRESH_TTL` | no | `720h` | vigencia del refresh token (cookie) |
| `LOGIN_ACCOUNT_MAX_FAILURES` | no | `5` | fallos antes de bloquear la cuenta (RF-017) |
| `LOGIN_IP_MAX_FAILURES` | no | `20` | fallos por IP antes de bloquear (RF-017 / VULN-020) |
| `LOGIN_FAILURE_WINDOW` | no | `15m` | ventana deslizante de los dos contadores |
| `LOGIN_LOCKOUT_DURATION` | no | `15m` | duración del bloqueo de cuenta |
| `TRUSTED_PROXIES` | no | vacío | CIDR separados por coma; sin esto, `X-Forwarded-For` se ignora siempre (T6) |
| `ARGON2_MEMORY_KIB` / `_ITERATIONS` / `_PARALLELISM` / `_CONCURRENCY` | no | `65536` / `3` / `2` / `4` | parámetros de Argon2id |
| `SMTP_HOST` / `_PORT` / `_FROM` | no | `mailpit` / `1025` / `no-reply@identity.local` | |
| `PUBLIC_BASE_URL` | no | `http://localhost:8080` | usado para armar enlaces en los correos |

`Load()` acumula **todos** los errores de configuración antes de fallar
(ver el comentario en `config.go`): un solo arranque fallido lista todo lo
que falta, en vez de una variable a la vez.
