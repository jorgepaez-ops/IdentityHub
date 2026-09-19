# AGENTS.md — Identity Hub

Instrucciones para agentes de código (Codex). Este archivo y `odd/tasks/idp-semana-2.md` son
todo el contexto que necesitas: no asumas nada que no esté aquí o en `specs/`.
Los documentos del proyecto van en español; identificadores, código y mensajes de commit,
en inglés convencional.

## Qué es

Identity Hub es un proveedor de identidad (IdP) en Go + frontend React + PostgreSQL +
RabbitMQ, construido como vehículo de un pipeline DevSecOps de ciclo completo. El repo
contiene una **línea base deliberadamente vulnerable** (`specs/adr/0007`) y la fase actual
la remedia con evidencia antes/después. **`specs/` es la fuente de verdad**: el contrato es
`specs/03-api/openapi.yaml`; `backend/internal/api/gen.go` (cuando exista) es generado y
**no se edita a mano** nunca.

## Mapa del repo

```
specs/          requisitos (01), modelo (02), API (03), eventos (04), amenazas (05),
                aceptación .feature (06), trazabilidad generada (07), ADR
backend/        módulo Go `github.com/jorgepaez/identity-hub`: cmd/api, cmd/worker,
                internal/{api,config,events,observability,store}
frontend/       React + TS + Vite; Nginx en frontend/nginx/
db/migrations/  esquema versionado (golang-migrate: NNNNNN_nombre.up.sql / .down.sql)
deploy/         docker-compose.yml y observabilidad
security/       findings/ (fichas VULN-NNN), evidence/ (salidas de escáneres), .trivyignore
scripts/        traceability.py
odd/tasks/      lista de tareas de la feature en curso (idp-semana-2.md)
docs/           BITACORA.md, security-report.html, evidencia/ (ver más abajo)
e2e/tests/      vacío; Playwright llega en la semana 3
.github/workflows/  ci.yml, baseline-scan.yml, scheduled-scan.yml
```

## Comandos (verificados contra `Makefile` y los workflows)

| Qué | Comando |
|---|---|
| Ayuda | `make help` |
| Levantar / bajar stack | `make up` · `make down` · `make clean` (borra volúmenes) |
| Un servicio | `make restart S=api` · `make logs S=api` · `make ps` · `make build` |
| Tests Go (con `-race`) | `make test-go` (= `cd backend && go test -race -coverprofile=coverage.out -covermode=atomic ./...`) |
| Un paquete / una prueba | `cd backend && go test -race ./internal/config/...` · `go test -run TestRNF003 ./internal/config/` |
| Tests como en CI | `cd backend && go test -race -short ./...` |
| Tests frontend | `make test-front` (= `cd frontend && npm run test`) |
| Todo | `make test` |
| Lint + tipos | `make lint` (`go vet`, `golangci-lint` con gosec, `npm run lint`; si falta golangci-lint solo avisa, instálalo); tipos front: `cd frontend && npm run typecheck` |
| Formato Go | `make fmt` (gofmt) |
| Validar OpenAPI | `python3 -m openapi_spec_validator specs/03-api/openapi.yaml` (`pip install openapi-spec-validator==0.7.1`) |
| Trazabilidad | `python3 scripts/traceability.py` (escribe `specs/07-traceability.md`) · `--check` (falla si está desactualizada) |
| Gate de deriva local | `make gen` (hoy solo corre traceability.py; oapi-codegen/openapi-typescript los añade T2) |
| Tipos del cliente TS | `cd frontend && npm run gen:api` (script existente; requiere `npm install`) |
| Escaneos locales | `make scan` · `scan-secrets` · `scan-deps` · `scan-config` · `scan-image` (todos terminan con `\|\| true`: leer la salida, no el exit code) |
| BD | `make migrate` · `make psql` |
| Hooks locales | `pre-commit install` (gitleaks, gofmt, go build, traceability) |

Puertos del stack: web 8080, api 8081, Mailpit 8025, RabbitMQ 15672.
**No existen todavía** (los crean las tareas): sqlc, oapi-codegen, `make test-integration`,
pruebas Playwright, `docs/runbook.md`. `make e2e` es un placeholder.

## Cómo trabajas (acuerdo con Codex)

1. Una tarea de `odd/tasks/idp-semana-2.md` a la vez, en el orden del archivo. No empieces una
   tarea con dependencias sin marcar. Si una tarea está bloqueada, pasa a la siguiente cuya
   dependencia esté cumplida; si no hay ninguna, detente y avisa. Rama local: `feat/idp-semana-2`.
2. **TDD estricto**: escribe primero la prueba que falla y ejecútala (RED, pega la salida
   corta en el handoff); implementa lo mínimo (GREEN); refactoriza. No inventes evidencia.
3. Nombra las pruebas `TestRF012_Descripcion` / `TestRNF003_Descripcion` (ASCII, sin acentos):
   `scripts/traceability.py` las cuenta por ese patrón (`func Test(RF|RNF)NNN_...`).
4. Un Conventional Commit por tarea (`feat(auth): ...`, `fix(...)`, `test(...)`, `docs(...)`,
   `ci(...)`, `build(...)`, `chore(...)`), con pruebas y docs en el mismo commit.
   **Sin líneas `Co-Authored-By` ni atribución a IA.** Solo español/inglés técnico sobrio.
5. Marca la casilla de la tarea **solo tras ver pasar** sus comprobaciones. Un commit no puede
   contener su propio SHA, así que cada tarea cierra con **dos commits**: (a) el commit de la
   tarea (código, pruebas y docs); (b) un commit de registro, `docs(tasks): registra evidencia
   de T<id>`, que marca la casilla, anota el SHA de (a) en su línea `Commit:` y añade la nota de
   handoff. Nunca mezcles (b) con cambios de código, ni enmiendes (a) para meter su SHA.
   No marques casillas de `Usuario` ni de `Claude (revisión)`.
6. Nunca hagas `git push`, no abras PR, no uses `--force`, `reset --hard` ni reescribas
   historial, no crees ni muevas tags. Push, PR, tags y merge son del usuario.
7. Si un spec es ambiguo o contradictorio: **no adivines**. Anótalo en "Preguntas nuevas" (dentro de
   "Decisiones sobre las preguntas abiertas") del archivo de tareas y detén esa tarea. Si crees que hay
   que cambiar un spec, propónlo en "Cambios de spec propuestos"; no lo edites en silencio. Las únicas
   ediciones de `specs/` autorizadas hoy son C1 (`openapi.yaml`, solo en T1a) y C2 (ADR 0005, solo en T14a).
8. Tamaño orientativo: unas 400 líneas de cambio por tarea (heurística, no tope). Si lo correcto
   la supera, dilo en el handoff y sigue; no partas artificialmente.
9. No commitees `.atl/`, `.gga`, `docs/diagramas/` ni `.idea/` salvo que la tarea lo pida.

## Límites duros

- **Línea base vulnerable.** No borres ni reescribas el tag `v0.0.0-vuln-baseline` ni estos
  archivos, sembrados a propósito y necesarios como evidencia "antes", hasta que lo haga la
  tarea designada (entre paréntesis):
  `backend/internal/api/legacy_auth.go` y su ruta `/auth/legacy-login` en `server.go`
  (VULN-001, 002, 004, 005, 006, 007; T23) · `backend/go.mod` con chi 5.0.11, jwt v4, pgx 5.5.1,
  x/text y la directiva `go 1.22` (VULN-020 chi T6, VULN-021 T23, VULN-022 y x/text T24) ·
  `backend/Dockerfile` (VULN-008 a 012; T26, T27) · `frontend/Dockerfile` (VULN-016 a 018; T28) ·
  `frontend/nginx/default.conf` (VULN-013 a 015; T29) · `frontend/package.json` con axios y
  lodash antiguos (T25) · `deploy/docker-compose.yml` (VULN-003, 019 y 024; T26, T30). Su comentario de endurecimiento dice hoy
  `VULN-020`; no lo renumeres antes de T30 (el id VULN-020 es el hallazgo de chi).
  Añadir código nuevo junto a ellos es válido; "arreglarlos de paso" destruye la evidencia.
  Solo tareas marcadas `Remedia:` los tocan, y solo tras `T0.1` a `T0.3` marcadas.
- Nunca commitees secretos reales, claves privadas ni `.env`. Las claves de desarrollo se
  generan localmente y viven en `.env` (ignorado por git). No pegues URLs con credenciales
  ni asignaciones `CLAVE=valor` en docs: Gitleaks las marcaría.
- Nunca debilites un gate para ponerlo en verde: nada de `continue-on-error`, bajar umbrales,
  ampliar `.gitleaks.toml`/`.trivyignore`, añadir `//nolint` o `#nosec` sin justificación y
  aprobación del usuario. Endurecer un gate sí es válido.
- Los contratos de `specs/` (OpenAPI, AsyncAPI, requisitos, ADR, features) no se modifican
  sin pasar por "Cambios de spec propuestos". `specs/07-traceability.md` es generado.
- El repo aún no tiene remoto (lo crea el usuario en `T0.1`; será público); no intentes configurarlo.

## Seguridad del núcleo IdP (referencias, no repetir aquí)

- Contraseñas: Argon2id con los parámetros de `specs/adr/0004`; el `CHECK` de `users`
  (`password_hash LIKE '$argon2id$%'`) se mantiene; hash señuelo si el usuario no existe.
- JWT: Ed25519, solo `EdDSA` aceptado, `kid` publicado en `GET /.well-known/jwks.json` (RF-004,
  AM-003). Clave privada por entorno; su ausencia impide arrancar (RNF-003).
- Refresh: opaco, 32 bytes de `crypto/rand`, guardado como SHA-256, rotación con `family_id`
  y revocación de familia ante reuso (`specs/adr/0005`, RF-005, RF-006).
- SQL siempre parametrizado vía sqlc; jamás concatenación (AM-006). Aleatoriedad solo con
  `crypto/rand`. Nada de contraseñas, tokens ni secretos en logs (RNF-012); tokens truncados
  a 8 caracteres en logs.
- Audit log append-only (RF-011, AM-010): trigger existente + `REVOKE UPDATE, DELETE` al rol
  `identity_app` (T9). IP de cliente solo de una fuente de confianza (VULN-020 chi, T6).
- Roles: se verifican en servidor, leyendo la BD en operaciones sensibles (AM-007, AM-021).
- Respuestas idénticas para correo existente e inexistente (AM-004). Errores en RFC 7807.

## Definición de terminado (por tarea)

- [ ] Prueba escrita primero; RED y GREEN observados y resumidos en el handoff.
- [ ] `make test-go` y `make lint` sin fallos nuevos (los hallazgos sembrados preexistentes no
      cuentan; cualquier hallazgo nuevo sí). Integración/frontend/Docker según la tarea.
- [ ] `python3 scripts/traceability.py` ejecutado y su salida commiteada si cambió.
- [ ] Código generado regenerado (sin diff) cuando la tarea toca specs, sqlc u OpenAPI.
- [ ] Docs/ficha/ADR afectados actualizados en el mismo commit.
- [ ] Commit de la tarea convencional y sin atribución, más el commit `docs(tasks)` de registro
      (casilla marcada, SHA del primero anotado, handoff escrito).

## Protocolo de traspaso

Al cerrar cada tarea añade en "Notas de handoff Codex" (archivo de tareas) 3-5 líneas:
qué cambió, comandos ejecutados con su resultado observado (incluye el RED), y dudas abiertas.
Claude revisa cada commit contra ese texto; que sea suficiente para revisar sin reejecutar todo.
El espejo en memoria del archivo de tareas lo reconcilia Claude; tú solo editas el archivo.

## Evidencia de vulnerabilidades (antes / después)

Convención completa en `odd/tasks/idp-semana-2.md`, sección "Protocolo de evidencia".
Resumen: por hallazgo, `docs/evidencia/VULN-XXX/` con `before.png`, `after.png` y
`evidencia.json` (los toma el **usuario**; tú no haces capturas). Tu parte: el commit de
remediación, dejar la ficha `security/findings/VULN-XXX-*.md` con "Commit de remediación" y
estado actualizado, y no ejecutar ningún commit de remediación antes de que exista el "antes".

## Reglas de revisión de Go (`.gga` usa este archivo como `RULES_FILE`)

- SQL solo por `sqlc` o consultas parametrizadas; nunca `fmt.Sprintf`/concatenación en SQL (VULN-005, VULN-022).
- Aleatoriedad de seguridad con `crypto/rand`, nunca `math/rand`; comparaciones de secretos y MAC con `subtle.ConstantTimeCompare`.
- Contraseñas solo con Argon2id (ADR 0004); nunca MD5/SHA-1/SHA-256 directo para contraseñas.
- Ni contraseñas, tokens, refresh tokens ni cuerpos de mensajes en logs ni en errores devueltos al cliente.
- Errores envueltos con contexto (`%w`), `context.Context` propagado y sin `panic` en rutas de petición.
- La IP de cliente sale solo de la lógica de proxies confiables (T6); nunca de `X-Forwarded-For` sin validar (VULN-020).
- Código generado (`gen.go`, `*.sql.go`) no se revisa ni se edita a mano.
- Cada cambio de comportamiento trae su prueba (RED antes de GREEN) en el mismo commit.

## Reglas de revisión de frontend (`.gga` usa este archivo como `RULES_FILE`)

- Sin `dangerouslySetInnerHTML` ni `eval` (AM-015, RNF-009); tokens nunca en `localStorage`.
- Tipos de la API desde `frontend/src/api/schema.d.ts` (generado), no a mano.
- Cambios de frontend pasan `npm run lint`, `npm run typecheck` y `npm run test`.
