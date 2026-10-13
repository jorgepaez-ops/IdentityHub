# seguimiento-pre-v1 — Cerrar el seguimiento técnico antes del tag `v1.0.0`

## Objetivo

No dejar pendientes técnicos abiertos en la entrega, salvo F5 (integración de terceros y outbox,
trabajo futuro decidido el 2026-10-12). Decisión del usuario (2026-10-13): «incluye todo, también los
code smells de SonarCloud».

## Problema y por qué

Las revisiones nativas del 2026-10-13, la sección «Limitaciones» del informe técnico y la «Deuda menor»
de `odd/tasks/cierre-entrega.md` dejaron trabajo anotado como seguimiento. El usuario no quiere entregar
con esos temas abiertos.

## Alcance autorizado

- S1-S7 de abajo, con pruebas y documentación junto al comportamiento.
- Fuera de alcance: F5; ampliar alcances de claves de servicio (función nueva, cercana a F5); aplicar
  Terraform y correr Falco en CI (límites de diseño documentados).

## Restricciones

- Rama `feat/seguimiento-pre-v1`, apilada sobre `docs/notas-2026-10-13` (documentación de cierre aún sin PR).
- Cada tarea cierra con un commit de unidad de trabajo (Conventional Commits, sin atribución de IA).
- Política de pruebas primero cuando exista una prueba ejecutable determinista (RED → GREEN).
- Delegación: Sonnet escribe; Opus revisa antes de cada commit (CLAUDE.md, reparto de modelos).
- Entrega: `ask-on-risk`; el pronóstico supera ~400 líneas, se pregunta la estrategia de cadena.

## Tareas

- [x] **S1 — Verificación automática del release.** Paso en `release.yml` (job `publish`, después de
  firmar y atestar y antes de mover `latest`) que ejecute `cosign verify` y
  `cosign verify-attestation --type cyclonedx` con la identidad del workflow y el emisor OIDC de Actions.
  Prueba real: el tag `v1.0.0`. Documentar en informe, ADR 0011, manual de despliegue y modelo de amenazas
  (AM-008).
  Evidencia: sin RED local posible (el paso solo corre en el release; no hay prueba ejecutable determinista).
  Comprobado: `actionlint` limpio; `zizmor` sin hallazgos, igual que HEAD (0 y 2 suprimidos); orden del
  paso: después de firmar/atestar y antes de mover `latest`; cosign-installer v4.1.2 (Cosign v3).
- [x] **S2 — Clasificador del job 0 en `scripts/` con prueba.** `scripts/ci_changes.py` (o `.sh`) con
  pruebas sobre: solo documentación, mixto, `.md` bajo `specs/`/`.github/`/`scripts/`, renombre de código a
  `docs/`, lista vacía, error de git. `ci.yml` lo invoca. Sustituir la guarda repetida en cada paso del job
  `image` por una forma que no dependa de cada paso nuevo (por ejemplo, un paso que termine el job temprano
  no existe en Actions: evaluar acción compuesta o matriz condicional; documentar la elección).
  Evidencia: RED = `python3 scripts/ci_changes_test.py` con 12 fallos (el script no existía; las 2 pruebas del
  lint de guardas pasaron desde el inicio porque las guardas ya existían). GREEN = 12 pruebas OK;
  `actionlint` limpio; `zizmor` 21 hallazgos antes y después. Elección del job `image`: se mantienen las
  guardas por paso (una acción compuesta sacaría las acciones fijadas por SHA del alcance de Dependabot,
  `directory: "/"` no recorre `.github/actions/*`, y el checkout debe quedar fuera) y la prueba falla si un
  paso de `image` no tiene la guarda; documentado en el encabezado de `ci.yml`.
- [x] **S3 — Prueba de los puertos de observabilidad en loopback.** Prueba (en CI, job barato) que lea
  `docker compose config` y falle si Prometheus, Loki o Grafana publican sin `host_ip: 127.0.0.1`.
  Aclarar en la guía el caso de `localhost` resuelto a `::1`.
  Evidencia: RED = `python3 scripts/compose_ports_test.py` con 4 errores (`unsafe_ports` aún sin implementar,
  `NotImplementedError`); GREEN = 4 pruebas OK contra `docker compose config` real. Mutación comprobada: quitar
  `127.0.0.1:` del puerto 9090 en una copia del compose hace fallar la prueba (`host_ip=None`). Corre en el job 1
  (`spec-drift`, siempre activo) y en `make spec-drift`.
- [x] **S4 — Limpieza menor.** Bloque de URLs repetido entre `up` y `up-app` del Makefile; sección
  «Levantar el stack» duplicada entre `README.md` y `docs/guia-desarrollo.md` (una fuente y un enlace).
  Evidencia: no hay prueba ejecutable determinista para el Makefile; comprobado con `make -n up`, `make -n up-app`
  y `make help` contra `HEAD` (mismas líneas de comando; el bloque de URLs ahora es la variable `PRINT_APP_URLS`
  y su salida real es idéntica; `help` igual salvo el nombre del archivo). README: «Levantar el stack» queda en un
  párrafo con enlace a `docs/guia-desarrollo.md#levantar-el-stack`.
- [x] **S5 — RNF-010 (latencia) con prueba de carga.** Prueba de carga mínima y reproducible contra el
  stack de CI con un umbral alineado con `specs/01-requirements.md` (RNF-010) y su gate; la matriz debe
  pasar RNF-010 de «sin cubrir» a cubierto.
  Evidencia: `make load-test` (`scripts/load-test.sh` + `tests/load/auth-latency.js`, k6 en
  `grafana/k6@sha256:e66db15b…`, digest leído con `docker pull` + `docker inspect`) mide refresh, `GET /me`
  y JWKS con umbral `p(95)<300` (RNF-010: p95 de los endpoints de autenticación, sin Argon2id; el login
  se hace una vez en `setup()`). RED: con `P95_MS=1` termina con código 99 y marca los tres umbrales
  `✗`. GREEN contra `make up-app` (20 s por escenario, 2 074 peticiones, 0 % de errores): corrida 1
  refresh p95 4,79 ms / p99 6,88 ms, `/me` 3,15 / 4,15 ms, JWKS 2,14 / 2,54 ms; corrida 2 (vía `make
  load-test`) p95 5,46 / 3,25 / 2,25 ms. Paso nuevo en el job 7 de CI tras `make e2e`; sin
  `with-raised-login-limit.sh` a propósito (ningún login falla y refresh va a ~3 r/s < 5 r/s de Nginx).
  Matriz: RNF-010 pasa de «sin cubrir» a completo (Escenarios 1 + Controles CI); 26 completos, 7 parciales,
  0 sin cubrir. `actionlint` limpio; `zizmor` 21 hallazgos antes y después (sin nuevos); `make informe` ok
  (rangos `{{code:...ci.yml}}` del informe desplazados +6). Rutas de CI no ejecutadas en un runner real.
- [x] **S6 — Segunda evidencia de los 7 requisitos parciales.** RF-008, RNF-001, RNF-002, RNF-006,
  RNF-007, RNF-008, RNF-012 tienen una sola columna de evidencia en `specs/07-traceability.md`
  (completo = 2 de 4: Escenarios, Pruebas Go, E2E, Controles CI). Añadir la evidencia que falte con
  pruebas reales, sin inflar la matriz.
  Evidencia (cada una es una verificación real, con negativo): RF-008, `perfil-propio.feature` (2 escenarios)
  + 2 E2E `RF-008 ...` (GET /me con y sin token, PATCH del nombre y 400 con nombre vacío). RNF-007, E2E
  `RNF-007 ...`: `/metrics` de API y worker en formato Prometheus, un login fallido real sube
  `identity_login_attempts_total{result="failed"}` en 1, todas las líneas de log son JSON y el tablero consulta las
  cuatro señales. RNF-012, E2E `RNF-012 ...`: login fallido + completo + refresh y ningún secreto (contraseñas, mfaToken,
  access/refresh token, código) aparece en los logs de api ni worker; RED: añadir `escuchando` a la lista de
  secretos hace fallar la prueba. RNF-001, RNF-002, RNF-006, RNF-008: `scripts/pipeline_controls_test.py` (compose
  renderizado, Dockerfile y workflows; 12 pruebas, en `make spec-drift` y en el job 1) + 4 escenarios en
  `requisitos-no-funcionales.feature`; RED por mutación (worker `read_only: false`, `cosign verify` quitado de
  `release.yml`, `continue-on-error` en `sast`): fallan 3 pruebas; restaurado, 12 OK. El generador no se tocó: los
  Python no cuentan por sí solos en la matriz, por eso van respaldados por escenarios `@RNF-NNN` que nombran su prueba.
  Resultado: `make e2e` 63 pruebas OK (eran 59); matriz 33 completos / 0 parciales / 0 sin cubrir. Alcances
  honestos en el informe: RNF-006 verifica al publicar, no al desplegar; RNF-008 solo api/worker/web; RNF-002 sin
  despliegue automatizado. Sin Go tocado (golangci-lint no aplica). `actionlint` limpio, `zizmor` 21 hallazgos (igual
  que HEAD).
  Correcciones de la revisión nativa de S5/S6 (91404ff, dea7d27, 2840afe, cdafb16, 43556d2):
  - `e2e/support/logs.ts`: `maxBuffer` de 256 MiB en `docker logs` (91404ff). Se probó acotar con `--since` y se descartó: la
    API no registra nada por petición y la ventana quedó vacía («api no emitió logs», 2 pruebas rojas); además el chequeo
    de RNF-012 debe cubrir también la aceptación de la invitación.
  - RNF-007: playwright ya es `workers: 1`, pero el contador es global al proceso; ahora `>= antes + 1`. Variable `titles` renombrada a `panels` (91404ff).
  - Job 7: `make load-test` ahora corre bajo `scripts/with-raised-login-limit.sh` como `make e2e` (dea7d27). Comprobado: justo
    después de `make e2e` (IP con 36 fallos > límite 20) pasó el load test. Actualizados comentario de ci.yml (mismo número de líneas) y guía.
  Segunda ronda de la revisión nativa (S5/S6):
  - `traceability.py`: gramática explícita de `# Verificación:` (`(Nombre)`, `job <id>`, `paso "<nombre>"`, `make <objetivo>` con o sin comillas); el id de job ya no arrastra puntuación, el nombre de clase/función exige palabra completa o comilla de cierre y las comillas que no son `paso "..."` se ignoran; 5 pruebas negativas nuevas (RED 4 fallos, luego 21 OK); docstring y README de specs/06-acceptance alineados (da7afbf).
  - `load-test.sh`: la guarda de producción mira también `COMPOSE_FILE`; límite documentado (prod comparte proyecto `identity-hub` y servicio `db`, no se puede distinguir el contenedor); trap EXIT separado de INT/TERM (130/143, limpieza conserva el código); nueva `scripts/load_test_script_test.py` con docker simulado en PATH, enganchada a `make spec-drift` y al job spec-drift (RED 3 fallos, luego 6 OK) (1d4b874).
  - `Makefile`: `load-test` usa `WITH_RAISED_LIMIT` mediante el objetivo privado `_load-test-run`; `with-raised-login-limit.sh` ya restaura con trap EXIT y conserva el código (comprobado con un compose simulado y un comando que sale con 9: restauró y devolvió 9) (1521894).
  - `scripts/README.md`: catálogo de scripts y pruebas, enlazado desde la guía y el README.
  - `scripts/load-test.sh`: secretos por nombre (`-e LOAD_PW`), cuenta `load-*` deshabilitada al salir (no se borra: la FK
    `ON DELETE SET NULL` de `audit_log` es un UPDATE que el disparador append-only rechaza) y rechazo de `docker-compose.prod.yml` (dea7d27).
  - `auth-latency.js`: constantes con comentario para los reintentos del código MFA y los 2 s de desfase de relojes (dea7d27).
  - `pipeline_controls_test.py`: `latest` se publica una vez y tras el último sign/attest/verify; gate de release con `exit-code: "1"`;
    prueba de Dockerfile renombrada a lo que demuestra (2840afe).
  - `traceability.py`: `verification_errors` comprueba por máquina el `# Verificación:` de cada escenario solo-`@RNF`; RED en copia temporal (cdafb16); README de 06-acceptance actualizado.
  - Docs: tabla de evidencia de los 8 requisitos en el informe (Fase 4), subsección en la guía (43556d2).
  Tercera ronda de la revisión nativa (S5/S6):
  - `load_test_script_test.py`: el stub «slow» de k6 espera (sondeo de 50 ms, tope de 30 s) a que la prueba cree `STUB_RELEASE`, que se crea solo después de enviar la señal, así INT/TERM siempre llegan con k6 corriendo; docstring corregido; 10 corridas seguidas OK.
  - `traceability.py`: una cadena entre comillas dobles sin prefijo en `# Verificación:` es error; nuevo `e2e "<título>"` comprobado contra `e2e/tests/*.spec.ts`; `make <objetivo>` solo cuenta entre comillas o comillas invertidas (prosa como «make sure» ya no se lee como objetivo); 3 pruebas nuevas/ajustadas (RED 3 fallos, luego 23 OK); README de specs/06-acceptance y scripts/README alineados; matriz sin cambios.
  - `Makefile`: `_load-test-run` pasa `COMPOSE='$(COMPOSE)'` explícito a `scripts/load-test.sh`; comprobado con `make -n` y `make up-app && make load-test` real (exit 0, p95 3.82 ms), luego `make down`.
  Cuarta ronda de la revisión nativa (S5/S6):
  - `traceability.py`: `verification_errors` usa un único tokenizador (`VERIFICATION_TOKEN`, una pasada) con la gramática documentada en un solo texto (docstring, `specs/06-acceptance/README.md`, `scripts/README.md`); `make <objetivo>` suelto, entre comillas invertidas o dobles iguales (delimitadores distintos son error); `e2e` acepta `test(`/`test.only(`/`test.skip(` con comillas simples, dobles o invertidas; ci.yml, Makefile y specs E2E se leen una vez; prueba nueva de que los `.feature` no usan «make» como prosa. `load_test_script_test.py`: constantes con nombre y el exit 99 documentado (STUB_RELEASE nunca apareció). RED 6 fallos, luego 29 OK; matriz sin cambios.
- [ ] **S7 — Code smells de SonarCloud.** 123 abiertos al 2026-10-13 (CRITICAL 32, MAJOR 55, MINOR 34,
  INFO 2); reglas principales: go:S3776 (16), typescript:S6819 (16), css:S4666 (11), godre:S8196 (10),
  plsql:S1192 (6), python:S3776 (6). Corregir sin cambiar comportamiento, con las pruebas como red; los
  falsos positivos se justifican en Sonar, no se silencian en bloque.
  - Quinta ronda (inline, Opus): las comillas invertidas fuera de una referencia son formato si están cerradas; el mensaje de comilla sin pareja ya no culpa a `make`; prueba fuerte del mensaje; la prueba de los `.feature` reales tokeniza sus líneas `# Verificación:`; `test_the_grammar_copies_match_the_docstring` mantiene sincronizadas las tres copias de la gramática (mutación en `scripts/README.md`: falla). Revisión de la quinta ronda: aprobada; la advertencia R3-001 (restaurar la prueba de «make» en prosa fuera de `# Verificación:`) se descarta porque el analizador solo lee esas líneas y la ronda anterior marcó esa prueba como incorrecta; se aplicaron las dos sugerencias (prueba con ruta real entre comillas invertidas y `skipTest` con `python -OO`).

Lote 1/3 (Go), 2026-10-13: ver apéndice «S7 — inventario Sonar». Corregido: godre:S8242 (`internal/testdb/testdb.go:73`, `migrationLock` ya no guarda `context.Context`; `release`/`releaseIfHeld` lo reciben por parámetro) y de paso el `errcheck` que golangci-lint con `-tags=integration` marcaba en la misma función (`_ = dropDatabase(...)`). Sin cambio de comportamiento. No aplicado: go:S3776 x16 (todos en archivos `_test.go`, fuera de alcance por instrucción), godre:S8196 x10 (la descripción de la regla no está en la API; el nombre «método+er» daría `WithinLoginTransactioner`, peor que `Repository`; se propone marcar falso positivo en Sonar, decisión del usuario), go:S1135 (falso positivo: «todo» en español). Sin código generado marcado; `.sonarcloud.properties` ya excluye duplicados de `gen.go`/`sqlc`. Observación fuera de alcance: `gocognit` da 25 a `(*smtpMailer).Send` (`backend/cmd/worker/smtp.go:80`) y Sonar no lo marca (cuenta distinto). Evidencia: golangci-lint v2.13.2 0 issues (con y sin `-tags=integration`), `go vet`, `gofmt`, `make test-go` OK (59.9 %), `make test-integration` con Postgres 16 desechable OK (81.1 % antes y después; funciones de `testdb` sin pérdida de cobertura), `gocognit -over 15` sin hallazgos en `testdb.go`.

## Criterios de aceptación

1. Las pruebas de cada tarea pasan (RED observado antes de GREEN cuando aplica) y CI queda en verde.
2. `python3 scripts/traceability.py --check` al día; 0 requisitos sin cubrir y 0 parciales, salvo los que
   se justifiquen por escrito.
3. SonarCloud con 0 code smells abiertos o cada restante justificado como falso positivo en Sonar.
4. Informe técnico, bitácora y manuales sin «Limitaciones» o «Trabajo futuro» que ya estén resueltos.

## Comprobaciones aplicables

`make test-go`, `make test-integration` (con `TEST_DATABASE_URL`), `make test-front`, `make lint` con
golangci-lint real, `npm run typecheck`, `python3 scripts/traceability.py --check`, `actionlint`, `zizmor`
sin hallazgos nuevos, CI completo en el PR.

## Pronóstico de entrega

S1 ~40 · S2 ~200 · S3 ~80 · S4 ~60 · S5 ~200 · S6 ~350 · S7 ~900 líneas → ~1 800, supera ~400.
Estrategia: `ask-on-risk` → `stacked-to-main` (usuario, 2026-10-13). Cortes: PR-A = S1-S4; PR-B = S5-S6; PR-C… = S7 por lotes de ~400 líneas. El PR de documentación (`docs/notas-2026-10-13`) va antes.

## Progreso

| Tarea | Ruta | Estado | Commit | Revisión |
|---|---|---|---|---|
| S1 | delegado (Sonnet) | hecho | 118192d | — |
| S2 | delegado (Sonnet) | hecho | e2701fd | — |
| S3 | delegado (Sonnet) | hecho | fa6b8fe | — |
| S4 | delegado (Sonnet) | hecho | aeeb74e | — |
| S5 | delegado (Sonnet) | hecho | d64043a | — |
| S6 | delegado (Sonnet) | hecho | df56da3 | — |
| S7 | delegado (Sonnet) | en curso: lote 1/3 Go hecho (godre:S8242); lotes 2/3 y 3/3 pendientes | `e8b3dd0` (S8242 + errcheck), `6a41c55` (inventario) | — |

Revisión nativa de S1-S4 (2026-10-13): aprobada (4 lentes, 15 archivos). Correcciones posteriores, también
aprobadas: identidad de `cosign verify` restringida a `release.yml` en un tag (probada contra `v0.9.1`, y
rechaza la identidad de `ci.yml`) (`a15ee05`); informe con el clasificador probado y extracto de `release.yml`
corregido (`f1b60ca`). Advertencia descartada con evidencia: la prueba de puertos sin `.env` pasa en una copia
limpia con `CI=true` (el compose no usa `env_file` ni interpolación sin `:?`). El rango 161-196 del extracto
incluye a propósito el comentario de la línea 161 y termina en el último renglón del paso de `latest`.

## Siguiente paso

PR de documentación (tras la revisión del PDF por el usuario), luego PR-A (S1-S4) y S5-S6.

## Apéndice S7 — inventario Sonar (2026-10-13)

Fuente: `api/issues/search` (CODE_SMELL, sin resolver), 123 al momento de la consulta. Agrupado por lenguaje y regla; las columnas «Estado» de Go las llena el lote 1/3, el resto queda «pendiente» para los lotes 2/3 (frontend/contabilidad/e2e/CSS/docs JS) y 3/3 (Python, shell, SQL, workflows).

| Lenguaje | Regla | Severidad | Archivo:línea | Mensaje | Estado |
|---|---|---|---|---|---|
| css | css:S4666 | MAJOR | `frontend/src/styles.css:106` | Duplicate selector ".stat-tile", first used at line 103 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:177` | Duplicate selector ".link-button", first used at line 146 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:201` | Duplicate selector ":root", first used at line 1 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:203` | Duplicate selector ":root[data-theme="dark"]", first used at line 31 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:204` | Duplicate selector "body", first used at line 33 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:214` | Duplicate selector ".left-rail", first used at line 55 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:215` | Duplicate selector "nav a", first used at line 59 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:217` | Duplicate selector ".logout-button", first used at line 64 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:218` | Duplicate selector ".icon-button", first used at line 154 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:222` | Duplicate selector ".table-scroll", first used at line 134 | pendiente |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:225` | Duplicate selector ".toast", first used at line 168 | pendiente |
| githubactions | githubactions:S1135 | INFO | `.github/workflows/ci.yml:14` | Complete the task associated to this "TODO" comment. | pendiente |
| go | go:S1135 | INFO | `backend/internal/events/events_test.go:85` | Complete the task associated to this TODO comment. | falso positivo: «todo» es español («todo tipo de evento»), no un marcador TODO |
| go | go:S3776 | CRITICAL | `backend/cmd/worker/smtp_test.go:136` | Refactor this method to reduce its Cognitive Complexity from 26 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/api/login_test.go:308` | Refactor this method to reduce its Cognitive Complexity from 17 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/api/login_test.go:429` | Refactor this method to reduce its Cognitive Complexity from 19 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/api/oauth_test.go:133` | Refactor this method to reduce its Cognitive Complexity from 23 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/api/server_generated_test.go:15` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/audit/audit_integration_test.go:13` | Refactor this method to reduce its Cognitive Complexity from 19 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/auth/bootstrap/bootstrap_integration_test.go:169` | Refactor this method to reduce its Cognitive Complexity from 16 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/auth/login/login_test.go:168` | Refactor this method to reduce its Cognitive Complexity from 17 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/auth/mfa/mfa_integration_test.go:103` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/auth/refresh/refresh_integration_test.go:23` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/auth/servicekey/servicekey_integration_test.go:186` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/auth/servicekey/servicekey_test.go:282` | Refactor this method to reduce its Cognitive Complexity from 22 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/store/queries_integration_test.go:18` | Refactor this method to reduce its Cognitive Complexity from 16 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/store/rolegrid_integration_test.go:20` | Refactor this method to reduce its Cognitive Complexity from 25 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/store/rolegrid_integration_test.go:75` | Refactor this method to reduce its Cognitive Complexity from 25 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | go:S3776 | CRITICAL | `backend/internal/store/sessions_integration_test.go:17` | Refactor this method to reduce its Cognitive Complexity from 22 to the 15 allowed. | fuera de alcance (archivo de pruebas); propuesta: marcar «won't fix» en Sonar o refactor aparte |
| go | godre:S8196 | MINOR | `backend/cmd/api/main.go:67` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/cmd/api/main.go:243` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/auth/bootstrap/bootstrap.go:55` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/auth/employee/employee.go:46` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/auth/invitationresend/resend.go:32` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/auth/login/login.go:86` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/auth/logout/logout.go:40` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/auth/mfa/mfa.go:160` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/auth/refresh/refresh.go:77` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8196 | MINOR | `backend/internal/events/broker.go:13` | Rename this interface to follow Go naming conventions for single-method interfaces. | pendiente de decisión: nombre método+«er» (p. ej. WithinLoginTransactioner) empeora la legibilidad; descripción de la regla no accesible por API. Propuesta: marcar falso positivo en Sonar |
| go | godre:S8242 | MAJOR | `backend/internal/testdb/testdb.go:73` | Remove this 'context.Context' field and pass context as a parameter to methods that need it. | corregido (lote 1/3) |
| javascript | javascript:S4624 | MAJOR | `docs/informe/build.mjs:138` | Refactor this code to not use nested template literals. | pendiente |
| javascript | javascript:S4624 | MAJOR | `docs/informe/build.mjs:157` | Refactor this code to not use nested template literals. | pendiente |
| javascript | javascript:S6582 | MINOR | `docs/informe/build.mjs:81` | Prefer using an optional chain expression instead, as it's more concise and easier to read. | pendiente |
| javascript | javascript:S7758 | MINOR | `docs/informe/build.mjs:122` | Prefer `String.fromCodePoint()` over `String.fromCharCode()`. | pendiente |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | pendiente |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | pendiente |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | pendiente |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | pendiente |
| javascript | javascript:S878 | MAJOR | `docs/informe/build.mjs:136` | Unexpected use of comma operator. | pendiente |
| javascript | javascript:S878 | MAJOR | `docs/informe/build.mjs:137` | Unexpected use of comma operator. | pendiente |
| javascript | javascript:S8786 | MAJOR | `docs/informe/build.mjs:88` | Simplify this regular expression to reduce its runtime, as it has super-linear performance due to backtracking. | pendiente |
| javascript | javascript:S9382 | MINOR | `docs/informe/build.mjs:301` | Unexpected `await` inside a loop. | pendiente |
| javascript | javascript:S9382 | MINOR | `docs/informe/build.mjs:322` | Unexpected `await` inside a loop. | pendiente |
| javascript | javascript:S9382 | MINOR | `docs/informe/build.mjs:322` | Unexpected `await` inside a loop. | pendiente |
| plsql | plsql:OrderByExplicitAscCheck | MINOR | `db/queries/roles.sql:6` | Add ASC in order to make the order explicit. | pendiente |
| plsql | plsql:OrderByExplicitAscCheck | MINOR | `db/queries/roles.sql:26` | Add ASC in order to make the order explicit. | pendiente |
| plsql | plsql:OrderByExplicitAscCheck | MINOR | `db/queries/users.sql:39` | Add ASC in order to make the order explicit. | pendiente |
| plsql | plsql:S1192 | CRITICAL | `db/migrations/000001_initial_schema.up.sql:17` | Define a constant instead of duplicating this literal 5 times. | pendiente |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:38` | Define a constant instead of duplicating this literal 11 times. | pendiente |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:62` | Define a constant instead of duplicating this literal 5 times. | pendiente |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:104` | Define a constant instead of duplicating this literal 5 times. | pendiente |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:287` | Define a constant instead of duplicating this literal 6 times. | pendiente |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:338` | Define a constant instead of duplicating this literal 4 times. | pendiente |
| python | python:S1481 | MINOR | `scripts/zap-gate.py:109` | Replace the unused local variable "comment" with "_". | pendiente |
| python | python:S3358 | MAJOR | `scripts/traceability.py:337` | Extract this nested conditional expression into an independent statement. | pendiente |
| python | python:S3358 | MAJOR | `scripts/traceability.py:338` | Extract this nested conditional expression into an independent statement. | pendiente |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:69` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. | pendiente |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:114` | Refactor this function to reduce its Cognitive Complexity from 22 to the 15 allowed. | pendiente |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:167` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. | pendiente |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:240` | Refactor this function to reduce its Cognitive Complexity from 25 to the 15 allowed. | pendiente |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:288` | Refactor this function to reduce its Cognitive Complexity from 40 to the 15 allowed. | pendiente |
| python | python:S3776 | CRITICAL | `scripts/zap-gate.py:80` | Refactor this function to reduce its Cognitive Complexity from 22 to the 15 allowed. | pendiente |
| shell | shelldre:S131 | CRITICAL | `scripts/ip-lockout-warning.sh:26` | Add a default case (*) to handle unexpected values. | pendiente |
| shell | shelldre:S7679 | MAJOR | `scripts/gen-smtp-certs.sh:48` | Assign this positional parameter to a local variable. | pendiente |
| shell | shelldre:S7679 | MAJOR | `scripts/ip-lockout-warning.sh:58` | Assign this positional parameter to a local variable. | pendiente |
| shell | shelldre:S7679 | MAJOR | `scripts/ip-lockout-warning.sh:58` | Assign this positional parameter to a local variable. | pendiente |
| shell | shelldre:S7682 | MAJOR | `scripts/ip-lockout-warning.sh:57` | Add an explicit return statement at the end of the function. | pendiente |
| shell | shelldre:S7688 | MAJOR | `deploy/postgres-init/01-identity-app-role.sh:10` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | pendiente |
| shell | shelldre:S7688 | MAJOR | `scripts/ip-lockout-warning.sh:55` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | pendiente |
| shell | shelldre:S7688 | MAJOR | `scripts/ip-lockout-warning.sh:62` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | pendiente |
| shell | shelldre:S7688 | MAJOR | `scripts/with-raised-login-limit.sh:15` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | pendiente |
| shell | shelldre:S7688 | MAJOR | `scripts/with-raised-login-limit.sh:25` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | pendiente |
| typescript | typescript:S3358 | MAJOR | `contabilidad/src/views/Transactions.tsx:48` | Extract this nested ternary operation into an independent statement. | pendiente |
| typescript | typescript:S3358 | MAJOR | `contabilidad/src/views/Transactions.tsx:52` | Extract this nested ternary operation into an independent statement. | pendiente |
| typescript | typescript:S3358 | MAJOR | `frontend/src/features/admin/AuditLogPage.tsx:49` | Extract this nested ternary operation into an independent statement. | pendiente |
| typescript | typescript:S3735 | CRITICAL | `frontend/src/features/admin/ServiceKeysPage.tsx:246` | Remove this use of the "void" operator. | pendiente |
| typescript | typescript:S3776 | CRITICAL | `contabilidad/src/auth/flow.ts:54` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. | pendiente |
| typescript | typescript:S3776 | CRITICAL | `frontend/src/features/admin/UserDrawer.tsx:97` | Refactor this function to reduce its Cognitive Complexity from 18 to the 15 allowed. | pendiente |
| typescript | typescript:S4624 | MAJOR | `e2e/manual/capturas.ts:114` | Refactor this code to not use nested template literals. | pendiente |
| typescript | typescript:S4624 | MAJOR | `e2e/support/mailpit.ts:22` | Refactor this code to not use nested template literals. | pendiente |
| typescript | typescript:S5906 | MINOR | `frontend/src/auditLog.test.tsx:136` | Prefer "expect(await within(table).findAllByText('ana@example.test')).toHaveLength(2)" over this generic assertion for b | pendiente |
| typescript | typescript:S6479 | MAJOR | `frontend/src/features/account/MyAccountPage.tsx:36` | Do not use Array index in keys | pendiente |
| typescript | typescript:S6479 | MAJOR | `frontend/src/features/account/PublicPages.tsx:34` | Do not use Array index in keys | pendiente |
| typescript | typescript:S6479 | MAJOR | `frontend/src/features/admin/shared.tsx:63` | Do not use Array index in keys | pendiente |
| typescript | typescript:S6551 | MINOR | `frontend/src/test-utils.ts:19` | 'input' may use Object's default stringification format ('[object Object]') when stringified. | pendiente |
| typescript | typescript:S6551 | MINOR | `frontend/src/test-utils.ts:21` | 'init.body' may use Object's default stringification format ('[object Object]') when stringified. | pendiente |
| typescript | typescript:S6819 | MAJOR | `contabilidad/src/Login.tsx:15` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `contabilidad/src/Toast.tsx:15` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `contabilidad/src/views/ConfirmDialog.tsx:61` | Use <dialog> instead of the "dialog" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/App.tsx:136` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/components/LoadingSkeleton.tsx:2` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/components/Toast.tsx:16` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/MyAccountPage.tsx:37` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/PublicPages.tsx:120` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/PublicPages.tsx:155` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/PublicPages.tsx:184` | Use <output> instead of the "status" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/AuditLogPage.tsx:243` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/AuditLogPage.tsx:249` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/HomePage.tsx:63` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/UserDrawer.tsx:140` | Use <dialog> instead of the "dialog" role to ensure accessibility across all devices. | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/UsersPage.tsx:23` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | pendiente |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/UsersPage.tsx:188` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | pendiente |
| typescript | typescript:S6847 | MAJOR | `contabilidad/src/views/ConfirmDialog.tsx:61` | Non-interactive elements should not be assigned mouse or keyboard event listeners. | pendiente |
| typescript | typescript:S6847 | MAJOR | `frontend/src/features/admin/UserDrawer.tsx:140` | Non-interactive elements should not be assigned mouse or keyboard event listeners. | pendiente |
| typescript | typescript:S6848 | MAJOR | `contabilidad/src/views/ConfirmDialog.tsx:60` | Avoid non-native interactive elements. If using native HTML is not possible, add an appropriate role and support for tab | pendiente |
| typescript | typescript:S7503 | MINOR | `e2e/support/global-teardown.ts:9` | Async function 'globalTeardown' has no 'await' expression. | pendiente |
| typescript | typescript:S7755 | MINOR | `contabilidad/src/aggregate.ts:48` | Prefer `.at(…)` over `[….length - index]`. | pendiente |
| typescript | typescript:S7781 | MINOR | `frontend/src/features/csv.ts:20` | Prefer `String#replaceAll()` over `String#replace()`. | pendiente |
| typescript | typescript:S9382 | MINOR | `e2e/manual/capturas.ts:258` | Unexpected `await` inside a loop. | pendiente |
| typescript | typescript:S9382 | MINOR | `e2e/manual/capturas.ts:498` | Unexpected `await` inside a loop. | pendiente |
| typescript | typescript:S9382 | MINOR | `e2e/support/mailpit.ts:33` | Unexpected `await` inside a loop. | pendiente |
| typescript | typescript:S9382 | MINOR | `e2e/support/mailpit.ts:39` | Unexpected `await` inside a loop. | pendiente |
| typescript | typescript:S9382 | MINOR | `e2e/support/mailpit.ts:74` | Unexpected `await` inside a loop. | pendiente |
