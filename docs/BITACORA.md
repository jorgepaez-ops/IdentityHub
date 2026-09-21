# Bitácora del proyecto

Registro de avance, decisiones tomadas sobre la marcha y hallazgos. Se añade una
entrada por sesión de trabajo, la más reciente arriba.

Esta bitácora es también **evidencia de proceso**: junto con el historial de git
y las fichas de `security/findings/`, documenta cómo se llegó al resultado y no
solo cuál fue.

**Convención:** cada entrada cierra con *Estado* y *Siguiente paso*, para que
retomar el trabajo no exija reconstruir el contexto desde cero.

---

## 2026-09-20 · Semana 2 (inicio) — Evidencia "antes", hallazgos del pipeline y reestructuración

Cubre el 2026-09-19 y el 2026-09-20. Seguimiento fino por tarea en `odd/tasks/idp-semana-2.md`;
esta entrada resume lo que importa para entender el proyecto.

### Hecho

- **T0.1**: repo público publicado (`main` y el tag `v0.0.0-vuln-baseline`). GitHub bloqueó el primer push por
  *push protection* (token de Slack sembrado en `legacy_auth.go`): evidencia "antes" de VULN-001.
- **T1a**: OpenAPI enmendado, el refresh token viaja en cookie `HttpOnly; Secure; SameSite=Strict`.
- **T0.4**: plantilla de fichas con filas de evidencia y `docs/evidencia/README.md`.
- **T0.2 y T0.3 (en curso)**: dos escaneos de la línea base (runs 35476102444 y 35534898422), los 26
  `docs/evidencia/VULN-XXX/evidencia.json` con su "antes", alertas de code scanning leídas por API,
  `curl` local sobre la imagen `web` y las 12 huellas de Gitleaks para T31.
- **T1** (Codex): `gen.go` generado desde el OpenAPI con oapi-codegen v2.5.1 + runtime v1.1.2; el servidor implementa las 22
  operaciones (501 las pendientes). Se descartó oapi-codegen v2.8.0: obliga a `go 1.24` y actualiza `x/text`, lo que rompe
  Q11 y borraría la evidencia de VULN-026.
- **T6** (Codex): se retira `middleware.RealIP` de chi (VULN-020) y `ClientIP` solo confía en `X-Forwarded-For` desde los
  proxies configurados (`TRUSTED_PROXIES`, vacío por defecto). chi pasa a v5.3.2 y **Go a 1.25** (Q19, aprobado por el
  usuario): `govulncheck` ya no informa GO-2026-5775 ni 5777 ni los 26 avisos de la biblioteca estándar.
- **Salto a Go 1.25, efectos colaterales:** `golangci-lint` v1 no analiza código de Go 1.25, así que el CI usa golangci-lint
  v2.13.2 con la configuración migrada; `make lint` escondía los fallos y ahora falla; con el linter v2 solo quedan los 3
  avisos de la línea base (`legacy_auth.go`, hasta T23).
- **T0.5** (Codex): las 19 fichas que faltaban; ya hay una por cada VULN-001 a VULN-026. **Fase 0 cerrada.**
- **T0.2 cerrada.** El escaneo semanal (run 35547924202) encontró 33 vulnerabilidades y aun así terminó en éxito sin abrir la
  incidencia: `govulncheck | tee` sin `pipefail` esconde el fallo (D9). Corregido en `scheduled-scan.yml`.
- **T14a** (Codex): la ADR 0005 retira la ventana de gracia de 10 segundos (Q4) y documenta el riesgo residual y su
  mitigación en el cliente.
- **T8** (Codex): tokens de acceso EdDSA de 15 minutos, `GET /.well-known/jwks.json` y el middleware `RequireAuth`; la clave
  de firma es obligatoria (`JWT_SIGNING_KEY`). La prueba de confusión de algoritmo de Codex firmaba tokens sin `kid` y
  habría pasado por otra razón: se rehízo para que solo el algoritmo pueda causar el rechazo.
- **T7** (Codex): Argon2id con los parámetros de la ADR 0004, verificación en tiempo constante, hash señuelo y un semáforo
  que acota a unos 256 MiB simultáneos. La prueba de integración de Codex no podía pasar (omitía una columna `NOT NULL`)
  y se corrigió al ejecutarla contra la base real.
- **T5** (Codex): sqlc v1.31.1 con consultas parametrizadas para usuarios y auditoría, y un adaptador que oculta los
  tipos generados. El hook de revisión previo al commit detectó errores sin contexto en el adaptador y se corrigieron.
- **T4** (Codex): paquete `testdb` que crea y borra una base PostgreSQL temporal por prueba, `make test-integration` y el
  job 6b del CI con PostgreSQL 16.15 fijado por digest. Claude ejecutó las pruebas contra una base real (el sandbox de
  Codex no llega a `localhost:5432`).
- **T2 y T3** (Codex y Claude): `make gen` regenera el servidor Go, los tipos TypeScript y la matriz; el job `spec-drift`
  del CI falla con cualquier deriva (comprobado con una sonda negativa). Fase 1 cerrada.
- **Informe con capturas** (Claude Desktop, fuera del repo): 20 de los 26 VULN ya tienen captura "antes" (005, 006 y 007 cuentan como "no aparecen").

### Decisiones tomadas sobre la marcha

- **Q16 · Evidencia en dos capas.** Claude Desktop no puede escribir en el repo (errores de permisos), así que
  las capturas viven en su informe `.docx` y el repo guarda solo `evidencia.json` (texto). Se rehízo T0.3.
- **Q17 · Sin Dependabot por ahora.** La evidencia de dependencias sale de govulncheck y npm audit.
- **Cierre de tareas de Codex.** El sandbox de Codex no puede escribir en `.git`: Claude revisa el diff y hace
  los dos commits de cada tarea (el de la tarea y el de registro).
- **Comprobar antes de creer.** Contrastar lo que informa cada herramienta con los archivos de evidencia corrigió
  varias suposiciones (ver Hallazgos).

### Hallazgos

**Correcciones al registro de evidencia** (la columna "Gate" ya refleja lo observado):

- Ningún gate detecta VULN-005, 006, 007, 011 ni 024. Que el pipeline esté en verde sobre ellos no es una garantía.
- `USER root` (VULN-009, 018) lo detecta Trivy config `DS002`, no Hadolint. Los secretos en `ENV` (VULN-012) también
  los marca Trivy (`DS031`).
- `GO-2026-5774`, citado en la ficha de VULN-020, no aparece en govulncheck; sí 5775 y 5777.

**Defectos del propio pipeline:**

- **D2 · La imagen `api` no se puede construir hoy.** `apt-get` sobre `debian:11-slim` da 404 en
  `bullseye-security` (Debian 11 ya no recibe paquetes). Es evidencia de VULN-008 y afecta a T27.
- **D6 · El job `secrets` del CI no escaneó nada** en el push inicial: `053e15f^..9f04fec` no existe porque
  `053e15f` es el primer commit. Un gate en rojo por la causa equivocada.
- **D7 · El job "Configuración de contenedores" falla en "Set up job"**: Hadolint y Trivy config no corren en CI.
- **D8 · El job "Dependencias vulnerables" se detiene en govulncheck** y `npm audit` y `osv-scanner` no se ejecutan.
- **D1 · Gitleaks se escaneaba a sí mismo** (16 de 28 hallazgos). Corregido: ahora da 12.

**Hallazgos sin ficha ni id** (el id solo se asigna al crear la ficha; decidir en T0.5 si se abren):

- CodeQL #81 `go/log-injection` (Medium, `legacy_auth.go:113`).
- `amqp091-go` GO-2026-6372 (`broker.go` y `cmd/worker`), y 26 avisos de la biblioteca estándar de Go 1.22 que solo
  se corrigen con Go 1.25 (decisión Q11).
- Semgrep: 40 acciones de GitHub fijadas por etiqueta, 1 `run-shell-injection` en `baseline-scan.yml`, 2
  `request-host-used` en `default.conf`. gosec G104 x3 en `broker.go`.
- 33 alertas CodeQL `js/remote-property-injection` en `docs/diagramas/*.html`: ruido de diagramas generados;
  excluirlas de CodeQL sería tocar un gate y requiere aprobación.

### Dónde queda cada cosa

| Qué | Dónde |
|---|---|
| Estado y decisiones por tarea | `odd/tasks/idp-semana-2.md` (registro de evidencia, Q1 a Q17) |
| Evidencia por VULN, en texto | `docs/evidencia/VULN-XXX/evidencia.json` (26) |
| Salidas de los escáneres y su análisis | `security/evidence/actions-<run>/README.md`, `local-web-baseline.txt` |
| Capturas de pantalla | Informe de Claude Desktop (fuera del repo) |
| Fichas de hallazgos | `security/findings/` (7 hoy; T0.5 crea 19 más) |

### Estado

- Rama `feat/idp-semana-2`; los commits posteriores al push del 2026-09-20 están solo en local.
- Con captura "antes" en el informe: 20 de 26. Sin captura aún: VULN-013, 014 y 015 (evidencia local ya guardada en
  texto), VULN-011 y 024 (ningún gate los detecta) y VULN-023 (ya remediado).
- **T0.3 cerrada.** VULN-013, 014 y 015 se documentan con la salida en texto (Q18). T0.2 espera el escaneo semanal (lunes 2026-09-21).

### Siguiente paso

1. Cerrar T0.2 con el escaneo semanal del lunes; después T0.5 (fichas).
2. Cerrar T0.2 (el escaneo semanal corre solo el lunes 2026-09-21) y T0.3; después T0.5.
3. Empezar el código con T1 (oapi-codegen); Codex escribe y Claude revisa y commitea.

---

## 2026-09-05 · Semana 1 — Specs, esqueleto y línea base vulnerable

### Hecho

**Specs como fuente de verdad (22 archivos).**
31 requisitos (19 RF + 12 RNF), cada uno con criterio de aceptación verificable.
Contrato OpenAPI 3.0.3 con 22 operaciones, cada una declarando su
`x-requirement`. Contrato AsyncAPI con 5 eventos y la topología de RabbitMQ.
Modelo de amenazas STRIDE con 22 amenazas, donde cada mitigación nombra el gate
del pipeline que impide que se pierda. 15 escenarios Gherkin. 8 ADR.

La matriz de trazabilidad **no se escribe a mano**: `scripts/traceability.py` la
deriva del OpenAPI, de las etiquetas de los `.feature` y de los nombres de las
pruebas. Estado actual, sin maquillaje: 0 completos, 17 parciales, 14 sin
cubrir.

**Aplicación.**
Seis contenedores operativos: `db`, `broker`, `mailpit`, `api`, `worker`, `web`.
Esquema de base de datos con los invariantes aplicados por PostgreSQL, no por
disciplina del código.

**Pipeline.**
`ci.yml` con 8 gates bloqueantes · `baseline-scan.yml` en modo informe sobre el
tag vulnerable · `scheduled-scan.yml` semanal que abre incidencia ante CVE
nuevos en código que no ha cambiado.

### Verificado contra el sistema en ejecución

No son afirmaciones: se comprobaron una a una.

| Comprobación | Resultado |
|---|---|
| Recorrido completo navegador → Nginx → API → PostgreSQL y RabbitMQ | correcto |
| Publicar evento → worker → SMTP → Mailpit | correo entregado |
| Republicar el mismo `eventId` | descartado, sigue habiendo 1 correo |
| `INSERT` de un hash MD5 en `users` | rechazado por `CHECK` |
| `UPDATE` y `DELETE` sobre `audit_log` | rechazados por disparador |
| Segunda sesión activa en la misma familia | rechazada por índice único |
| Inyección SQL en el endpoint sembrado | llega literal a la consulta |
| Cabeceras de seguridad en Nginx | ausentes, como se esperaba |
| Usuario de los tres contenedores | `uid=0` |

### Decisiones tomadas sobre la marcha

**Las vulnerabilidades se siembran en el esqueleto, no en el núcleo de auth.**
El plan las ponía en la primera pasada de la semana 2 para remediarlas
reescribiendo en la semana 3. Escribir el IdP dos veces no cabe en un mes.
Concentradas en el esqueleto disparan exactamente los mismos gates, y el núcleo
se escribe bien desde el principio. *Motivo: coste, no criterio de seguridad.*

**OpenAPI 3.0.3 en lugar de 3.1** — `kin-openapi`, sobre el que se apoya
`oapi-codegen`, tiene soporte parcial de 3.1. Ver `adr/0003`.

**El registro de auditoría se protege con disparador, no solo con permisos.**
Un disparador se aplica a todo el mundo, incluido quien se conecte con
credenciales de más. La revocación de `UPDATE`/`DELETE` al rol `identity_app` se
añade en la semana 2, encima y no en lugar del disparador.

**Se eliminó el repositorio git de `~/Documents/Proyectos`.** Tenía cero commits
y englobaba todos los proyectos del usuario; GitHub Actions exige que los
workflows estén en la raíz del repositorio. Movido a `.git.bak-20260905-vacio`
(reversible) tras verificar que no tenía commits, ramas, stash, remotos ni
reflog.

### Hallazgos

Sembrados a propósito, funcionando como se esperaba: VULN-001 (credenciales
incrustadas), VULN-002 (MD5), VULN-005 (SQL concatenado), y los defectos de
Dockerfile y Nginx.

**Los tres que no habíamos previsto —y que justifican tener el pipeline:**

- **VULN-020** · `middleware.RealIP` de chi acepta `X-Forwarded-For` sin
  validar. Rompe a la vez el bloqueo por fuerza bruta (el contador va por IP) y
  la credibilidad del registro de auditoría. Es la frontera de confianza T2 de
  nuestro propio modelo de amenazas, mal implementada.
- **VULN-022** · Inyección SQL **en el driver pgx**, por debajo de nuestro
  código. Desmonta el "uso consultas parametrizadas, luego estoy a salvo".
- **VULN-023** · Gitleaks con reglas por defecto detectaba **2 de 12** secretos:
  identifica por formato (AWS, Slack), no por contexto (`POSTGRES_PASSWORD`).
  Con `.gitleaks.toml` propio pasó a 12 sin falsos positivos. Sin la línea base
  deliberada, ese job habría estado en verde y lo habríamos leído como "no hay
  secretos".

### Estado

- Repositorio: `main` + tag `v0.0.0-vuln-baseline`. Árbol limpio.
- `ci.yml` **en rojo a propósito**: los gates encuentran lo sembrado.
- El pipeline aún **no se ha ejecutado en GitHub Actions** (falta subir el repo).

### Entregables publicados

- **Informe de seguridad de la línea base** —
  https://claude.ai/code/artifact/c652b965-baba-49f8-a283-c57ca7fae1c4
  Fuente versionada en `docs/security-report.html`; se republica desde ese mismo
  archivo, de modo que el documento y el repositorio no pueden divergir.
  La columna `main` está vacía a propósito hasta que exista la medición.

### Siguiente paso

1. Crear el repositorio en GitHub y subirlo. Confirmar que los tres workflows
   se ejecutan y que `baseline-scan.yml` produce el inventario completo.
2. Empezar el núcleo del IdP (semana 2): `sqlc`, Argon2id, JWT Ed25519 con
   JWKS, rotación de refresh con detección de reuso, RBAC y audit log.
3. Generar `backend/internal/api/gen.go` con `oapi-codegen` y activar el gate
   real de deriva de specs.
