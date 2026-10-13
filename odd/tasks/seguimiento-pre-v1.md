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
- [ ] **S5 — RNF-010 (latencia) con prueba de carga.** Prueba de carga mínima y reproducible contra el
  stack de CI con un umbral alineado con `specs/01-requirements.md` (RNF-010) y su gate; la matriz debe
  pasar RNF-010 de «sin cubrir» a cubierto.
- [ ] **S6 — Segunda evidencia de los 7 requisitos parciales.** RF-008, RNF-001, RNF-002, RNF-006,
  RNF-007, RNF-008, RNF-012 tienen una sola columna de evidencia en `specs/07-traceability.md`
  (completo = 2 de 4: Escenarios, Pruebas Go, E2E, Controles CI). Añadir la evidencia que falte con
  pruebas reales, sin inflar la matriz.
- [ ] **S7 — Code smells de SonarCloud.** 123 abiertos al 2026-10-13 (CRITICAL 32, MAJOR 55, MINOR 34,
  INFO 2); reglas principales: go:S3776 (16), typescript:S6819 (16), css:S4666 (11), godre:S8196 (10),
  plsql:S1192 (6), python:S3776 (6). Corregir sin cambiar comportamiento, con las pruebas como red; los
  falsos positivos se justifican en Sonar, no se silencian en bloque.

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
| S5 | — | pendiente | — | — |
| S6 | — | pendiente | — | — |
| S7 | — | pendiente | — | — |

Revisión nativa de S1-S4 (2026-10-13): aprobada (4 lentes, 15 archivos). Correcciones posteriores, también
aprobadas: identidad de `cosign verify` restringida a `release.yml` en un tag (probada contra `v0.9.1`, y
rechaza la identidad de `ci.yml`) (`a15ee05`); informe con el clasificador probado y extracto de `release.yml`
corregido (`f1b60ca`). Advertencia descartada con evidencia: la prueba de puertos sin `.env` pasa en una copia
limpia con `CI=true` (el compose no usa `env_file` ni interpolación sin `:?`). El rango 161-196 del extracto
incluye a propósito el comentario de la línea 161 y termina en el último renglón del paso de `latest`.

## Siguiente paso

PR de documentación (tras la revisión del PDF por el usuario), luego PR-A (S1-S4) y S5-S6.
