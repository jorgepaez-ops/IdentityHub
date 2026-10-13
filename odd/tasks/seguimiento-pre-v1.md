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
- [x] **S7 — Code smells de SonarCloud.** 123 abiertos al 2026-10-13 (CRITICAL 32, MAJOR 55, MINOR 34,
  INFO 2); reglas principales: go:S3776 (16), typescript:S6819 (16), css:S4666 (11), godre:S8196 (10),
  plsql:S1192 (6), python:S3776 (6). Corregir sin cambiar comportamiento, con las pruebas como red; los
  falsos positivos se justifican en Sonar, no se silencian en bloque.
  - Quinta ronda (inline, Opus): las comillas invertidas fuera de una referencia son formato si están cerradas; el mensaje de comilla sin pareja ya no culpa a `make`; prueba fuerte del mensaje; la prueba de los `.feature` reales tokeniza sus líneas `# Verificación:`; `test_the_grammar_copies_match_the_docstring` mantiene sincronizadas las tres copias de la gramática (mutación en `scripts/README.md`: falla). Revisión de la quinta ronda: aprobada; la advertencia R3-001 (restaurar la prueba de «make» en prosa fuera de `# Verificación:`) se descarta porque el analizador solo lee esas líneas y la ronda anterior marcó esa prueba como incorrecta; se aplicaron las dos sugerencias (prueba con ruta real entre comillas invertidas y `skipTest` con `python -OO`).

Lote 1/3 (Go), 2026-10-13: ver apéndice «S7 — inventario Sonar». Corregido: godre:S8242 (`internal/testdb/testdb.go:73`, `migrationLock` ya no guarda `context.Context`; `release`/`releaseIfHeld` lo reciben por parámetro) y de paso el `errcheck` que golangci-lint con `-tags=integration` marcaba en la misma función (`_ = dropDatabase(...)`). Sin cambio de comportamiento. No aplicado: go:S3776 x16 (todos en archivos `_test.go`, fuera de alcance por instrucción), godre:S8196 x10 (la descripción de la regla no está en la API; el nombre «método+er» daría `WithinLoginTransactioner`, peor que `Repository`; se propone marcar falso positivo en Sonar, decisión del usuario), go:S1135 (falso positivo: «todo» en español). Sin código generado marcado; `.sonarcloud.properties` ya excluye duplicados de `gen.go`/`sqlc`. Observación fuera de alcance: `gocognit` da 25 a `(*smtpMailer).Send` (`backend/cmd/worker/smtp.go:80`) y Sonar no lo marca (cuenta distinto). Evidencia: golangci-lint v2.13.2 0 issues (con y sin `-tags=integration`), `go vet`, `gofmt`, `make test-go` OK (59.9 %), `make test-integration` con Postgres 16 desechable OK (81.1 % antes y después; funciones de `testdb` sin pérdida de cobertura), `gocognit -over 15` sin hallazgos en `testdb.go`.

Lote 2/3 (TS/JS), 2026-10-13: 55 hallazgos typescript/javascript; 51 corregidos y 4 falsos positivos documentados en el apéndice (`e2e/manual/capturas.ts:258`, `e2e/support/mailpit.ts:33/39/74`: bucles secuenciales intencionales). S6819 con elementos nativos: `<dialog open>` en `ConfirmDialog` (contabilidad) y `UserDrawer` (frontend), `<output>` para `role=status` (toasts, skeleton, avisos de éxito, Login), `<fieldset>` para `role=group`; el teclado de los diálogos (Escape, Tab, clic en el fondo) pasó a escuchas nativas con `addEventListener` y CSS ajustado (`.dialog`, `.drawer`, `.success-box`, reset de `fieldset`) para que la apariencia no cambie. Claves de lista sin índice con `keyedMessages` (sufijo de ocurrencia: hay una prueba con mensajes idénticos). S3776 en `completeCallback` (contabilidad) y `UserDrawer.submit`; S3358, S7755, S7781, S6551, S5906, S3735 y, en `docs/informe/build.mjs`, S6582/S8786/S7758/S878/S4624/S9382. Evidencia: frontend lint/typecheck/build OK y vitest 264/264 antes y después; contabilidad lint/typecheck/build OK y vitest 148/148 antes y después; `e2e` `tsc --noEmit` OK; `make informe` con la misma salida que antes (54 páginas, mismo índice); `make up-app && make e2e` 63/63 y `make down`. Los 11 hallazgos `css:` quedan fuera de este lote.

Lote 3/3 (resto de lenguajes), 2026-10-13: 40 hallazgos, ninguno queda «pendiente» en el apéndice. Corregidos: `css:S4666` x11 (`ab2ad9a`; los bloques duplicados se fusionaron verificando propiedad por propiedad que el orden de la cascada no cambia: el segundo `:root[data-theme="dark"]` pisaba todas las variables del primero, que era código muerto; `.logout-button` conservaba `margin-top:auto` anulado por el bloque posterior), `plsql:OrderByExplicitAscCheck` x3 (`40302a3`; `ASC` explícito en todos los `ORDER BY` de `db/queries/` y `sqlc generate` con v1.31.1, cuyo único cambio en Go son esas cadenas SQL), `python:S3776` x6 / `S3358` x2 / `S1481` x1 (`078f7c2`; `traceability.py` y `zap-gate.py` partidos en funciones con nombre, complejidad cognitiva máxima 15 medida con complexipy), `shelldre:S7688` x5 / `S7679` x3 / `S7682` / `S131` (`a5f8365`; `[[ ]]` en los scripts con shebang bash, parámetro posicional a variable local en `ip-lockout-warning.sh` y a variable normal en `gen-smtp-certs.sh`, que es `#!/bin/sh`, sin `local`). No aplicable: `plsql:S1192` en `db/migrations/000001_initial_schema.up.sql` (migración aplicada e inmutable; `db/migrations/**` pasó a `sonar.exclusions`). Falsos positivos: `plsql:S1192` x5 en `db/queries/users.sql` (etiquetas de enums en consultas independientes; no hay constantes entre sentencias y parametrizarlas cambiaría firmas y comportamiento) y `githubactions:S1135` (la palabra «TODOS» en español). Evidencia: los 7 `scripts/*_test.py` salen con 0 (12+4+6+13+17+31+11 pruebas), `make spec-drift` y `traceability.py --check` OK, `bash -n`/`sh -n` OK en los scripts tocados; frontend lint/typecheck/build OK y vitest 264/264 antes y después; `make test-integration` con `postgres:16-bookworm` desechable en 55432 OK (81.1 % antes y después, 29 paquetes `ok`; contenedor eliminado); `make up-app && make e2e` 63/63 y `make down`.

Lote 1b/3 (Go, «incluye todo» con archivos de pruebas), 2026-10-13: corregido godre:S8196 x10 (`c623e04`; `Repository` de bootstrap/employee/invitationresend/login/logout/mfa/refresh pasan a `…Transactor`, un solo método cada una; `healthcheckHTTPClient` → `healthcheckRequester`, `userLookup` → `userByIDGetter`, `deferredConfirmation` → `confirmationWaiter`), go:S1135 (`8f8609d`) y go:S3776 x16 en pruebas, más `(*smtpMailer).Send` (gocognit 25 → `dial`/`secure`/`authenticate`/`transmit`, STARTTLS obligatorio, AUTH solo con TLS y QUIT best-effort intactos) en `0223446`; pruebas por paquete: worker `0223446`, api `b44396a`, auth + audit `7bb35fd`, store `b83f7c3`. Sin cambio de comportamiento ni de nombres de prueba. Evidencia: golangci-lint v2.13.2 0 issues (con y sin `-tags=integration`), `go vet` y `gofmt` limpios, `make test-go` OK (550 `=== RUN`, 59.9 % antes y después), `make test-integration` con Postgres 16 desechable OK (631 `=== RUN` antes y después, mismos nombres salvo UUID aleatorios en subpruebas, 0 FAIL, 0 SKIP; 81.1 % antes y después), `gocognit -over 15` sin hallazgos en las funciones tocadas (quedan otras funciones de prueba que gocognit cuenta distinto de Sonar y que Sonar no marca), `scripts/traceability.py --check` al día.
Revisión nativa del lote Go (2026-10-13), en worktrees por corte porque el lote completo (1 233 líneas) excede el
presupuesto de los revisores: A (`fb568d5..0223446`: testdb, interfaces, comentario, `smtpMailer.Send`) aprobada;
B (`..b44396a`: pruebas de api) aprobada; C2 (`7bb35fd..435f8cc`: pruebas de store y nota) aprobada. C1
(`b44396a..7bb35fd`: pruebas de auth y audit) **sin revisión nativa**: el revisor de Gentle AI rechazó 4 veces la
captura («the candidate could not be inspected»), con dos tamaños de corte; por decisión del usuario se declinó solo
esa revisión (`declined_this_candidate`). Revisión manual de Opus en su lugar: conteo de `t.Fatal/t.Error` por
archivo antes/después; las diferencias (audit 12→5, mfa 56→55, servicekey_test 39→37) son verificaciones movidas a
helpers con `t.Helper()` (`mustExec`, `requireErrorContaining`, `countRows`) o unidas en un bucle equivalente; no se
perdió ninguna. 631 pruebas de integración y 550 unitarias antes y después. Advertencias de A sobre la nota del lote 1
(escrita antes del 1b) superadas por la nota del 1b y el apéndice. Sugerencia de C2 (`context.Background()` en helpers):
no aplica, las pruebas padre ya usan `context.Background()`. Sugerencia de A (`smtp_test` reasigna `s.r` tras `upgrade`):
aplicada (`upgrade` es el único que arma el lector en TLS implícito).

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
| S7 | delegado (Sonnet) | hecho (3 lotes): lote 1/3 Go hecho (godre:S8242, S8196, S3776 x16, S1135, más `smtpMailer.Send`); lote 2/3 TS/JS hecho (51 corregidos, 4 falsos positivos e2e; `e5510f9` contabilidad, `e49e17b` frontend, `6841cf1` docs/informe, `463398e` e2e); lote 3/3 hecho (`ab2ad9a` css, `40302a3` sql, `078f7c2` python, `a5f8365` shell; 1 no aplicable y 6 falsos positivos documentados); todas las filas del apéndice resueltas | `e8b3dd0`, `6a41c55`, `c623e04` (S8196), `8f8609d` (S1135), `0223446` (worker), `b44396a` (api), `7bb35fd` (auth + audit), `b83f7c3` (store) | — |

Revisión nativa de S1-S4 (2026-10-13): aprobada (4 lentes, 15 archivos). Correcciones posteriores, también
aprobadas: identidad de `cosign verify` restringida a `release.yml` en un tag (probada contra `v0.9.1`, y
rechaza la identidad de `ci.yml`) (`a15ee05`); informe con el clasificador probado y extracto de `release.yml`
corregido (`f1b60ca`). Advertencia descartada con evidencia: la prueba de puertos sin `.env` pasa en una copia
limpia con `CI=true` (el compose no usa `env_file` ni interpolación sin `:?`). El rango 161-196 del extracto
incluye a propósito el comentario de la línea 161 y termina en el último renglón del paso de `latest`.

Informe técnico sincronizado con el estado final de S1-S7 (2026-10-13): alcance, conteos de pruebas, puertos en loopback, pipeline, desafíos y limitaciones; rangos `{{code:...}}` de `ci.yml` corregidos; `make informe` ok.

## Siguiente paso

PR de documentación (tras la revisión del PDF por el usuario), luego PR-A (S1-S4) y S5-S6.

## Apéndice S7 — inventario Sonar (2026-10-13)

Fuente: `api/issues/search` (CODE_SMELL, sin resolver), 123 al momento de la consulta. Agrupado por lenguaje y regla; las columnas «Estado» de Go las llena el lote 1/3, el resto queda «pendiente» para los lotes 2/3 (frontend/contabilidad/e2e/CSS/docs JS) y 3/3 (Python, shell, SQL, workflows).

| Lenguaje | Regla | Severidad | Archivo:línea | Mensaje | Estado |
|---|---|---|---|---|---|
| css | css:S4666 | MAJOR | `frontend/src/styles.css:106` | Duplicate selector ".stat-tile", first used at line 103 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:177` | Duplicate selector ".link-button", first used at line 146 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:201` | Duplicate selector ":root", first used at line 1 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:203` | Duplicate selector ":root[data-theme="dark"]", first used at line 31 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:204` | Duplicate selector "body", first used at line 33 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:214` | Duplicate selector ".left-rail", first used at line 55 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:215` | Duplicate selector "nav a", first used at line 59 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:217` | Duplicate selector ".logout-button", first used at line 64 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:218` | Duplicate selector ".icon-button", first used at line 154 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:222` | Duplicate selector ".table-scroll", first used at line 134 | corregido `ab2ad9a` |
| css | css:S4666 | MAJOR | `frontend/src/styles.css:225` | Duplicate selector ".toast", first used at line 168 | corregido `ab2ad9a` |
| githubactions | githubactions:S1135 | INFO | `.github/workflows/ci.yml:14` | Complete the task associated to this "TODO" comment. | falso positivo: «TODOS» (español, «si TODOS los archivos modificados…») en un comentario; no hay tarea pendiente |
| go | go:S1135 | INFO | `backend/internal/events/events_test.go:85` | Complete the task associated to this TODO comment. | corregido (lote 1b, `8f8609d`): comentario reformulado («cualquier tipo de evento») |
| go | go:S3776 | CRITICAL | `backend/cmd/worker/smtp_test.go:136` | Refactor this method to reduce its Cognitive Complexity from 26 to the 15 allowed. | corregido (lote 1b, `0223446`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/api/login_test.go:308` | Refactor this method to reduce its Cognitive Complexity from 17 to the 15 allowed. | corregido (lote 1b, `b44396a`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/api/login_test.go:429` | Refactor this method to reduce its Cognitive Complexity from 19 to the 15 allowed. | corregido (lote 1b, `b44396a`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/api/oauth_test.go:133` | Refactor this method to reduce its Cognitive Complexity from 23 to the 15 allowed. | corregido (lote 1b, `b44396a`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/api/server_generated_test.go:15` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | corregido (lote 1b, `b44396a`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/audit/audit_integration_test.go:13` | Refactor this method to reduce its Cognitive Complexity from 19 to the 15 allowed. | corregido (lote 1b, `7bb35fd`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/auth/bootstrap/bootstrap_integration_test.go:169` | Refactor this method to reduce its Cognitive Complexity from 16 to the 15 allowed. | corregido (lote 1b, `7bb35fd`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/auth/login/login_test.go:168` | Refactor this method to reduce its Cognitive Complexity from 17 to the 15 allowed. | corregido (lote 1b, `7bb35fd`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/auth/mfa/mfa_integration_test.go:103` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | corregido (lote 1b, `7bb35fd`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/auth/refresh/refresh_integration_test.go:23` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | corregido (lote 1b, `7bb35fd`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/auth/servicekey/servicekey_integration_test.go:186` | Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed. | corregido (lote 1b, `7bb35fd`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/auth/servicekey/servicekey_test.go:282` | Refactor this method to reduce its Cognitive Complexity from 22 to the 15 allowed. | corregido (lote 1b, `7bb35fd`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/store/queries_integration_test.go:18` | Refactor this method to reduce its Cognitive Complexity from 16 to the 15 allowed. | corregido (lote 1b, `b83f7c3`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/store/rolegrid_integration_test.go:20` | Refactor this method to reduce its Cognitive Complexity from 25 to the 15 allowed. | corregido (lote 1b, `b83f7c3`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/store/rolegrid_integration_test.go:75` | Refactor this method to reduce its Cognitive Complexity from 25 to the 15 allowed. | corregido (lote 1b, `b83f7c3`): helpers con nombre, mismas aserciones |
| go | go:S3776 | CRITICAL | `backend/internal/store/sessions_integration_test.go:17` | Refactor this method to reduce its Cognitive Complexity from 22 to the 15 allowed. | corregido (lote 1b, `b83f7c3`): helpers con nombre, mismas aserciones |
| go | godre:S8196 | MINOR | `backend/cmd/api/main.go:67` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/cmd/api/main.go:243` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/auth/bootstrap/bootstrap.go:55` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/auth/employee/employee.go:46` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/auth/invitationresend/resend.go:32` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/auth/login/login.go:86` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/auth/logout/logout.go:40` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/auth/mfa/mfa.go:160` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/auth/refresh/refresh.go:77` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8196 | MINOR | `backend/internal/events/broker.go:13` | Rename this interface to follow Go naming conventions for single-method interfaces. | corregido (lote 1b, `c623e04`): renombrada a nombre de agente («…Transactor», `healthcheckRequester`, `userByIDGetter`, `confirmationWaiter`) |
| go | godre:S8242 | MAJOR | `backend/internal/testdb/testdb.go:73` | Remove this 'context.Context' field and pass context as a parameter to methods that need it. | corregido (lote 1/3) |
| javascript | javascript:S4624 | MAJOR | `docs/informe/build.mjs:138` | Refactor this code to not use nested template literals. | corregido `6841cf1` |
| javascript | javascript:S4624 | MAJOR | `docs/informe/build.mjs:157` | Refactor this code to not use nested template literals. | corregido `6841cf1` |
| javascript | javascript:S6582 | MINOR | `docs/informe/build.mjs:81` | Prefer using an optional chain expression instead, as it's more concise and easier to read. | corregido `6841cf1` |
| javascript | javascript:S7758 | MINOR | `docs/informe/build.mjs:122` | Prefer `String.fromCodePoint()` over `String.fromCharCode()`. | corregido `6841cf1` |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | corregido `6841cf1` |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | corregido `6841cf1` |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | corregido `6841cf1` |
| javascript | javascript:S7781 | MINOR | `docs/informe/build.mjs:31` | Prefer `String#replaceAll()` over `String#replace()`. | corregido `6841cf1` |
| javascript | javascript:S878 | MAJOR | `docs/informe/build.mjs:136` | Unexpected use of comma operator. | corregido `6841cf1` |
| javascript | javascript:S878 | MAJOR | `docs/informe/build.mjs:137` | Unexpected use of comma operator. | corregido `6841cf1` |
| javascript | javascript:S8786 | MAJOR | `docs/informe/build.mjs:88` | Simplify this regular expression to reduce its runtime, as it has super-linear performance due to backtracking. | corregido `6841cf1` |
| javascript | javascript:S9382 | MINOR | `docs/informe/build.mjs:301` | Unexpected `await` inside a loop. | falso positivo (revertido tras la revisión nativa): el render secuencial deja el orden estable y reporta cada diagrama roto; la llamada única solo informaba el primer error |
| javascript | javascript:S9382 | MINOR | `docs/informe/build.mjs:322` | Unexpected `await` inside a loop. | corregido `6841cf1` |
| javascript | javascript:S9382 | MINOR | `docs/informe/build.mjs:322` | Unexpected `await` inside a loop. | corregido `6841cf1` |
| plsql | plsql:OrderByExplicitAscCheck | MINOR | `db/queries/roles.sql:6` | Add ASC in order to make the order explicit. | corregido `40302a3` |
| plsql | plsql:OrderByExplicitAscCheck | MINOR | `db/queries/roles.sql:26` | Add ASC in order to make the order explicit. | corregido `40302a3` |
| plsql | plsql:OrderByExplicitAscCheck | MINOR | `db/queries/users.sql:39` | Add ASC in order to make the order explicit. | corregido `40302a3` |
| plsql | plsql:S1192 | CRITICAL | `db/migrations/000001_initial_schema.up.sql:17` | Define a constant instead of duplicating this literal 5 times. | no aplicable: migración aplicada e inmutable (`db/migrations/**` pasó a `sonar.exclusions` en `.sonarcloud.properties`, `40302a3`) |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:38` | Define a constant instead of duplicating this literal 11 times. | falso positivo: los literales son etiquetas de enums (`user_status`, tipos de token, estados) en sentencias independientes; PostgreSQL/sqlc no tienen constantes entre consultas y parametrizarlas cambiaría las firmas Go y el comportamiento |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:62` | Define a constant instead of duplicating this literal 5 times. | falso positivo: los literales son etiquetas de enums (`user_status`, tipos de token, estados) en sentencias independientes; PostgreSQL/sqlc no tienen constantes entre consultas y parametrizarlas cambiaría las firmas Go y el comportamiento |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:104` | Define a constant instead of duplicating this literal 5 times. | falso positivo: los literales son etiquetas de enums (`user_status`, tipos de token, estados) en sentencias independientes; PostgreSQL/sqlc no tienen constantes entre consultas y parametrizarlas cambiaría las firmas Go y el comportamiento |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:287` | Define a constant instead of duplicating this literal 6 times. | falso positivo: los literales son etiquetas de enums (`user_status`, tipos de token, estados) en sentencias independientes; PostgreSQL/sqlc no tienen constantes entre consultas y parametrizarlas cambiaría las firmas Go y el comportamiento |
| plsql | plsql:S1192 | CRITICAL | `db/queries/users.sql:338` | Define a constant instead of duplicating this literal 4 times. | falso positivo: los literales son etiquetas de enums (`user_status`, tipos de token, estados) en sentencias independientes; PostgreSQL/sqlc no tienen constantes entre consultas y parametrizarlas cambiaría las firmas Go y el comportamiento |
| python | python:S1481 | MINOR | `scripts/zap-gate.py:109` | Replace the unused local variable "comment" with "_". | corregido `078f7c2` |
| python | python:S3358 | MAJOR | `scripts/traceability.py:337` | Extract this nested conditional expression into an independent statement. | corregido `078f7c2` |
| python | python:S3358 | MAJOR | `scripts/traceability.py:338` | Extract this nested conditional expression into an independent statement. | corregido `078f7c2` |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:69` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. | corregido `078f7c2` |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:114` | Refactor this function to reduce its Cognitive Complexity from 22 to the 15 allowed. | corregido `078f7c2` |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:167` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. | corregido `078f7c2` |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:240` | Refactor this function to reduce its Cognitive Complexity from 25 to the 15 allowed. | corregido `078f7c2` |
| python | python:S3776 | CRITICAL | `scripts/traceability.py:288` | Refactor this function to reduce its Cognitive Complexity from 40 to the 15 allowed. | corregido `078f7c2` |
| python | python:S3776 | CRITICAL | `scripts/zap-gate.py:80` | Refactor this function to reduce its Cognitive Complexity from 22 to the 15 allowed. | corregido `078f7c2` |
| shell | shelldre:S131 | CRITICAL | `scripts/ip-lockout-warning.sh:26` | Add a default case (*) to handle unexpected values. | corregido `a5f8365` |
| shell | shelldre:S7679 | MAJOR | `scripts/gen-smtp-certs.sh:48` | Assign this positional parameter to a local variable. | corregido `a5f8365` |
| shell | shelldre:S7679 | MAJOR | `scripts/ip-lockout-warning.sh:58` | Assign this positional parameter to a local variable. | corregido `a5f8365` |
| shell | shelldre:S7679 | MAJOR | `scripts/ip-lockout-warning.sh:58` | Assign this positional parameter to a local variable. | corregido `a5f8365` |
| shell | shelldre:S7682 | MAJOR | `scripts/ip-lockout-warning.sh:57` | Add an explicit return statement at the end of the function. | corregido `a5f8365` |
| shell | shelldre:S7688 | MAJOR | `deploy/postgres-init/01-identity-app-role.sh:10` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | corregido `a5f8365` |
| shell | shelldre:S7688 | MAJOR | `scripts/ip-lockout-warning.sh:55` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | corregido `a5f8365` |
| shell | shelldre:S7688 | MAJOR | `scripts/ip-lockout-warning.sh:62` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | corregido `a5f8365` |
| shell | shelldre:S7688 | MAJOR | `scripts/with-raised-login-limit.sh:15` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | corregido `a5f8365` |
| shell | shelldre:S7688 | MAJOR | `scripts/with-raised-login-limit.sh:25` | Use '[[' instead of '[' for conditional tests. The '[[' construct is safer and more feature-rich. | corregido `a5f8365` |
| typescript | typescript:S3358 | MAJOR | `contabilidad/src/views/Transactions.tsx:48` | Extract this nested ternary operation into an independent statement. | corregido `e5510f9` |
| typescript | typescript:S3358 | MAJOR | `contabilidad/src/views/Transactions.tsx:52` | Extract this nested ternary operation into an independent statement. | corregido `e5510f9` |
| typescript | typescript:S3358 | MAJOR | `frontend/src/features/admin/AuditLogPage.tsx:49` | Extract this nested ternary operation into an independent statement. | corregido `e49e17b` |
| typescript | typescript:S3735 | CRITICAL | `frontend/src/features/admin/ServiceKeysPage.tsx:246` | Remove this use of the "void" operator. | corregido `e49e17b` |
| typescript | typescript:S3776 | CRITICAL | `contabilidad/src/auth/flow.ts:54` | Refactor this function to reduce its Cognitive Complexity from 16 to the 15 allowed. | corregido `e5510f9` |
| typescript | typescript:S3776 | CRITICAL | `frontend/src/features/admin/UserDrawer.tsx:97` | Refactor this function to reduce its Cognitive Complexity from 18 to the 15 allowed. | corregido `e49e17b` |
| typescript | typescript:S4624 | MAJOR | `e2e/manual/capturas.ts:114` | Refactor this code to not use nested template literals. | corregido `463398e` |
| typescript | typescript:S4624 | MAJOR | `e2e/support/mailpit.ts:22` | Refactor this code to not use nested template literals. | corregido `463398e` |
| typescript | typescript:S5906 | MINOR | `frontend/src/auditLog.test.tsx:136` | Prefer "expect(await within(table).findAllByText('ana@example.test')).toHaveLength(2)" over this generic assertion for b | corregido `e49e17b` |
| typescript | typescript:S6479 | MAJOR | `frontend/src/features/account/MyAccountPage.tsx:36` | Do not use Array index in keys | corregido `e49e17b` |
| typescript | typescript:S6479 | MAJOR | `frontend/src/features/account/PublicPages.tsx:34` | Do not use Array index in keys | corregido `e49e17b` |
| typescript | typescript:S6479 | MAJOR | `frontend/src/features/admin/shared.tsx:63` | Do not use Array index in keys | corregido `e49e17b` |
| typescript | typescript:S6551 | MINOR | `frontend/src/test-utils.ts:19` | 'input' may use Object's default stringification format ('[object Object]') when stringified. | corregido `e49e17b` |
| typescript | typescript:S6551 | MINOR | `frontend/src/test-utils.ts:21` | 'init.body' may use Object's default stringification format ('[object Object]') when stringified. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `contabilidad/src/Login.tsx:15` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e5510f9` |
| typescript | typescript:S6819 | MAJOR | `contabilidad/src/Toast.tsx:15` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e5510f9` |
| typescript | typescript:S6819 | MAJOR | `contabilidad/src/views/ConfirmDialog.tsx:61` | Use <dialog> instead of the "dialog" role to ensure accessibility across all devices. | corregido `e5510f9` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/App.tsx:136` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/components/LoadingSkeleton.tsx:2` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/components/Toast.tsx:16` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/MyAccountPage.tsx:37` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/PublicPages.tsx:120` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/PublicPages.tsx:155` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/account/PublicPages.tsx:184` | Use <output> instead of the "status" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/AuditLogPage.tsx:243` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/AuditLogPage.tsx:249` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/HomePage.tsx:63` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/UserDrawer.tsx:140` | Use <dialog> instead of the "dialog" role to ensure accessibility across all devices. | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/UsersPage.tsx:23` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | corregido `e49e17b` |
| typescript | typescript:S6819 | MAJOR | `frontend/src/features/admin/UsersPage.tsx:188` | Use <details>, <fieldset>, <optgroup>, or <address> instead of the "group" role to ensure accessibility across all devic | corregido `e49e17b` |
| typescript | typescript:S6847 | MAJOR | `contabilidad/src/views/ConfirmDialog.tsx:61` | Non-interactive elements should not be assigned mouse or keyboard event listeners. | corregido `e5510f9` |
| typescript | typescript:S6847 | MAJOR | `frontend/src/features/admin/UserDrawer.tsx:140` | Non-interactive elements should not be assigned mouse or keyboard event listeners. | corregido `e49e17b` |
| typescript | typescript:S6848 | MAJOR | `contabilidad/src/views/ConfirmDialog.tsx:60` | Avoid non-native interactive elements. If using native HTML is not possible, add an appropriate role and support for tab | corregido `e5510f9` |
| typescript | typescript:S7503 | MINOR | `e2e/support/global-teardown.ts:9` | Async function 'globalTeardown' has no 'await' expression. | corregido `463398e` |
| typescript | typescript:S7755 | MINOR | `contabilidad/src/aggregate.ts:48` | Prefer `.at(…)` over `[….length - index]`. | corregido `e5510f9` |
| typescript | typescript:S7781 | MINOR | `frontend/src/features/csv.ts:20` | Prefer `String#replaceAll()` over `String#replace()`. | corregido `e49e17b` |
| typescript | typescript:S9382 | MINOR | `e2e/manual/capturas.ts:258` | Unexpected `await` inside a loop. | falso positivo: los 4 intentos fallidos de login deben ir en serie para contar contra el bloqueo (en paralelo la cuenta podría no llegar al umbral de forma determinista) |
| typescript | typescript:S9382 | MINOR | `e2e/manual/capturas.ts:498` | Unexpected `await` inside a loop. | corregido `463398e` |
| typescript | typescript:S9382 | MINOR | `e2e/support/mailpit.ts:33` | Unexpected `await` inside a loop. | falso positivo: bucle de sondeo con reintento; cada intento depende del anterior |
| typescript | typescript:S9382 | MINOR | `e2e/support/mailpit.ts:39` | Unexpected `await` inside a loop. | falso positivo: bucle de sondeo con reintento; cada intento depende del anterior |
| typescript | typescript:S9382 | MINOR | `e2e/support/mailpit.ts:74` | Unexpected `await` inside a loop. | falso positivo: bucle de sondeo con espera entre intentos (`sleep`); no es paralelizable |

Revisión nativa del lote 2 (2026-10-13): aprobada (28 archivos, 492 líneas). Correcciones posteriores: `build.mjs` vuelve a renderizar Mermaid diagrama por diagrama (la llamada única perdía diagnósticos; S9382 en esa línea pasa a falso positivo); el comentario de seguridad de `ServiceKeysPage.tsx` vuelve a quedar sobre `RevealedKey`.

Revisión nativa del lote 3 (2026-10-13): aprobada (13 archivos, 514 líneas). Advertencia de orden de cascada en `styles.css` verificada con un script que compara, por selector y propiedad, la secuencia de valores y contextos `@media` antes/después: las 25 diferencias eliminan solo declaraciones que ya pisaba otra posterior del mismo contexto (el valor final no cambia; `.logout-button` `margin-top` queda `0` en ambos contextos, igual que antes). Sugerencias aplicadas: se quita el `.link-button:hover` redundante y se corrige el docstring de `requirement_state`.

Verificación visual del lote 2 (2026-10-13, Playwright, sin cambios en el repo): 76 capturas por lado (escritorio 1280×800 y móvil 390×844, tema claro y oscuro) de HEAD contra `06f48d2`, con diff por píxel. Las diferencias se deben solo a datos de prueba distintos entre los dos stacks (filas, correos, id de cuenta); drawer, diálogo de confirmación, `<output>` y `<fieldset>` sin cambio visual. Teclado: foco inicial dentro del diálogo, Tab/Shift+Tab dan la vuelta, Escape y clic afuera cierran; `getByRole('status')` y `role=group` siguen expuestos. Revisión manual de Opus de las capturas del drawer y del diálogo de Contabilidad.
